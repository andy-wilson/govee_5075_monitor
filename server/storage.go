package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// StorageBackend defines the interface for different storage implementations
// This allows easy migration between JSON, SQLite, InfluxDB, etc.
type StorageBackend interface {
	// Initialize sets up the storage backend
	Initialize() error

	// SaveReadings saves readings for a device
	SaveReadings(deviceAddr string, readings []Reading) error

	// LoadReadings loads readings for a device within a time range
	LoadReadings(deviceAddr string, fromTime, toTime time.Time) ([]Reading, error)

	// LoadAllDeviceReadings loads all readings for a device
	LoadAllDeviceReadings(deviceAddr string) ([]Reading, error)

	// GetDevices returns a list of all unique device addresses
	GetDevices() ([]string, error)

	// DeleteOldReadings removes readings older than the retention period
	DeleteOldReadings(cutoffTime time.Time) error

	// GetReadingCount returns the total number of readings
	GetReadingCount() (int64, error)

	// GetReadingCountByDevice returns reading count per device
	GetReadingCountByDevice(deviceAddr string) (int64, error)

	// GetLatestReadings returns the N most recent readings across all devices
	GetLatestReadings(limit int) ([]Reading, error)

	// GetReadingsPage returns paginated readings with filtering
	GetReadingsPage(offset, limit int, deviceAddr, clientID string, fromTime, toTime time.Time) ([]Reading, int64, error)

	// GetHourlyAggregates returns hourly aggregated data
	GetHourlyAggregates(deviceAddr string, fromTime, toTime time.Time) ([]AggregateReading, error)

	// Close closes the storage backend
	Close() error
}

// AggregateReading represents aggregated sensor data
type AggregateReading struct {
	DeviceAddr string    `json:"device_addr"`
	Timestamp  time.Time `json:"timestamp"`
	AvgTempC   float64   `json:"avg_temp_c"`
	MinTempC   float64   `json:"min_temp_c"`
	MaxTempC   float64   `json:"max_temp_c"`
	AvgHumidity float64  `json:"avg_humidity"`
	MinHumidity float64  `json:"min_humidity"`
	MaxHumidity float64  `json:"max_humidity"`
	Count      int       `json:"count"`
}

// SQLiteStorage implements StorageBackend using SQLite
type SQLiteStorage struct {
	db       *sql.DB
	dbPath   string
	mu       sync.RWMutex
}

// NewSQLiteStorage creates a new SQLite storage backend
func NewSQLiteStorage(dbPath string) *SQLiteStorage {
	return &SQLiteStorage{
		dbPath: dbPath,
	}
}

// Initialize sets up the SQLite database and creates tables
func (s *SQLiteStorage) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(s.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %v", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", s.dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	s.db = db

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS readings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_name TEXT NOT NULL,
		device_addr TEXT NOT NULL,
		temp_c REAL NOT NULL,
		temp_f REAL NOT NULL,
		temp_offset REAL NOT NULL,
		humidity REAL NOT NULL,
		humidity_offset REAL NOT NULL,
		abs_humidity REAL NOT NULL,
		dew_point_c REAL NOT NULL,
		dew_point_f REAL NOT NULL,
		steam_pressure REAL NOT NULL,
		battery INTEGER NOT NULL,
		rssi INTEGER NOT NULL,
		timestamp DATETIME NOT NULL,
		client_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_device_addr ON readings(device_addr);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON readings(timestamp);
	CREATE INDEX IF NOT EXISTS idx_client_id ON readings(client_id);
	CREATE INDEX IF NOT EXISTS idx_device_timestamp ON readings(device_addr, timestamp);

	-- Aggregated hourly data for faster queries
	CREATE TABLE IF NOT EXISTS hourly_aggregates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_addr TEXT NOT NULL,
		hour_timestamp DATETIME NOT NULL,
		avg_temp_c REAL NOT NULL,
		min_temp_c REAL NOT NULL,
		max_temp_c REAL NOT NULL,
		avg_humidity REAL NOT NULL,
		min_humidity REAL NOT NULL,
		max_humidity REAL NOT NULL,
		count INTEGER NOT NULL,
		UNIQUE(device_addr, hour_timestamp)
	);

	CREATE INDEX IF NOT EXISTS idx_hourly_device ON hourly_aggregates(device_addr, hour_timestamp);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %v", err)
	}

	// Set pragmas for better performance
	pragmas := []string{
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = 10000",
		"PRAGMA temp_store = MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to set pragma: %v", err)
		}
	}

	return nil
}

