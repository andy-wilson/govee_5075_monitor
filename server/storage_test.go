package main

import (
	"path/filepath"
	"testing"
	"time"
)

// TestSQLiteStorage tests the SQLite storage backend
func TestSQLiteStorage(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage := NewSQLiteStorage(dbPath)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Test data
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{
			DeviceName:  "Test Sensor 1",
			DeviceAddr:  deviceAddr,
			TempC:       25.5,
			TempF:       77.9,
			Humidity:    60.0,
			Battery:     85,
			RSSI:        -50,
			Timestamp:   time.Now().Add(-1 * time.Hour),
			ClientID:    "test-client",
		},
		{
			DeviceName:  "Test Sensor 1",
			DeviceAddr:  deviceAddr,
			TempC:       26.0,
			TempF:       78.8,
			Humidity:    62.0,
			Battery:     84,
			RSSI:        -52,
			Timestamp:   time.Now(),
			ClientID:    "test-client",
		},
	}

	// Test SaveReadings
	if err := storage.SaveReadings(deviceAddr, readings); err != nil {
		t.Fatalf("Failed to save readings: %v", err)
	}

	// Test LoadAllDeviceReadings
	loaded, err := storage.LoadAllDeviceReadings(deviceAddr)
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	if len(loaded) != len(readings) {
		t.Errorf("Expected %d readings, got %d", len(readings), len(loaded))
	}

	// Test GetDevices
	devices, err := storage.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}

	if devices[0] != deviceAddr {
		t.Errorf("Expected device %s, got %s", deviceAddr, devices[0])
	}

	// Test GetReadingCount
	count, err := storage.GetReadingCount()
	if err != nil {
		t.Fatalf("Failed to get reading count: %v", err)
	}

	if count != int64(len(readings)) {
		t.Errorf("Expected count %d, got %d", len(readings), count)
	}

	// Test GetReadingCountByDevice
	deviceCount, err := storage.GetReadingCountByDevice(deviceAddr)
	if err != nil {
		t.Fatalf("Failed to get device reading count: %v", err)
	}

	if deviceCount != int64(len(readings)) {
		t.Errorf("Expected device count %d, got %d", len(readings), deviceCount)
	}

	// Test GetLatestReadings
	latest, err := storage.GetLatestReadings(1)
	if err != nil {
		t.Fatalf("Failed to get latest readings: %v", err)
	}

	if len(latest) != 1 {
		t.Errorf("Expected 1 latest reading, got %d", len(latest))
	}

	// Test GetReadingsPage
	page, total, err := storage.GetReadingsPage(0, 10, deviceAddr, "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Failed to get readings page: %v", err)
	}

	if total != int64(len(readings)) {
		t.Errorf("Expected total %d, got %d", len(readings), total)
	}

	if len(page) != len(readings) {
		t.Errorf("Expected %d readings in page, got %d", len(readings), len(page))
	}

	// Test DeleteOldReadings
	cutoff := time.Now().Add(-30 * time.Minute)
	if err := storage.DeleteOldReadings(cutoff); err != nil {
		t.Fatalf("Failed to delete old readings: %v", err)
	}

	// Verify deletion
	afterDelete, err := storage.GetReadingCount()
	if err != nil {
		t.Fatalf("Failed to get count after delete: %v", err)
	}

	if afterDelete >= count {
		t.Errorf("Expected fewer readings after delete, got %d", afterDelete)
	}

	t.Logf("SQLite storage tests passed")
}

