package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Reading represents a single measurement from a Govee device
type Reading struct {
	DeviceName     string    `json:"device_name"`
	DeviceAddr     string    `json:"device_addr"`
	TempC          float64   `json:"temp_c"`
	TempF          float64   `json:"temp_f"`
	TempOffset     float64   `json:"temp_offset"`
	Humidity       float64   `json:"humidity"`
	HumidityOffset float64   `json:"humidity_offset"`
	AbsHumidity    float64   `json:"abs_humidity"`
	DewPointC      float64   `json:"dew_point_c"`
	DewPointF      float64   `json:"dew_point_f"`
	SteamPressure  float64   `json:"steam_pressure"`
	Battery        int       `json:"battery"`
	RSSI           int       `json:"rssi"`
	Timestamp      time.Time `json:"timestamp"`
	ClientID       string    `json:"client_id"`
}

// DeviceStatus represents the latest status of a device
type DeviceStatus struct {
	DeviceName     string    `json:"device_name"`
	DeviceAddr     string    `json:"device_addr"`
	TempC          float64   `json:"temp_c"`
	TempF          float64   `json:"temp_f"`
	TempOffset     float64   `json:"temp_offset"`
	Humidity       float64   `json:"humidity"`
	HumidityOffset float64   `json:"humidity_offset"`
	AbsHumidity    float64   `json:"abs_humidity"`
	DewPointC      float64   `json:"dew_point_c"`
	DewPointF      float64   `json:"dew_point_f"`
	SteamPressure  float64   `json:"steam_pressure"`
	Battery        int       `json:"battery"`
	RSSI           int       `json:"rssi"`
	LastUpdate     time.Time `json:"last_update"`
	ClientID       string    `json:"client_id"`
	LastSeen       time.Time `json:"last_seen"`
	ReadingCount   int       `json:"reading_count"`
}

// ClientStatus represents the latest status of a client
type ClientStatus struct {
	ClientID        string    `json:"client_id"`
	LastSeen        time.Time `json:"last_seen"`
	DeviceCount     int       `json:"device_count"`
	ReadingCount    int       `json:"reading_count"`
	ConnectedSince  time.Time `json:"connected_since"`
	IsActive        bool      `json:"is_active"`
	InactiveTimeout time.Duration
}

// AuthConfig represents configuration for API keys
type AuthConfig struct {
	EnableAuth      bool              `json:"enable_auth"`
	APIKeys         map[string]string `json:"api_keys"` // Map of API key -> client ID
	AdminKey        string            `json:"admin_key"`
	DefaultAPIKey   string            `json:"default_api_key"`
	AllowDefaultKey bool              `json:"allow_default_key"`
}

// StorageConfig represents configuration for time-based partitioning and retention
type StorageConfig struct {
	BaseDir            string        `json:"base_dir"`              // Base storage directory
	TimePartitioning   bool          `json:"time_partitioning"`     // Enable time-based partitioning
	PartitionInterval  time.Duration `json:"partition_interval"`    // Interval for new partitions (e.g., 24h, 168h/weekly, 720h/monthly)
	RetentionPeriod    time.Duration `json:"retention_period"`      // How long to keep data (0 = forever)
	MaxReadingsPerFile int           `json:"max_readings_per_file"` // Maximum readings per file
	CompressOldData    bool          `json:"compress_old_data"`     // Compress older partitions
}

// Server represents the Govee server
type Server struct {
	// Maps device address to device status
	devices map[string]*DeviceStatus
	// Maps client ID to client status
	clients map[string]*ClientStatus
	// Stores readings for a device
	readings map[string][]Reading
	// Mutex for thread safety
	mu sync.RWMutex
	// File logger
	logger *os.File
	// Configuration settings
	config *Config
	// Authentication configuration
	auth *AuthConfig
	// Storage manager
	storageManager *StorageManager
	// Shutdown context
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	// Rate limiter
	rateLimiter *RateLimiter
}

// RateLimiter tracks rate limits per IP address
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
	}
}

// GetLimiter returns the rate limiter for an IP address
func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(10, 20) // 10 req/sec, burst 20
		rl.limiters[ip] = limiter
	}

	return limiter
}

// Config represents server configuration
type Config struct {
	Port               int           `json:"port"`
	LogFile            string        `json:"log_file"`
	ClientTimeout      time.Duration `json:"client_timeout"`
	ReadingsPerDevice  int           `json:"readings_per_device"`
	StorageDir         string        `json:"storage_dir"`
	PersistenceEnabled bool          `json:"persistence_enabled"`
	SaveInterval       time.Duration `json:"save_interval"`
	EnableHTTPS        bool          `json:"enable_https"`
	CertFile           string        `json:"cert_file"`
	KeyFile            string        `json:"key_file"`
}

// StorageManager handles reading/writing data with partitioning and retention policies
type StorageManager struct {
	config      *StorageConfig
	mu          sync.RWMutex
	currentTime time.Time // Used for determining partition boundaries
}

// NewStorageManager creates a storage manager with the given configuration
func NewStorageManager(config *StorageConfig) *StorageManager {
	// Set default values if not specified
	if config.PartitionInterval == 0 {
		config.PartitionInterval = 30 * 24 * time.Hour // Default 30 days
	}
	if config.MaxReadingsPerFile == 0 {
		config.MaxReadingsPerFile = 1000 // Default 1000 readings per file
	}

	return &StorageManager{
		config:      config,
		currentTime: time.Now(),
	}
}