// SaveReadings saves readings to SQLite database
func (s *SQLiteStorage) SaveReadings(deviceAddr string, readings []Reading) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO readings (
			device_name, device_addr, temp_c, temp_f, temp_offset,
			humidity, humidity_offset, abs_humidity, dew_point_c, dew_point_f,
			steam_pressure, battery, rssi, timestamp, client_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, r := range readings {
		_, err := stmt.Exec(
			r.DeviceName, r.DeviceAddr, r.TempC, r.TempF, r.TempOffset,
			r.Humidity, r.HumidityOffset, r.AbsHumidity, r.DewPointC, r.DewPointF,
			r.SteamPressure, r.Battery, r.RSSI, r.Timestamp, r.ClientID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert reading: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// LoadReadings loads readings from SQLite within a time range
func (s *SQLiteStorage) LoadReadings(deviceAddr string, fromTime, toTime time.Time) ([]Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT device_name, device_addr, temp_c, temp_f, temp_offset,
			   humidity, humidity_offset, abs_humidity, dew_point_c, dew_point_f,
			   steam_pressure, battery, rssi, timestamp, client_id
		FROM readings
		WHERE device_addr = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(query, deviceAddr, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query readings: %v", err)
	}
	defer rows.Close()

	return s.scanReadings(rows)
}

// LoadAllDeviceReadings loads all readings for a device
func (s *SQLiteStorage) LoadAllDeviceReadings(deviceAddr string) ([]Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT device_name, device_addr, temp_c, temp_f, temp_offset,
			   humidity, humidity_offset, abs_humidity, dew_point_c, dew_point_f,
			   steam_pressure, battery, rssi, timestamp, client_id
		FROM readings
		WHERE device_addr = ?
		ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(query, deviceAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to query readings: %v", err)
	}
	defer rows.Close()

	return s.scanReadings(rows)
}

// scanReadings is a helper to scan SQL rows into Reading structs
func (s *SQLiteStorage) scanReadings(rows *sql.Rows) ([]Reading, error) {
	var readings []Reading
	for rows.Next() {
		var r Reading
		err := rows.Scan(
			&r.DeviceName, &r.DeviceAddr, &r.TempC, &r.TempF, &r.TempOffset,
			&r.Humidity, &r.HumidityOffset, &r.AbsHumidity, &r.DewPointC, &r.DewPointF,
			&r.SteamPressure, &r.Battery, &r.RSSI, &r.Timestamp, &r.ClientID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reading: %v", err)
		}
		readings = append(readings, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating readings: %v", err)
	}

	return readings, nil
}

// GetDevices returns all unique device addresses
func (s *SQLiteStorage) GetDevices() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT DISTINCT device_addr FROM readings ORDER BY device_addr"
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %v", err)
	}
	defer rows.Close()

	var devices []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return nil, fmt.Errorf("failed to scan device: %v", err)
		}
		devices = append(devices, addr)
	}

	return devices, nil
}

// DeleteOldReadings removes readings older than cutoff time
func (s *SQLiteStorage) DeleteOldReadings(cutoffTime time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM readings WHERE timestamp < ?", cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to delete old readings: %v", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		// Also delete old aggregates
		s.db.Exec("DELETE FROM hourly_aggregates WHERE hour_timestamp < ?", cutoffTime)

		// Vacuum to reclaim space (do this periodically, not every time)
		go s.db.Exec("VACUUM")
	}

	return nil
}

// GetReadingCount returns total reading count
func (s *SQLiteStorage) GetReadingCount() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM readings").Scan(&count)
	return count, err
}

// GetReadingCountByDevice returns reading count for a specific device
func (s *SQLiteStorage) GetReadingCountByDevice(deviceAddr string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM readings WHERE device_addr = ?", deviceAddr).Scan(&count)
	return count, err
}