// TestJSONStorage tests the JSON storage backend
func TestJSONStorage(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	storage := NewJSONStorage(tmpDir)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Test data
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{
			DeviceName: "Test Sensor 1",
			DeviceAddr: deviceAddr,
			TempC:      25.5,
			TempF:      77.9,
			Humidity:   60.0,
			Battery:    85,
			RSSI:       -50,
			Timestamp:  time.Now().Add(-1 * time.Hour),
			ClientID:   "test-client",
		},
		{
			DeviceName: "Test Sensor 1",
			DeviceAddr: deviceAddr,
			TempC:      26.0,
			TempF:      78.8,
			Humidity:   62.0,
			Battery:    84,
			RSSI:       -52,
			Timestamp:  time.Now(),
			ClientID:   "test-client",
		},
	}

	// Test SaveReadings
	if err := storage.SaveReadings(deviceAddr, readings); err != nil {
		t.Fatalf("Failed to save readings: %v", err)
	}

	// Verify file was created
	files, err := filepath.Glob(filepath.Join(tmpDir, "readings_*.json"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	// Test LoadAllDeviceReadings
	loaded, err := storage.LoadAllDeviceReadings(deviceAddr)
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	if len(loaded) != len(readings) {
		t.Errorf("Expected %d readings, got %d", len(readings), len(loaded))
	}

	// Test GetDevices
	devices, err := storage.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}

	// Test GetReadingCount
	count, err := storage.GetReadingCount()
	if err != nil {
		t.Fatalf("Failed to get reading count: %v", err)
	}

	if count != int64(len(readings)) {
		t.Errorf("Expected count %d, got %d", len(readings), count)
	}

	t.Logf("JSON storage tests passed")
}

// TestStorageBackendInterface ensures both implementations satisfy the interface
func TestStorageBackendInterface(t *testing.T) {
	var _ StorageBackend = (*SQLiteStorage)(nil)
	var _ StorageBackend = (*JSONStorage)(nil)

	t.Log("Both storage backends implement StorageBackend interface")
}

// BenchmarkSQLiteInsert benchmarks SQLite insert performance
func BenchmarkSQLiteInsert(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	storage := NewSQLiteStorage(dbPath)
	if err := storage.Initialize(); err != nil {
		b.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	deviceAddr := "AA:BB:CC:DD:EE:FF"
	reading := Reading{
		DeviceName: "Test Sensor",
		DeviceAddr: deviceAddr,
		TempC:      25.5,
		Humidity:   60.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test-client",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.SaveReadings(deviceAddr, []Reading{reading})
	}
}

// BenchmarkSQLiteQuery benchmarks SQLite query performance
func BenchmarkSQLiteQuery(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	storage := NewSQLiteStorage(dbPath)
	if err := storage.Initialize(); err != nil {
		b.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Insert test data
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	for i := 0; i < 1000; i++ {
		reading := Reading{
			DeviceName: "Test Sensor",
			DeviceAddr: deviceAddr,
			TempC:      25.5,
			Humidity:   60.0,
			Battery:    85,
			Timestamp:  time.Now().Add(time.Duration(-i) * time.Minute),
			ClientID:   "test-client",
		}
		storage.SaveReadings(deviceAddr, []Reading{reading})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.LoadAllDeviceReadings(deviceAddr)
	}
}

// BenchmarkJSONInsert benchmarks JSON insert performance
func BenchmarkJSONInsert(b *testing.B) {
	tmpDir := b.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	deviceAddr := "AA:BB:CC:DD:EE:FF"
	reading := Reading{
		DeviceName: "Test Sensor",
		DeviceAddr: deviceAddr,
		TempC:      25.5,
		Humidity:   60.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test-client",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.SaveReadings(deviceAddr, []Reading{reading})
	}
}

// BenchmarkJSONQuery benchmarks JSON query performance
func BenchmarkJSONQuery(b *testing.B) {
	tmpDir := b.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	// Insert test data
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	var readings []Reading
	for i := 0; i < 1000; i++ {
		readings = append(readings, Reading{
			DeviceName: "Test Sensor",
			DeviceAddr: deviceAddr,
			TempC:      25.5,
			Humidity:   60.0,
			Battery:    85,
			Timestamp:  time.Now().Add(time.Duration(-i) * time.Minute),
			ClientID:   "test-client",
		})
	}
	storage.SaveReadings(deviceAddr, readings)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.LoadAllDeviceReadings(deviceAddr)
	}
}

// TestSQLiteLoadReadingsTimeRange tests loading with time range filter
func TestSQLiteLoadReadingsTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage := NewSQLiteStorage(dbPath)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 20.0, Timestamp: now.Add(-3 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 25.0, Timestamp: now.Add(-1 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 30.0, Timestamp: now, ClientID: "test"},
	}
	storage.SaveReadings(deviceAddr, readings)

	// Load only last 2 hours
	fromTime := now.Add(-2 * time.Hour)
	toTime := now.Add(1 * time.Hour)
	loaded, err := storage.LoadReadings(deviceAddr, fromTime, toTime)
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("Expected 2 readings in time range, got %d", len(loaded))
	}
}