// sanitizeDeviceAddr validates and sanitizes device MAC addresses to prevent path traversal
func sanitizeDeviceAddr(addr string) (string, error) {
	// MAC address format: XX:XX:XX:XX:XX:XX or XXXXXXXXXXXX
	matched, _ := regexp.MatchString(`^[0-9A-Fa-f:]{12,17}$`, addr)
	if !matched {
		return "", fmt.Errorf("invalid device address format")
	}
	// Remove colons and convert to lowercase for consistent filenames
	sanitized := strings.ReplaceAll(strings.ToLower(addr), ":", "")
	return sanitized, nil
}

// validateReading validates sensor reading values
func validateReading(r *Reading) error {
	if r.TempC < -50 || r.TempC > 100 {
		return fmt.Errorf("temperature out of range: %.1f°C", r.TempC)
	}
	if r.Humidity < 0 || r.Humidity > 100 {
		return fmt.Errorf("humidity out of range: %.1f%%", r.Humidity)
	}
	if r.Battery < 0 || r.Battery > 100 {
		return fmt.Errorf("battery out of range: %d%%", r.Battery)
	}
	if len(r.DeviceName) > 100 {
		return fmt.Errorf("device name too long")
	}
	if len(r.DeviceAddr) == 0 {
		return fmt.Errorf("device address required")
	}
	if len(r.ClientID) == 0 {
		return fmt.Errorf("client ID required")
	}
	if len(r.ClientID) > 100 {
		return fmt.Errorf("client ID too long")
	}
	// Timestamp should be recent (within 24 hours)
	now := time.Now()
	if r.Timestamp.After(now.Add(time.Hour)) {
		return fmt.Errorf("timestamp in future")
	}
	if r.Timestamp.Before(now.Add(-24 * time.Hour)) {
		return fmt.Errorf("timestamp too old")
	}
	return nil
}

// getPartitionDirForTime returns the directory path for a specific time
func (sm *StorageManager) getPartitionDirForTime(t time.Time) string {
	if !sm.config.TimePartitioning {
		return sm.config.BaseDir
	}

	// Format the time into a directory name based on the partitioning interval
	var format string
	switch {
	case sm.config.PartitionInterval <= 24*time.Hour:
		// Daily partitioning
		format = "2006-01-02" // YYYY-MM-DD
	case sm.config.PartitionInterval <= 7*24*time.Hour:
		// Weekly partitioning
		year, week := t.ISOWeek()
		return filepath.Join(sm.config.BaseDir, fmt.Sprintf("%d-W%02d", year, week))
	default:
		// Monthly partitioning
		format = "2006-01" // YYYY-MM
	}

	return filepath.Join(sm.config.BaseDir, t.Format(format))
}

// getCurrentPartitionDir returns the directory for the current time period
func (sm *StorageManager) getCurrentPartitionDir() string {
	return sm.getPartitionDirForTime(time.Now())
}

// saveReadings saves readings for a device to the appropriate partition
func (sm *StorageManager) saveReadings(deviceAddr string, readings []Reading) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Sanitize device address to prevent path traversal
	sanitizedAddr, err := sanitizeDeviceAddr(deviceAddr)
	if err != nil {
		return fmt.Errorf("invalid device address: %v", err)
	}

	// Get the current partition directory
	partitionDir := sm.getCurrentPartitionDir()

	// Create partition directory if it doesn't exist
	if err := os.MkdirAll(partitionDir, 0755); err != nil {
		return fmt.Errorf("failed to create partition directory: %v", err)
	}

	// Create the device file path with sanitized address
	deviceFile := filepath.Join(partitionDir, fmt.Sprintf("readings_%s.json", sanitizedAddr))

	// Serialize and save the readings (compact JSON for efficiency)
	readingsData, err := json.Marshal(readings)
	if err != nil {
		return fmt.Errorf("failed to marshal readings for device %s: %v", deviceAddr, err)
	}

	if err := os.WriteFile(deviceFile, readingsData, 0644); err != nil {
		return fmt.Errorf("failed to save readings for device %s: %v", deviceAddr, err)
	}

	return nil
}

// loadReadings loads readings for a specific device across all relevant partitions
func (sm *StorageManager) loadReadings(deviceAddr string, fromTime, toTime time.Time) ([]Reading, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Sanitize device address to prevent path traversal
	sanitizedAddr, err := sanitizeDeviceAddr(deviceAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid device address: %v", err)
	}

	var allReadings []Reading

	// If not using time partitioning, just load from the base directory
	if !sm.config.TimePartitioning {
		deviceFile := filepath.Join(sm.config.BaseDir, fmt.Sprintf("readings_%s.json", sanitizedAddr))
		readings, err := sm.loadReadingsFromFile(deviceFile)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		allReadings = append(allReadings, readings...)
	} else {
		// Determine which partitions to check based on the time range
		startPartition := sm.getPartitionDirForTime(fromTime)
		endPartition := sm.getPartitionDirForTime(toTime)

		// List all partition directories
		partitions, err := sm.listPartitionDirs()
		if err != nil {
			return nil, err
		}

		// Load readings from relevant partitions
		for _, partition := range partitions {
			// Only include partitions in the time range
			if (fromTime.IsZero() || partition >= startPartition) &&
				(toTime.IsZero() || partition <= endPartition) {
				deviceFile := filepath.Join(partition, fmt.Sprintf("readings_%s.json", sanitizedAddr))
				readings, err := sm.loadReadingsFromFile(deviceFile)
				if err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				allReadings = append(allReadings, readings...)
			}
		}
	}

	// Filter readings by time range if specified
	if !fromTime.IsZero() || !toTime.IsZero() {
		var filteredReadings []Reading
		for _, r := range allReadings {
			if (fromTime.IsZero() || r.Timestamp.After(fromTime) || r.Timestamp.Equal(fromTime)) &&
				(toTime.IsZero() || r.Timestamp.Before(toTime) || r.Timestamp.Equal(toTime)) {
				filteredReadings = append(filteredReadings, r)
			}
		}
		allReadings = filteredReadings
	}

	// Sort readings by timestamp
	sort.Slice(allReadings, func(i, j int) bool {
		return allReadings[i].Timestamp.Before(allReadings[j].Timestamp)
	})

	return allReadings, nil
}