// GetLatestReadings returns the N most recent readings
func (s *SQLiteStorage) GetLatestReadings(limit int) ([]Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT device_name, device_addr, temp_c, temp_f, temp_offset,
			   humidity, humidity_offset, abs_humidity, dew_point_c, dew_point_f,
			   steam_pressure, battery, rssi, timestamp, client_id
		FROM readings
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest readings: %v", err)
	}
	defer rows.Close()

	return s.scanReadings(rows)
}

// GetReadingsPage returns paginated readings with filtering
func (s *SQLiteStorage) GetReadingsPage(offset, limit int, deviceAddr, clientID string, fromTime, toTime time.Time) ([]Reading, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build dynamic query based on filters
	where := []string{"1=1"}
	args := []interface{}{}

	if deviceAddr != "" {
		where = append(where, "device_addr = ?")
		args = append(args, deviceAddr)
	}
	if clientID != "" {
		where = append(where, "client_id = ?")
		args = append(args, clientID)
	}
	if !fromTime.IsZero() {
		where = append(where, "timestamp >= ?")
		args = append(args, fromTime)
	}
	if !toTime.IsZero() {
		where = append(where, "timestamp <= ?")
		args = append(args, toTime)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM readings %s", whereClause)
	var total int64
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count readings: %v", err)
	}

	// Get paginated results
	query := fmt.Sprintf(`
		SELECT device_name, device_addr, temp_c, temp_f, temp_offset,
			   humidity, humidity_offset, abs_humidity, dew_point_c, dew_point_f,
			   steam_pressure, battery, rssi, timestamp, client_id
		FROM readings
		%s
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query readings page: %v", err)
	}
	defer rows.Close()

	readings, err := s.scanReadings(rows)
	return readings, total, err
}

// GetHourlyAggregates returns hourly aggregated data
func (s *SQLiteStorage) GetHourlyAggregates(deviceAddr string, fromTime, toTime time.Time) ([]AggregateReading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First check if aggregates exist, if not compute them on the fly
	query := `
		SELECT device_addr, hour_timestamp, avg_temp_c, min_temp_c, max_temp_c,
			   avg_humidity, min_humidity, max_humidity, count
		FROM hourly_aggregates
		WHERE device_addr = ? AND hour_timestamp >= ? AND hour_timestamp <= ?
		ORDER BY hour_timestamp DESC
	`

	rows, err := s.db.Query(query, deviceAddr, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregates: %v", err)
	}
	defer rows.Close()

	var aggregates []AggregateReading
	for rows.Next() {
		var a AggregateReading
		err := rows.Scan(
			&a.DeviceAddr, &a.Timestamp, &a.AvgTempC, &a.MinTempC, &a.MaxTempC,
			&a.AvgHumidity, &a.MinHumidity, &a.MaxHumidity, &a.Count,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregate: %v", err)
		}
		aggregates = append(aggregates, a)
	}

	// If no pre-computed aggregates, compute on the fly
	if len(aggregates) == 0 {
		return s.computeHourlyAggregates(deviceAddr, fromTime, toTime)
	}

	return aggregates, nil
}

// computeHourlyAggregates computes aggregates on-the-fly when not pre-computed
func (s *SQLiteStorage) computeHourlyAggregates(deviceAddr string, fromTime, toTime time.Time) ([]AggregateReading, error) {
	query := `
		SELECT
			device_addr,
			datetime(timestamp, 'start of hour') as hour,
			AVG(temp_c) as avg_temp,
			MIN(temp_c) as min_temp,
			MAX(temp_c) as max_temp,
			AVG(humidity) as avg_humidity,
			MIN(humidity) as min_humidity,
			MAX(humidity) as max_humidity,
			COUNT(*) as count
		FROM readings
		WHERE device_addr = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY device_addr, datetime(timestamp, 'start of hour')
		ORDER BY hour DESC
	`

	rows, err := s.db.Query(query, deviceAddr, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to compute aggregates: %v", err)
	}
	defer rows.Close()

	var aggregates []AggregateReading
	for rows.Next() {
		var a AggregateReading
		var hourStr string
		err := rows.Scan(
			&a.DeviceAddr, &hourStr, &a.AvgTempC, &a.MinTempC, &a.MaxTempC,
			&a.AvgHumidity, &a.MinHumidity, &a.MaxHumidity, &a.Count,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan computed aggregate: %v", err)
		}
		a.Timestamp, _ = time.Parse("2006-01-02 15:04:05", hourStr)
		aggregates = append(aggregates, a)
	}

	return aggregates, nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// JSONStorage implements StorageBackend using JSON files (legacy support)
type JSONStorage struct {
	baseDir string
	mu      sync.RWMutex
}

// NewJSONStorage creates a new JSON file-based storage backend
func NewJSONStorage(baseDir string) *JSONStorage {
	return &JSONStorage{
		baseDir: baseDir,
	}
}

// Initialize sets up the JSON storage directories
func (j *JSONStorage) Initialize() error {
	return os.MkdirAll(j.baseDir, 0755)
}

// SaveReadings saves readings to JSON files
func (j *JSONStorage) SaveReadings(deviceAddr string, readings []Reading) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	sanitizedAddr, err := sanitizeDeviceAddr(deviceAddr)
	if err != nil {
		return err
	}

	deviceFile := filepath.Join(j.baseDir, fmt.Sprintf("readings_%s.json", sanitizedAddr))
	data, err := json.Marshal(readings)
	if err != nil {
		return fmt.Errorf("failed to marshal readings: %v", err)
	}

	return os.WriteFile(deviceFile, data, 0644)
}

// LoadReadings loads readings from JSON files (with time filtering)
func (j *JSONStorage) LoadReadings(deviceAddr string, fromTime, toTime time.Time) ([]Reading, error) {
	allReadings, err := j.LoadAllDeviceReadings(deviceAddr)
	if err != nil {
		return nil, err
	}

	// Filter by time range
	var filtered []Reading
	for _, r := range allReadings {
		if (r.Timestamp.Equal(fromTime) || r.Timestamp.After(fromTime)) &&
			(r.Timestamp.Equal(toTime) || r.Timestamp.Before(toTime)) {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// LoadAllDeviceReadings loads all readings for a device from JSON
func (j *JSONStorage) LoadAllDeviceReadings(deviceAddr string) ([]Reading, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	sanitizedAddr, err := sanitizeDeviceAddr(deviceAddr)
	if err != nil {
		return nil, err
	}

	deviceFile := filepath.Join(j.baseDir, fmt.Sprintf("readings_%s.json", sanitizedAddr))
	data, err := os.ReadFile(deviceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Reading{}, nil
		}
		return nil, fmt.Errorf("failed to read readings file: %v", err)
	}

	var readings []Reading
	if err := json.Unmarshal(data, &readings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal readings: %v", err)
	}

	return readings, nil
}

// GetDevices returns all device addresses from JSON files
func (j *JSONStorage) GetDevices() ([]string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	files, err := filepath.Glob(filepath.Join(j.baseDir, "readings_*.json"))
	if err != nil {
		return nil, err
	}

	var devices []string
	for _, file := range files {
		base := filepath.Base(file)
		// Extract device address from filename: readings_<addr>.json
		addr := strings.TrimPrefix(base, "readings_")
		addr = strings.TrimSuffix(addr, ".json")
		devices = append(devices, addr)
	}

	return devices, nil
}

// DeleteOldReadings removes old readings from JSON files
func (j *JSONStorage) DeleteOldReadings(cutoffTime time.Time) error {
	devices, err := j.GetDevices()
	if err != nil {
		return err
	}

	for _, device := range devices {
		readings, err := j.LoadAllDeviceReadings(device)
		if err != nil {
			continue
		}

		// Filter out old readings
		var kept []Reading
		for _, r := range readings {
			if r.Timestamp.After(cutoffTime) {
				kept = append(kept, r)
			}
		}

		if len(kept) != len(readings) {
			j.SaveReadings(device, kept)
		}
	}

	return nil
}

// GetReadingCount returns total count from JSON files
func (j *JSONStorage) GetReadingCount() (int64, error) {
	devices, err := j.GetDevices()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, device := range devices {
		readings, err := j.LoadAllDeviceReadings(device)
		if err != nil {
			continue
		}
		total += int64(len(readings))
	}

	return total, nil
}

// GetReadingCountByDevice returns count for specific device
func (j *JSONStorage) GetReadingCountByDevice(deviceAddr string) (int64, error) {
	readings, err := j.LoadAllDeviceReadings(deviceAddr)
	if err != nil {
		return 0, err
	}
	return int64(len(readings)), nil
}

// GetLatestReadings returns N most recent readings (simplified for JSON)
func (j *JSONStorage) GetLatestReadings(limit int) ([]Reading, error) {
	devices, err := j.GetDevices()
	if err != nil {
		return nil, err
	}

	var allReadings []Reading
	for _, device := range devices {
		readings, err := j.LoadAllDeviceReadings(device)
		if err != nil {
			continue
		}
		allReadings = append(allReadings, readings...)
	}

	// Sort by timestamp descending
	sort.Slice(allReadings, func(i, j int) bool {
		return allReadings[i].Timestamp.After(allReadings[j].Timestamp)
	})

	if len(allReadings) > limit {
		allReadings = allReadings[:limit]
	}

	return allReadings, nil
}

// GetReadingsPage returns paginated readings (simplified for JSON)
func (j *JSONStorage) GetReadingsPage(offset, limit int, deviceAddr, clientID string, fromTime, toTime time.Time) ([]Reading, int64, error) {
	var allReadings []Reading

	if deviceAddr != "" {
		readings, err := j.LoadReadings(deviceAddr, fromTime, toTime)
		if err != nil {
			return nil, 0, err
		}
		allReadings = readings
	} else {
		devices, err := j.GetDevices()
		if err != nil {
			return nil, 0, err
		}
		for _, device := range devices {
			readings, err := j.LoadReadings(device, fromTime, toTime)
			if err != nil {
				continue
			}
			allReadings = append(allReadings, readings...)
		}
	}

	// Filter by client ID if specified
	if clientID != "" {
		var filtered []Reading
		for _, r := range allReadings {
			if r.ClientID == clientID {
				filtered = append(filtered, r)
			}
		}
		allReadings = filtered
	}

	total := int64(len(allReadings))

	// Sort and paginate
	sort.Slice(allReadings, func(i, j int) bool {
		return allReadings[i].Timestamp.After(allReadings[j].Timestamp)
	})

	if offset >= len(allReadings) {
		return []Reading{}, total, nil
	}

	end := offset + limit
	if end > len(allReadings) {
		end = len(allReadings)
	}

	return allReadings[offset:end], total, nil
}

// GetHourlyAggregates returns aggregated data (computed on-the-fly for JSON)
func (j *JSONStorage) GetHourlyAggregates(deviceAddr string, fromTime, toTime time.Time) ([]AggregateReading, error) {
	readings, err := j.LoadReadings(deviceAddr, fromTime, toTime)
	if err != nil {
		return nil, err
	}

	// Group by hour
	hourlyData := make(map[string]*AggregateReading)
	for _, r := range readings {
		hour := r.Timestamp.Truncate(time.Hour)
		key := hour.Format(time.RFC3339)

		if agg, exists := hourlyData[key]; exists {
			agg.AvgTempC = (agg.AvgTempC*float64(agg.Count) + r.TempC) / float64(agg.Count+1)
			agg.AvgHumidity = (agg.AvgHumidity*float64(agg.Count) + r.Humidity) / float64(agg.Count+1)
			if r.TempC < agg.MinTempC {
				agg.MinTempC = r.TempC
			}
			if r.TempC > agg.MaxTempC {
				agg.MaxTempC = r.TempC
			}
			if r.Humidity < agg.MinHumidity {
				agg.MinHumidity = r.Humidity
			}
			if r.Humidity > agg.MaxHumidity {
				agg.MaxHumidity = r.Humidity
			}
			agg.Count++
		} else {
			hourlyData[key] = &AggregateReading{
				DeviceAddr:  deviceAddr,
				Timestamp:   hour,
				AvgTempC:    r.TempC,
				MinTempC:    r.TempC,
				MaxTempC:    r.TempC,
				AvgHumidity: r.Humidity,
				MinHumidity: r.Humidity,
				MaxHumidity: r.Humidity,
				Count:       1,
			}
		}
	}

	// Convert map to slice
	var aggregates []AggregateReading
	for _, agg := range hourlyData {
		aggregates = append(aggregates, *agg)
	}

	// Sort by timestamp descending
	sort.Slice(aggregates, func(i, j int) bool {
		return aggregates[i].Timestamp.After(aggregates[j].Timestamp)
	})

	return aggregates, nil
}

// Close is a no-op for JSON storage
func (j *JSONStorage) Close() error {
	return nil
}