// TestSQLiteGetHourlyAggregates tests hourly aggregation
func TestSQLiteGetHourlyAggregates(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage := NewSQLiteStorage(dbPath)
	storage.Initialize()
	defer storage.Close()

	// Insert data and pre-compute aggregates into the hourly_aggregates table
	baseTime := time.Date(2023, 6, 15, 14, 0, 0, 0, time.UTC)
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 20.0, Humidity: 40.0, Timestamp: baseTime.Add(10 * time.Minute), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 22.0, Humidity: 42.0, Timestamp: baseTime.Add(20 * time.Minute), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 24.0, Humidity: 44.0, Timestamp: baseTime.Add(30 * time.Minute), ClientID: "test"},
	}
	storage.SaveReadings(deviceAddr, readings)

	// Insert pre-computed aggregate directly into the table
	storage.db.Exec(`
		INSERT INTO hourly_aggregates
		(device_addr, hour_timestamp, avg_temp_c, min_temp_c, max_temp_c, avg_humidity, min_humidity, max_humidity, count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deviceAddr, baseTime, 22.0, 20.0, 24.0, 42.0, 40.0, 44.0, 3)

	fromTime := baseTime
	toTime := baseTime.Add(1 * time.Hour)
	aggregates, err := storage.GetHourlyAggregates(deviceAddr, fromTime, toTime)
	if err != nil {
		t.Fatalf("Failed to get hourly aggregates: %v", err)
	}

	if len(aggregates) == 0 {
		t.Error("Expected at least 1 aggregate")
	} else {
		agg := aggregates[0]
		if agg.Count != 3 {
			t.Errorf("Expected count 3, got %d", agg.Count)
		}
		if agg.MinTempC != 20.0 {
			t.Errorf("Expected min temp 20.0, got %f", agg.MinTempC)
		}
		if agg.MaxTempC != 24.0 {
			t.Errorf("Expected max temp 24.0, got %f", agg.MaxTempC)
		}
	}
}

// TestSQLiteGetReadingsPageFilters tests page filtering
func TestSQLiteGetReadingsPageFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage := NewSQLiteStorage(dbPath)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	// Add readings for different devices and clients
	storage.SaveReadings("DEVICE1", []Reading{
		{DeviceName: "D1", DeviceAddr: "DEVICE1", TempC: 25.0, Timestamp: now, ClientID: "client1"},
	})
	storage.SaveReadings("DEVICE2", []Reading{
		{DeviceName: "D2", DeviceAddr: "DEVICE2", TempC: 26.0, Timestamp: now, ClientID: "client2"},
	})

	// Filter by device
	page, total, _ := storage.GetReadingsPage(0, 10, "DEVICE1", "", time.Time{}, time.Time{})
	if total != 1 || len(page) != 1 {
		t.Errorf("Device filter failed: total=%d, page=%d", total, len(page))
	}

	// Filter by client
	page, total, _ = storage.GetReadingsPage(0, 10, "", "client2", time.Time{}, time.Time{})
	if total != 1 || len(page) != 1 {
		t.Errorf("Client filter failed: total=%d, page=%d", total, len(page))
	}

	// Filter by time range
	fromTime := now.Add(-1 * time.Hour)
	toTime := now.Add(1 * time.Hour)
	page, total, _ = storage.GetReadingsPage(0, 10, "", "", fromTime, toTime)
	if total != 2 {
		t.Errorf("Time filter failed: expected 2, got %d", total)
	}
}

// TestJSONLoadReadingsTimeRange tests JSON time-filtered loading
func TestJSONLoadReadingsTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 20.0, Timestamp: now.Add(-3 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 25.0, Timestamp: now.Add(-1 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 30.0, Timestamp: now, ClientID: "test"},
	}
	storage.SaveReadings(deviceAddr, readings)

	// Load only last 2 hours
	fromTime := now.Add(-2 * time.Hour)
	toTime := now.Add(1 * time.Hour)
	loaded, err := storage.LoadReadings(deviceAddr, fromTime, toTime)
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("Expected 2 readings in time range, got %d", len(loaded))
	}
}

// TestJSONGetHourlyAggregates tests JSON hourly aggregation
func TestJSONGetHourlyAggregates(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	// Use a fixed hour for consistent testing
	baseHour := time.Date(2023, 6, 15, 14, 0, 0, 0, time.UTC)
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 20.0, Humidity: 40.0, Timestamp: baseHour.Add(10 * time.Minute), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 22.0, Humidity: 42.0, Timestamp: baseHour.Add(20 * time.Minute), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 24.0, Humidity: 44.0, Timestamp: baseHour.Add(30 * time.Minute), ClientID: "test"},
	}
	storage.SaveReadings(deviceAddr, readings)

	fromTime := baseHour
	toTime := baseHour.Add(1 * time.Hour)
	aggregates, err := storage.GetHourlyAggregates(deviceAddr, fromTime, toTime)
	if err != nil {
		t.Fatalf("Failed to get hourly aggregates: %v", err)
	}

	if len(aggregates) == 0 {
		t.Error("Expected at least 1 hourly aggregate")
	} else {
		// All 3 readings are in the same hour
		agg := aggregates[0]
		if agg.Count != 3 {
			t.Errorf("Expected count 3, got %d", agg.Count)
		}
	}
}

// TestJSONGetLatestReadings tests JSON latest readings
func TestJSONGetLatestReadings(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 20.0, Timestamp: now.Add(-3 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 25.0, Timestamp: now.Add(-2 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 30.0, Timestamp: now, ClientID: "test"},
	}
	storage.SaveReadings(deviceAddr, readings)

	latest, err := storage.GetLatestReadings(2)
	if err != nil {
		t.Fatalf("Failed to get latest readings: %v", err)
	}

	if len(latest) != 2 {
		t.Errorf("Expected 2 latest readings, got %d", len(latest))
	}

	// Should be ordered by timestamp descending
	if latest[0].TempC != 30.0 {
		t.Errorf("Expected most recent reading (30.0C), got %f", latest[0].TempC)
	}
}

// TestJSONGetReadingsPage tests JSON pagination
func TestJSONGetReadingsPage(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	var readings []Reading
	for i := 0; i < 10; i++ {
		readings = append(readings, Reading{
			DeviceName: "Test",
			DeviceAddr: deviceAddr,
			TempC:      float64(20 + i),
			Timestamp:  now.Add(time.Duration(-i) * time.Hour),
			ClientID:   "test",
		})
	}
	storage.SaveReadings(deviceAddr, readings)

	// Get page 1
	page1, total, err := storage.GetReadingsPage(0, 5, "", "", now.Add(-24*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Failed to get readings page: %v", err)
	}

	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}

	if len(page1) != 5 {
		t.Errorf("Expected 5 readings in page, got %d", len(page1))
	}

	// Get page 2
	page2, _, _ := storage.GetReadingsPage(5, 5, "", "", now.Add(-24*time.Hour), now.Add(time.Hour))
	if len(page2) != 5 {
		t.Errorf("Expected 5 readings in page 2, got %d", len(page2))
	}

	// Test offset beyond results
	page3, _, _ := storage.GetReadingsPage(100, 5, "", "", now.Add(-24*time.Hour), now.Add(time.Hour))
	if len(page3) != 0 {
		t.Errorf("Expected 0 readings for offset beyond results, got %d", len(page3))
	}
}

// TestJSONGetReadingsPageWithFilters tests JSON filtered pagination
func TestJSONGetReadingsPageWithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	device1 := "AA:BB:CC:DD:EE:01"
	device2 := "AA:BB:CC:DD:EE:02"
	storage.SaveReadings(device1, []Reading{
		{DeviceName: "D1", DeviceAddr: device1, TempC: 25.0, Timestamp: now, ClientID: "client1"},
	})
	storage.SaveReadings(device2, []Reading{
		{DeviceName: "D2", DeviceAddr: device2, TempC: 26.0, Timestamp: now, ClientID: "client2"},
	})

	// Filter by device
	page, total, _ := storage.GetReadingsPage(0, 10, device1, "", now.Add(-time.Hour), now.Add(time.Hour))
	if total != 1 || len(page) != 1 {
		t.Errorf("Device filter failed: total=%d, page=%d", total, len(page))
	}

	// Filter by client
	page, total, _ = storage.GetReadingsPage(0, 10, "", "client2", now.Add(-time.Hour), now.Add(time.Hour))
	if total != 1 || len(page) != 1 {
		t.Errorf("Client filter failed: total=%d, page=%d", total, len(page))
	}
}

// TestJSONDeleteOldReadings tests JSON retention cleanup
func TestJSONDeleteOldReadings(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 20.0, Timestamp: now.Add(-48 * time.Hour), ClientID: "test"},
		{DeviceName: "Test", DeviceAddr: deviceAddr, TempC: 25.0, Timestamp: now.Add(-1 * time.Hour), ClientID: "test"},
	}
	storage.SaveReadings(deviceAddr, readings)

	// Delete readings older than 24 hours
	cutoff := now.Add(-24 * time.Hour)
	err := storage.DeleteOldReadings(cutoff)
	if err != nil {
		t.Fatalf("Failed to delete old readings: %v", err)
	}

	loaded, _ := storage.LoadAllDeviceReadings(deviceAddr)
	if len(loaded) != 1 {
		t.Errorf("Expected 1 reading after cleanup, got %d", len(loaded))
	}
}

// TestJSONGetReadingCountByDevice tests per-device count
func TestJSONGetReadingCountByDevice(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	now := time.Now()
	device1 := "AA:BB:CC:DD:EE:01"
	device2 := "AA:BB:CC:DD:EE:02"
	storage.SaveReadings(device1, []Reading{
		{DeviceName: "D1", DeviceAddr: device1, TempC: 25.0, Timestamp: now, ClientID: "test"},
		{DeviceName: "D1", DeviceAddr: device1, TempC: 26.0, Timestamp: now, ClientID: "test"},
	})
	storage.SaveReadings(device2, []Reading{
		{DeviceName: "D2", DeviceAddr: device2, TempC: 27.0, Timestamp: now, ClientID: "test"},
	})

	count1, _ := storage.GetReadingCountByDevice(device1)
	if count1 != 2 {
		t.Errorf("Expected 2 readings for device1, got %d", count1)
	}

	count2, _ := storage.GetReadingCountByDevice(device2)
	if count2 != 1 {
		t.Errorf("Expected 1 reading for device2, got %d", count2)
	}
}

// TestJSONLoadNonExistent tests loading from non-existent device
func TestJSONLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()
	defer storage.Close()

	// Use valid MAC format for non-existent device
	loaded, err := storage.LoadAllDeviceReadings("00:00:00:00:00:00")
	if err != nil {
		t.Errorf("Expected no error for non-existent device: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("Expected 0 readings, got %d", len(loaded))
	}
}

// TestSQLiteClose tests closing SQLite storage
func TestSQLiteClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage := NewSQLiteStorage(dbPath)
	storage.Initialize()

	err := storage.Close()
	if err != nil {
		t.Errorf("Failed to close storage: %v", err)
	}

	// Double close should handle nil db
	err = storage.Close()
	// Should not panic
}

// TestJSONClose tests closing JSON storage
func TestJSONClose(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewJSONStorage(tmpDir)
	storage.Initialize()

	err := storage.Close()
	if err != nil {
		t.Errorf("Close should return nil: %v", err)
	}
}