// loadReadingsFromFile loads readings from a specific file
func (sm *StorageManager) loadReadingsFromFile(filePath string) ([]Reading, error) {
	// Check for compressed file first
	compressedPath := filePath + ".gz"
	if _, err := os.Stat(compressedPath); err == nil {
		// File is compressed, decompress it
		f, err := os.Open(compressedPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()

		data, err := io.ReadAll(gz)
		if err != nil {
			return nil, err
		}

		var readings []Reading
		if err := json.Unmarshal(data, &readings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal readings: %v", err)
		}

		return readings, nil
	}

	// Try regular file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var readings []Reading
	if err := json.Unmarshal(data, &readings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal readings: %v", err)
	}

	return readings, nil
}

// listPartitionDirs returns a sorted list of all partition directories
func (sm *StorageManager) listPartitionDirs() ([]string, error) {
	// If not using time partitioning, just return the base directory
	if !sm.config.TimePartitioning {
		return []string{sm.config.BaseDir}, nil
	}

	// List all directories in the base directory
	entries, err := os.ReadDir(sm.config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %v", err)
	}

	var partitions []string
	for _, entry := range entries {
		if entry.IsDir() {
			partitions = append(partitions, filepath.Join(sm.config.BaseDir, entry.Name()))
		}
	}

	// Sort partitions by name (which corresponds to chronological order due to formatting)
	sort.Strings(partitions)

	return partitions, nil
}

// enforceRetention enforces the retention policy by removing old partitions
func (sm *StorageManager) enforceRetention() error {
	// No retention policy if retention period is 0
	if sm.config.RetentionPeriod == 0 {
		return nil
	}

	// Calculate the cutoff time
	cutoffTime := time.Now().Add(-sm.config.RetentionPeriod)

	// Get all partition directories
	partitions, err := sm.listPartitionDirs()
	if err != nil {
		return err
	}

	// Remove partitions older than the retention period
	for _, partition := range partitions {
		// Skip if it's the base directory (not a partition)
		if partition == sm.config.BaseDir {
			continue
		}

		// Extract the time from the partition name
		partitionTime, err := sm.parsePartitionTime(filepath.Base(partition))
		if err != nil {
			log.Printf("Warning: Couldn't parse partition time from %s: %v", partition, err)
			continue
		}

		// If the partition is older than the cutoff, remove it
		if partitionTime.Before(cutoffTime) {
			log.Printf("Removing old partition: %s (older than %s)", partition, cutoffTime.Format("2006-01-02"))
			if err := os.RemoveAll(partition); err != nil {
				return fmt.Errorf("failed to remove old partition %s: %v", partition, err)
			}
		} else if sm.config.CompressOldData {
			// Compress old partitions that are within retention but not current
			currentPartitionDir := sm.getCurrentPartitionDir()
			if partition != currentPartitionDir && !isCompressed(partition) {
				if err := sm.compressPartition(partition); err != nil {
					log.Printf("Warning: Failed to compress partition %s: %v", partition, err)
				}
			}
		}
	}

	return nil
}

// parsePartitionTime parses a time from a partition directory name
func (sm *StorageManager) parsePartitionTime(partitionName string) (time.Time, error) {
	// Try different formats based on the partition interval

	if strings.Contains(partitionName, "-W") {
		// Weekly format: 2023-W01
		year, week := 0, 0
		if _, err := fmt.Sscanf(partitionName, "%d-W%02d", &year, &week); err != nil {
			return time.Time{}, err
		}
		// Create a time.Time for the first day of the given week
		jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		daysSinceJan1 := (week - 1) * 7
		return jan1.AddDate(0, 0, daysSinceJan1), nil
	} else if len(partitionName) == 7 {
		// Monthly format: 2023-01
		return time.Parse("2006-01", partitionName)
	} else if len(partitionName) == 10 {
		// Daily format: 2023-01-01
		return time.Parse("2006-01-02", partitionName)
	}

	return time.Time{}, fmt.Errorf("unknown partition format: %s", partitionName)
}

// isCompressed checks if a partition is already compressed
func isCompressed(partitionDir string) bool {
	// Check if there are any .gz files in the directory
	entries, err := os.ReadDir(partitionDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gz") {
			return true
		}
	}

	return false
}

// compressPartition compresses all JSON files in a partition
func (sm *StorageManager) compressPartition(partitionDir string) error {
	// Get all JSON files in the partition
	entries, err := os.ReadDir(partitionDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(partitionDir, entry.Name())
			compressedPath := filePath + ".gz"

			// Skip if already compressed
			if _, err := os.Stat(compressedPath); err == nil {
				continue
			}

			// Open the source file
			sourceFile, err := os.Open(filePath)
			if err != nil {
				return err
			}

			// Create the compressed file
			compressedFile, err := os.Create(compressedPath)
			if err != nil {
				sourceFile.Close()
				return err
			}

			// Create a gzip writer
			gzipWriter := gzip.NewWriter(compressedFile)

			// Copy data from source to compressed file
			_, err = io.Copy(gzipWriter, sourceFile)

			// Close all resources
			gzipWriter.Close()
			compressedFile.Close()
			sourceFile.Close()

			if err != nil {
				return err
			}

			// Remove the original file
			if err := os.Remove(filePath); err != nil {
				return err
			}

			log.Printf("Compressed file: %s", filePath)
		}
	}

	return nil
}

