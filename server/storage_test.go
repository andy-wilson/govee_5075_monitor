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