// NewServer creates a new Govee server instance
func NewServer(config *Config, auth *AuthConfig, storageManager *StorageManager) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		devices:        make(map[string]*DeviceStatus),
		clients:        make(map[string]*ClientStatus),
		readings:       make(map[string][]Reading),
		config:         config,
		auth:           auth,
		storageManager: storageManager,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
		rateLimiter:    NewRateLimiter(),
	}

	// Initialize logging if configured
	if config.LogFile != "" {
		logger, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
		} else {
			s.logger = logger
			log.Printf("Logging data to %s", config.LogFile)
		}
	}

	// Start persistence if enabled
	if config.PersistenceEnabled {
		// Create storage directory if it doesn't exist
		if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
			log.Printf("Failed to create storage directory: %v", err)
		}

		// Start background save routine
		go s.startPersistence(ctx)
	}

	// Start client timeout check routine
	go s.checkClientTimeouts(ctx)

	return s
}

// startPersistence starts the background routine for data persistence
func (s *Server) startPersistence(ctx context.Context) {
	ticker := time.NewTicker(s.config.SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.saveData()
		case <-ctx.Done():
			log.Println("Persistence routine shutting down")
			return
		}
	}
}

// saveData saves current server state to disk (optimized to minimize lock time)
func (s *Server) saveData() {
	// Take snapshot under read lock (fast)
	s.mu.RLock()
	devicesCopy := make(map[string]*DeviceStatus, len(s.devices))
	for k, v := range s.devices {
		devicesCopy[k] = v
	}
	clientsCopy := make(map[string]*ClientStatus, len(s.clients))
	for k, v := range s.clients {
		clientsCopy[k] = v
	}
	readingsCopy := make(map[string][]Reading, len(s.readings))
	for k, v := range s.readings {
		readingsCopy[k] = v
	}
	authCopy := s.auth
	enableAuth := s.auth.EnableAuth
	s.mu.RUnlock()

	// Now perform all I/O operations without holding the lock

	// Save device statuses
	devicesData, err := json.MarshalIndent(devicesCopy, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal devices data: %v", err)
	} else {
		if err := os.WriteFile(fmt.Sprintf("%s/devices.json", s.config.StorageDir), devicesData, 0644); err != nil {
			log.Printf("Failed to save devices data: %v", err)
		}
	}

	// Save client statuses
	clientsData, err := json.MarshalIndent(clientsCopy, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal clients data: %v", err)
	} else {
		if err := os.WriteFile(fmt.Sprintf("%s/clients.json", s.config.StorageDir), clientsData, 0644); err != nil {
			log.Printf("Failed to save clients data: %v", err)
		}
	}

	// Save API keys if auth is enabled
	if enableAuth {
		authData, err := json.MarshalIndent(authCopy, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal auth data: %v", err)
		} else {
			if err := os.WriteFile(fmt.Sprintf("%s/auth.json", s.config.StorageDir), authData, 0644); err != nil {
				log.Printf("Failed to save auth data: %v", err)
			}
		}
	}

	// Save recent readings for each device using the storage manager
	for deviceAddr, deviceReadings := range readingsCopy {
		if len(deviceReadings) > 0 {
			err := s.storageManager.saveReadings(deviceAddr, deviceReadings)
			if err != nil {
				log.Printf("Failed to save readings for device %s: %v", deviceAddr, err)
			}
		}
	}

	log.Println("Data saved to storage")
}

// loadData loads server state from disk
func (s *Server) loadData() {
	// Load device statuses
	devicesData, err := os.ReadFile(fmt.Sprintf("%s/devices.json", s.config.StorageDir))
	if err == nil {
		if err := json.Unmarshal(devicesData, &s.devices); err != nil {
			log.Printf("Failed to unmarshal devices data: %v", err)
		} else {
			log.Printf("Loaded %d devices from storage", len(s.devices))
		}
	}

	// Load client statuses
	clientsData, err := os.ReadFile(fmt.Sprintf("%s/clients.json", s.config.StorageDir))
	if err == nil {
		if err := json.Unmarshal(clientsData, &s.clients); err != nil {
			log.Printf("Failed to unmarshal clients data: %v", err)
		} else {
			log.Printf("Loaded %d clients from storage", len(s.clients))
		}
	}

	// Load auth configuration if auth is enabled
	if s.auth.EnableAuth {
		authData, err := os.ReadFile(fmt.Sprintf("%s/auth.json", s.config.StorageDir))
		if err == nil {
			var loadedAuth AuthConfig
			if err := json.Unmarshal(authData, &loadedAuth); err != nil {
				log.Printf("Failed to unmarshal auth data: %v", err)
			} else {
				// Only update the API keys, preserve other settings from command line
				s.auth.APIKeys = loadedAuth.APIKeys
				log.Printf("Loaded %d API keys from storage", len(s.auth.APIKeys))
			}
		}
	}

	// Mark all clients as inactive initially
	for _, client := range s.clients {
		client.IsActive = false
	}
}

// checkClientTimeouts periodically checks for inactive clients and cleans up old data
func (s *Server) checkClientTimeouts(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()

			// Mark inactive clients
			for clientID, client := range s.clients {
				if now.Sub(client.LastSeen) > s.config.ClientTimeout {
					client.IsActive = false
					log.Printf("Client %s marked as inactive (timeout: %v)", clientID, s.config.ClientTimeout)
				}

				// Remove very old inactive clients (10x timeout)
				if now.Sub(client.LastSeen) > s.config.ClientTimeout*10 {
					delete(s.clients, clientID)
					log.Printf("Removed stale client: %s", clientID)
				}
			}

			// Clean up very old devices (30 days)
			for deviceAddr, device := range s.devices {
				if now.Sub(device.LastSeen) > 30*24*time.Hour {
					delete(s.devices, deviceAddr)
					delete(s.readings, deviceAddr)
					log.Printf("Removed stale device: %s", deviceAddr)
				}
			}

			s.mu.Unlock()

		case <-ctx.Done():
			log.Println("Client timeout checker shutting down")
			return
		}
	}
}

// addReading adds a new reading to the server
func (s *Server) addReading(reading Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceAddr := reading.DeviceAddr
	clientID := reading.ClientID

	// Track if this is a new device
	_, deviceExists := s.devices[deviceAddr]

	// Update device status
	if device, exists := s.devices[deviceAddr]; exists {
		device.TempC = reading.TempC
		device.TempF = reading.TempF
		device.TempOffset = reading.TempOffset
		device.Humidity = reading.Humidity
		device.HumidityOffset = reading.HumidityOffset
		device.AbsHumidity = reading.AbsHumidity
		device.DewPointC = reading.DewPointC
		device.DewPointF = reading.DewPointF
		device.SteamPressure = reading.SteamPressure
		device.Battery = reading.Battery
		device.RSSI = reading.RSSI
		device.LastUpdate = reading.Timestamp
		device.LastSeen = time.Now()
		device.ClientID = clientID
		device.ReadingCount++
	} else {
		s.devices[deviceAddr] = &DeviceStatus{
			DeviceName:     reading.DeviceName,
			DeviceAddr:     deviceAddr,
			TempC:          reading.TempC,
			TempF:          reading.TempF,
			TempOffset:     reading.TempOffset,
			Humidity:       reading.Humidity,
			HumidityOffset: reading.HumidityOffset,
			AbsHumidity:    reading.AbsHumidity,
			DewPointC:      reading.DewPointC,
			DewPointF:      reading.DewPointF,
			SteamPressure:  reading.SteamPressure,
			Battery:        reading.Battery,
			RSSI:           reading.RSSI,
			LastUpdate:     reading.Timestamp,
			LastSeen:       time.Now(),
			ClientID:       clientID,
			ReadingCount:   1,
		}
	}

	// Update or create client status
	if client, exists := s.clients[clientID]; exists {
		client.LastSeen = time.Now()
		client.ReadingCount++
		client.IsActive = true
	} else {
		s.clients[clientID] = &ClientStatus{
			ClientID:        clientID,
			LastSeen:        time.Now(),
			DeviceCount:     1,
			ReadingCount:    1,
			ConnectedSince:  time.Now(),
			IsActive:        true,
			InactiveTimeout: s.config.ClientTimeout,
		}
	}

	// Increment device count only when adding a new device
	// This is much more efficient than recalculating all counts every time
	if !deviceExists && clientID != "" {
		if client, exists := s.clients[clientID]; exists {
			client.DeviceCount++
		}
	}

	// Store reading
	if _, exists := s.readings[deviceAddr]; !exists {
		s.readings[deviceAddr] = make([]Reading, 0)
	}

	// Append reading and maintain maximum size
	readings := append(s.readings[deviceAddr], reading)
	if len(readings) > s.config.ReadingsPerDevice {
		readings = readings[len(readings)-s.config.ReadingsPerDevice:]
	}
	s.readings[deviceAddr] = readings

	// Log reading if logger is available
	if s.logger != nil {
		logEntry, _ := json.Marshal(reading)
		s.logger.WriteString(string(logEntry) + "\n")
	}
}

// getDevices returns all device statuses
func (s *Server) getDevices() []*DeviceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*DeviceStatus, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}
	return devices
}

// getClients returns all client statuses
func (s *Server) getClients() []*ClientStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*ClientStatus, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	return clients
}

// getDeviceReadings returns readings for a specific device with optional time range
func (s *Server) getDeviceReadings(deviceAddr string, fromTime, toTime time.Time) ([]Reading, error) {
	// First try to get from in-memory store
	s.mu.RLock()
	inMemoryReadings, exists := s.readings[deviceAddr]
	s.mu.RUnlock()

	if exists && (fromTime.IsZero() && toTime.IsZero()) {
		// If no time range is specified and readings exist in memory, return those
		return inMemoryReadings, nil
	}

	// Otherwise, use the storage manager to get readings
	return s.storageManager.loadReadings(deviceAddr, fromTime, toTime)
}

// getDeviceStats returns statistics for a specific device
func (s *Server) getDeviceStats(deviceAddr string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})
	if readings, exists := s.readings[deviceAddr]; exists && len(readings) > 0 {
		// Calculate min, max, avg for primary metrics
		var sumTempC, sumHumidity, sumAbsHumidity, sumDewPointC, sumSteamPressure float64
		var minTempC, maxTempC = readings[0].TempC, readings[0].TempC
		var minHumidity, maxHumidity = readings[0].Humidity, readings[0].Humidity
		var minDewPointC, maxDewPointC = readings[0].DewPointC, readings[0].DewPointC
		var minAbsHumidity, maxAbsHumidity = readings[0].AbsHumidity, readings[0].AbsHumidity
		var minSteamPressure, maxSteamPressure = readings[0].SteamPressure, readings[0].SteamPressure

		for _, r := range readings {
			sumTempC += r.TempC
			sumHumidity += r.Humidity
			sumDewPointC += r.DewPointC
			sumAbsHumidity += r.AbsHumidity
			sumSteamPressure += r.SteamPressure

			if r.TempC < minTempC {
				minTempC = r.TempC
			}
			if r.TempC > maxTempC {
				maxTempC = r.TempC
			}
			if r.Humidity < minHumidity {
				minHumidity = r.Humidity
			}
			if r.Humidity > maxHumidity {
				maxHumidity = r.Humidity
			}
			if r.DewPointC < minDewPointC {
				minDewPointC = r.DewPointC
			}
			if r.DewPointC > maxDewPointC {
				maxDewPointC = r.DewPointC
			}
			if r.AbsHumidity < minAbsHumidity {
				minAbsHumidity = r.AbsHumidity
			}
			if r.AbsHumidity > maxAbsHumidity {
				maxAbsHumidity = r.AbsHumidity
			}
			if r.SteamPressure < minSteamPressure {
				minSteamPressure = r.SteamPressure
			}
			if r.SteamPressure > maxSteamPressure {
				maxSteamPressure = r.SteamPressure
			}
		}

		count := float64(len(readings))
		stats["count"] = len(readings)

		// Temperature stats
		stats["temp_c_min"] = minTempC
		stats["temp_c_max"] = maxTempC
		stats["temp_c_avg"] = sumTempC / count

		// Humidity stats
		stats["humidity_min"] = minHumidity
		stats["humidity_max"] = maxHumidity
		stats["humidity_avg"] = sumHumidity / count

		// Dew point stats
		stats["dew_point_c_min"] = minDewPointC
		stats["dew_point_c_max"] = maxDewPointC
		stats["dew_point_c_avg"] = sumDewPointC / count

		// Absolute humidity stats
		stats["abs_humidity_min"] = minAbsHumidity
		stats["abs_humidity_max"] = maxAbsHumidity
		stats["abs_humidity_avg"] = sumAbsHumidity / count

		// Steam pressure stats
		stats["steam_pressure_min"] = minSteamPressure
		stats["steam_pressure_max"] = maxSteamPressure
		stats["steam_pressure_avg"] = sumSteamPressure / count

		// Add first and last readings timestamps
		stats["first_reading"] = readings[0].Timestamp
		stats["last_reading"] = readings[len(readings)-1].Timestamp
	}
	return stats
}

// rateLimitMiddleware enforces rate limiting per IP address
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for health checks
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Get IP address (handle X-Forwarded-For for proxies)
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}

		limiter := s.rateLimiter.GetLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			log.Printf("Rate limit exceeded for IP: %s", ip)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Authentication middleware
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if disabled
		if !s.auth.EnableAuth {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for GET requests to public endpoints
		if r.Method == "GET" && (r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/js/") ||
			strings.HasPrefix(r.URL.Path, "/css/") ||
			strings.HasPrefix(r.URL.Path, "/img/") ||
			r.URL.Path == "/health") {
			next.ServeHTTP(w, r)
			return
		}

		// Check for API key in header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
			log.Printf("Authentication failed: No API key provided from %s", r.RemoteAddr)
			return
		}

		// Check if it's the admin key
		if apiKey == s.auth.AdminKey {
			// Admin key has access to everything
			next.ServeHTTP(w, r)
			return
		}

		// Check if it's the default key (if allowed)
		if s.auth.AllowDefaultKey && apiKey == s.auth.DefaultAPIKey {
			next.ServeHTTP(w, r)
			return
		}

		// Check if the API key is valid
		clientID, valid := s.auth.APIKeys[apiKey]
		if !valid {
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			log.Printf("Authentication failed from %s", r.RemoteAddr)
			return
		}

		// For POST to /readings, validate client ID and preserve request body
		if r.Method == "POST" && r.URL.Path == "/readings" {
			// Read body once
			bodyBytes, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				log.Printf("Failed to read request body: %v", err)
				return
			}

			// Parse JSON
			var reading Reading
			if err := json.Unmarshal(bodyBytes, &reading); err != nil {
				http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
				log.Printf("Invalid JSON from %s: %v", r.RemoteAddr, err)
				return
			}

			// Validate client ID matches API key
			if reading.ClientID != clientID {
				http.Error(w, "Unauthorized: Client ID mismatch", http.StatusUnauthorized)
				log.Printf("Client ID mismatch from %s", r.RemoteAddr)
				return
			}

			// Restore body for handler
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// API key is valid
		next.ServeHTTP(w, r)
	})
}

// handlers for HTTP endpoints

func (s *Server) handleReadings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Add a new reading
		var reading Reading
		if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate reading
		if err := validateReading(&reading); err != nil {
			http.Error(w, fmt.Sprintf("Invalid reading: %v", err), http.StatusBadRequest)
			log.Printf("Invalid reading from %s: %v", r.RemoteAddr, err)
			return
		}

		s.addReading(reading)
		w.WriteHeader(http.StatusCreated)

	case "GET":
		// Get readings for a specific device with optional time range
		deviceAddr := r.URL.Query().Get("device")
		if deviceAddr == "" {
			http.Error(w, "Missing device parameter", http.StatusBadRequest)
			return
		}

		// Parse time range parameters
		fromTimeStr := r.URL.Query().Get("from")
		toTimeStr := r.URL.Query().Get("to")

		var fromTime, toTime time.Time
		var err error

		if fromTimeStr != "" {
			fromTime, err = time.Parse(time.RFC3339, fromTimeStr)
			if err != nil {
				http.Error(w, "Invalid 'from' time format. Use RFC3339 format (e.g., 2023-04-10T15:04:05Z)", http.StatusBadRequest)
				return
			}
		}

		if toTimeStr != "" {
			toTime, err = time.Parse(time.RFC3339, toTimeStr)
			if err != nil {
				http.Error(w, "Invalid 'to' time format. Use RFC3339 format (e.g., 2023-04-10T15:04:05Z)", http.StatusBadRequest)
				return
			}
		}

		readings, err := s.getDeviceReadings(deviceAddr, fromTime, toTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error loading readings: %v", err), http.StatusInternalServerError)
			return
		}

		respondJSON(w, readings)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	devices := s.getDevices()
	respondJSON(w, devices)
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clients := s.getClients()
	respondJSON(w, clients)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceAddr := r.URL.Query().Get("device")
	if deviceAddr == "" {
		http.Error(w, "Missing device parameter", http.StatusBadRequest)
		return
	}

	stats := s.getDeviceStats(deviceAddr)
	respondJSON(w, stats)
}

func (s *Server) handleDashboardData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Prepare dashboard data
	dashboardData := struct {
		Devices         []*DeviceStatus      `json:"devices"`
		Clients         []*ClientStatus      `json:"clients"`
		ActiveClients   int                  `json:"active_clients"`
		TotalReadings   int                  `json:"total_readings"`
		RecentReadings  map[string][]Reading `json:"recent_readings"`
		ServerStartTime time.Time            `json:"server_start_time"`
	}{
		Devices:         make([]*DeviceStatus, 0, len(s.devices)),
		Clients:         make([]*ClientStatus, 0, len(s.clients)),
		RecentReadings:  make(map[string][]Reading),
		ServerStartTime: time.Now().Add(-time.Since(time.Now())), // Get server start time
	}

	// Add devices
	for _, device := range s.devices {
		dashboardData.Devices = append(dashboardData.Devices, device)
	}

	// Add clients and count active ones
	totalReadings := 0
	for _, client := range s.clients {
		dashboardData.Clients = append(dashboardData.Clients, client)
		if client.IsActive {
			dashboardData.ActiveClients++
		}
		totalReadings += client.ReadingCount
	}
	dashboardData.TotalReadings = totalReadings

	// Add recent readings (last 10 for each device)
	for addr, readings := range s.readings {
		if len(readings) > 0 {
			end := len(readings)
			start := end - 10
			if start < 0 {
				start = 0
			}
			dashboardData.RecentReadings[addr] = readings[start:end]
		}
	}

	respondJSON(w, dashboardData)
}

// handleAPIKeys handles API key management
func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	// This endpoint requires admin API key (checked in middleware)
	switch r.Method {
	case "GET":
		// List all API keys (except admin key)
		keys := make(map[string]string)
		for k, v := range s.auth.APIKeys {
			keys[k] = v
		}
		respondJSON(w, keys)

	case "POST":
		// Create new API key
		var keyData struct {
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&keyData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if keyData.ClientID == "" {
			http.Error(w, "Client ID is required", http.StatusBadRequest)
			return
		}

		// Generate a new API key
		newKey := generateAPIKey()

		s.mu.Lock()
		s.auth.APIKeys[newKey] = keyData.ClientID
		s.mu.Unlock()

		// Save auth data if persistence is enabled
		if s.config.PersistenceEnabled {
			s.saveData()
		}

		// Return the new key
		w.WriteHeader(http.StatusCreated)
		respondJSON(w, map[string]string{
			"api_key":   newKey,
			"client_id": keyData.ClientID,
		})

	case "DELETE":
		// Delete API key
		apiKeyToDelete := r.URL.Query().Get("key")
		if apiKeyToDelete == "" {
			http.Error(w, "Missing key parameter", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		if _, exists := s.auth.APIKeys[apiKeyToDelete]; exists {
			delete(s.auth.APIKeys, apiKeyToDelete)
			s.mu.Unlock()

			// Save auth data if persistence is enabled
			if s.config.PersistenceEnabled {
				s.saveData()
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("API key deleted"))
		} else {
			s.mu.Unlock()
			http.Error(w, "API key not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHealthCheck handles health check requests
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// generateAPIKey creates a new cryptographically secure random API key
func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate random API key: %v", err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

// respondJSON encodes data as JSON and handles errors properly
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
		// Response already started, can't change status code
	}
}

// handleStaticFiles serves the static files for the dashboard
func handleStaticFiles(dir string) http.Handler {
	return http.FileServer(http.Dir(dir))
}

func main() {
	// Parse command-line flags
	port := flag.Int("port", 8080, "server port")
	logFile := flag.String("log", "govee_server.log", "log file path")
	staticDir := flag.String("static", "./static", "static files directory")
	storageDir := flag.String("storage", "./data", "data storage directory")
	clientTimeout := flag.Duration("timeout", 5*time.Minute, "client inactivity timeout")
	readingsPerDevice := flag.Int("readings", 1000, "max readings to store per device")
	persistenceEnabled := flag.Bool("persist", true, "enable data persistence")
	saveInterval := flag.Duration("save-interval", 5*time.Minute, "interval for saving data")

	// Authentication flags
	enableAuth := flag.Bool("auth", true, "enable API key authentication")
	adminKey := flag.String("admin-key", "", "admin API key (generated if empty)")
	defaultKey := flag.String("default-key", "", "default API key for all clients (generated if empty)")
	allowDefaultKey := flag.Bool("allow-default", false, "allow the default API key to be used")

	// HTTPS flags
	enableHTTPS := flag.Bool("https", false, "enable HTTPS")
	certFile := flag.String("cert", "cert.pem", "path to TLS certificate file")
	keyFile := flag.String("key", "key.pem", "path to TLS key file")

	// Storage and retention flags
	timePartitioning := flag.Bool("time-partition", true, "enable time-based partitioning of data")
	partitionInterval := flag.Duration("partition-interval", 30*24*time.Hour, "interval for creating new partitions (e.g., 24h, 720h)")
	retentionPeriod := flag.Duration("retention", 0, "data retention period, 0 for unlimited (e.g., 8760h for 1 year)")
	maxReadingsPerFile := flag.Int("max-file-readings", 1000, "maximum readings per file")
	compressOldData := flag.Bool("compress", true, "compress older partitions to save space")

	flag.Parse()

	// Create authentication configuration
	auth := &AuthConfig{
		EnableAuth:      *enableAuth,
		APIKeys:         make(map[string]string),
		AllowDefaultKey: *allowDefaultKey,
	}

	// Generate admin key if not provided
	if auth.EnableAuth {
		if *adminKey == "" {
			auth.AdminKey = generateAPIKey()
			log.Printf("Generated admin API key: %s... (copy from data/auth.json)", auth.AdminKey[:12])
		} else {
			auth.AdminKey = *adminKey
			log.Printf("Using provided admin API key: %s...", auth.AdminKey[:12])
		}

		// Generate default key if allowed and not provided
		if auth.AllowDefaultKey {
			if *defaultKey == "" {
				auth.DefaultAPIKey = generateAPIKey()
				log.Printf("Generated default API key: %s... (copy from data/auth.json)", auth.DefaultAPIKey[:12])
			} else {
				auth.DefaultAPIKey = *defaultKey
				log.Printf("Using provided default API key: %s...", auth.DefaultAPIKey[:12])
			}
		}
	}

	// Create server configuration
	config := &Config{
		Port:               *port,
		LogFile:            *logFile,
		ClientTimeout:      *clientTimeout,
		ReadingsPerDevice:  *readingsPerDevice,
		StorageDir:         *storageDir,
		PersistenceEnabled: *persistenceEnabled,
		SaveInterval:       *saveInterval,
		EnableHTTPS:        *enableHTTPS,
		CertFile:           *certFile,
		KeyFile:            *keyFile,
	}

	// Create storage configuration
	storageConfig := &StorageConfig{
		BaseDir:            *storageDir,
		TimePartitioning:   *timePartitioning,
		PartitionInterval:  *partitionInterval,
		RetentionPeriod:    *retentionPeriod,
		MaxReadingsPerFile: *maxReadingsPerFile,
		CompressOldData:    *compressOldData,
	}

	// Create storage manager
	storageManager := NewStorageManager(storageConfig)

	// Create and initialize server
	server := NewServer(config, auth, storageManager)

	// Load data from storage if enabled
	if config.PersistenceEnabled {
		server.loadData()
	}

	// Start a routine to periodically enforce retention
	go func() {
		retentionTicker := time.NewTicker(24 * time.Hour) // Check retention daily
		defer retentionTicker.Stop()

		for {
			select {
			case <-retentionTicker.C:
				if err := storageManager.enforceRetention(); err != nil {
					log.Printf("Error enforcing retention: %v", err)
				}
			case <-server.shutdownCtx.Done():
				log.Println("Retention routine shutting down")
				return
			}
		}
	}()

	// Create HTTP server
	mux := http.NewServeMux()

	// Create middleware chain: rate limit -> auth
	rateLimitMiddleware := server.rateLimitMiddleware
	authMiddleware := server.authMiddleware

	// API endpoints with rate limiting and authentication
	mux.Handle("/readings", rateLimitMiddleware(authMiddleware(http.HandlerFunc(server.handleReadings))))
	mux.Handle("/devices", rateLimitMiddleware(authMiddleware(http.HandlerFunc(server.handleDevices))))
	mux.Handle("/clients", rateLimitMiddleware(authMiddleware(http.HandlerFunc(server.handleClients))))
	mux.Handle("/stats", rateLimitMiddleware(authMiddleware(http.HandlerFunc(server.handleStats))))
	mux.Handle("/dashboard/data", rateLimitMiddleware(authMiddleware(http.HandlerFunc(server.handleDashboardData))))
	mux.Handle("/api/keys", rateLimitMiddleware(authMiddleware(http.HandlerFunc(server.handleAPIKeys))))
	mux.Handle("/health", rateLimitMiddleware(http.HandlerFunc(server.handleHealthCheck)))

	// Serve static files for dashboard
	mux.Handle("/", handleStaticFiles(*staticDir))

	var httpServer *http.Server

	if *enableHTTPS {
		// Check if certificate files exist
		certPath := *certFile
		keyPath := *keyFile

		if !filepath.IsAbs(certPath) {
			certPath = filepath.Join(*storageDir, certPath)
		}

		if !filepath.IsAbs(keyPath) {
			keyPath = filepath.Join(*storageDir, keyPath)
		}

		// Create HTTPS server
		httpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}

		log.Printf("Starting Govee Server with HTTPS on port %d", config.Port)
		log.Printf("Using certificate: %s", certPath)
		log.Printf("Using key: %s", keyPath)

		// Start server in a goroutine
		go func() {
			if err := httpServer.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		}()
	} else {
		// Create HTTP server
		httpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		// Start server in a goroutine
		go func() {
			log.Printf("Starting Govee Server on port %d", config.Port)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	log.Println("Shutting down server...")

	// Stop background goroutines
	server.shutdownCancel()

	// Save data before shutting down
	if config.PersistenceEnabled {
		server.saveData()
	}

	// Create a deadline for HTTP server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server shutdown complete")
}
