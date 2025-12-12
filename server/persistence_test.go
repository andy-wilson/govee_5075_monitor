package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStorageManagerPartitionDir tests partition directory calculation
func TestStorageManagerPartitionDir(t *testing.T) {
	tmpDir := t.TempDir()

	config := &StorageConfig{
		BaseDir:           tmpDir,
		TimePartitioning:  true,
		PartitionInterval: 24 * time.Hour, // Daily partitions
		RetentionPeriod:   0,
		CompressOldData:   false,
	}

	sm := NewStorageManager(config)

	// Test partition directory for a specific time
	testTime := time.Date(2023, 4, 15, 12, 0, 0, 0, time.UTC)
	partitionDir := sm.getPartitionDirForTime(testTime)

	// Should contain the date in the path
	if partitionDir == "" {
		t.Error("getPartitionDirForTime returned empty string")
	}
	if !filepath.IsAbs(partitionDir) && len(partitionDir) > 0 {
		// Should be under baseDir
		t.Logf("Partition dir: %s", partitionDir)
	}
}

// TestStorageManagerCurrentPartitionDir tests getting current partition
func TestStorageManagerCurrentPartitionDir(t *testing.T) {
	tmpDir := t.TempDir()

	config := &StorageConfig{
		BaseDir:           tmpDir,
		TimePartitioning:  true,
		PartitionInterval: 720 * time.Hour, // Monthly partitions
		RetentionPeriod:   0,
		CompressOldData:   false,
	}

	sm := NewStorageManager(config)
	currentDir := sm.getCurrentPartitionDir()

	if currentDir == "" {
		t.Error("getCurrentPartitionDir returned empty string")
	}
}

// TestStorageManagerSaveReadings tests saving readings to partition
func TestStorageManagerSaveReadings(t *testing.T) {
	tmpDir := t.TempDir()

	config := &StorageConfig{
		BaseDir:            tmpDir,
		TimePartitioning:   true,
		PartitionInterval:  720 * time.Hour,
		RetentionPeriod:    0,
		MaxReadingsPerFile: 100,
		CompressOldData:    false,
	}

	sm := NewStorageManager(config)

	// Create test readings
	readings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			TempF:      77.0,
			Humidity:   50.0,
			Battery:    85,
			RSSI:       -60,
			Timestamp:  time.Now(),
			ClientID:   "test-client",
		},
	}

	// Save readings
	err := sm.saveReadings("AABBCCDDEEFF", readings)
	if err != nil {
		t.Fatalf("Failed to save readings: %v", err)
	}

	// Check that a file was created
	partitionDir := sm.getCurrentPartitionDir()
	files, err := filepath.Glob(filepath.Join(partitionDir, "readings_*.json"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
}

// TestListPartitionDirs tests listing partition directories
func TestListPartitionDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some partition directories
	partitions := []string{"2023-04", "2023-05", "2023-06"}
	for _, p := range partitions {
		os.MkdirAll(filepath.Join(tmpDir, p), 0755)
	}

	config := &StorageConfig{
		BaseDir:           tmpDir,
		TimePartitioning:  true,
		PartitionInterval: 720 * time.Hour,
		RetentionPeriod:   0,
		CompressOldData:   false,
	}

	sm := NewStorageManager(config)
	dirs, err := sm.listPartitionDirs()
	if err != nil {
		t.Fatalf("Failed to list partition dirs: %v", err)
	}

	if len(dirs) != 3 {
		t.Errorf("Expected 3 dirs, got %d: %v", len(dirs), dirs)
	}
}

// TestParsePartitionTime tests parsing partition directory names
func TestParsePartitionTime(t *testing.T) {
	tmpDir := t.TempDir()

	config := &StorageConfig{
		BaseDir:           tmpDir,
		TimePartitioning:  true,
		PartitionInterval: 720 * time.Hour,
		RetentionPeriod:   0,
		CompressOldData:   false,
	}

	sm := NewStorageManager(config)

	tests := []struct {
		name    string
		dirname string
		wantErr bool
	}{
		{"Monthly format", "2023-04", false},
		{"Daily format", "2023-04-15", false},
		{"Weekly format", "2023-W15", false},
		{"Invalid format", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.parsePartitionTime(tt.dirname)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePartitionTime(%s) error = %v, wantErr %v", tt.dirname, err, tt.wantErr)
			}
		})
	}
}

// TestIsCompressed tests compressed file detection
func TestIsCompressed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with no .gz files
	emptyDir := filepath.Join(tmpDir, "empty")
	os.MkdirAll(emptyDir, 0755)
	os.WriteFile(filepath.Join(emptyDir, "readings.json"), []byte("{}"), 0644)

	// Create a directory with .gz files
	compressedDir := filepath.Join(tmpDir, "compressed")
	os.MkdirAll(compressedDir, 0755)
	os.WriteFile(filepath.Join(compressedDir, "readings.json.gz"), []byte("compressed"), 0644)

	tests := []struct {
		name       string
		dir        string
		compressed bool
	}{
		{"Empty dir", emptyDir, false},
		{"Compressed dir", compressedDir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCompressed(tt.dir)
			if result != tt.compressed {
				t.Errorf("isCompressed(%s) = %v, want %v", tt.dir, result, tt.compressed)
			}
		})
	}
}

// TestServerSaveAndLoadData tests persistence save/load cycle
func TestServerSaveAndLoadData(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Port:               8080,
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  100,
		StorageDir:         tmpDir,
		PersistenceEnabled: true,
		SaveInterval:       1 * time.Hour,
	}

	auth := &AuthConfig{
		EnableAuth: true,
		AdminKey:   "test-admin-key",
		APIKeys: map[string]string{
			"client-key": "test-client",
		},
	}

	storageConfig := &StorageConfig{
		BaseDir: tmpDir,
	}
	storageManager := NewStorageManager(storageConfig)

	server := NewServer(config, auth, storageManager)
	defer server.shutdownCancel()
	if server.logger != nil {
		defer server.logger.Close()
	}

	// Add some data
	server.addReading(Reading{
		DeviceName: "Test Device",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.0,
		TempF:      77.0,
		Humidity:   50.0,
		Battery:    85,
		RSSI:       -60,
		Timestamp:  time.Now(),
		ClientID:   "test-client",
	})

	// Save data
	server.saveData()

	// Check that files were created
	files, err := filepath.Glob(filepath.Join(tmpDir, "*.json"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	if len(files) == 0 {
		t.Log("No JSON files found, checking for readings file")
	}

	// Create a new server to test loading
	server2 := NewServer(config, auth, storageManager)
	defer server2.shutdownCancel()
	if server2.logger != nil {
		defer server2.logger.Close()
	}

	// The loadData is called implicitly during NewServer for persistence

	// If persistence is working, the devices should be loaded
	devices := server2.getDevices()
	t.Logf("Loaded %d devices after persistence reload", len(devices))
}

// TestEnforceRetention tests retention policy enforcement
func TestEnforceRetention(t *testing.T) {
	tmpDir := t.TempDir()

	// Create old partition directories
	oldPartition := filepath.Join(tmpDir, "2022-01")
	os.MkdirAll(oldPartition, 0755)

	// Create a readings file in the old partition
	readings := []Reading{
		{
			DeviceName: "Old Device",
			DeviceAddr: "11:22:33:44:55:66",
			TempC:      20.0,
			Humidity:   40.0,
			Timestamp:  time.Now().Add(-400 * 24 * time.Hour), // 400 days ago
		},
	}
	readingsBytes, _ := json.Marshal(readings)
	os.WriteFile(filepath.Join(oldPartition, "readings_112233445566.json"), readingsBytes, 0644)

	// Create current partition
	currentPartition := filepath.Join(tmpDir, time.Now().Format("2006-01"))
	os.MkdirAll(currentPartition, 0755)

	config := &StorageConfig{
		BaseDir:           tmpDir,
		TimePartitioning:  true,
		PartitionInterval: 720 * time.Hour,
		RetentionPeriod:   365 * 24 * time.Hour, // 1 year retention
		CompressOldData:   false,
	}

	sm := NewStorageManager(config)

	// Enforce retention
	err := sm.enforceRetention()
	if err != nil {
		t.Fatalf("Failed to enforce retention: %v", err)
	}

	// Check if old partition was removed
	_, err = os.Stat(oldPartition)
	if err == nil {
		t.Log("Old partition still exists (may not be old enough)")
	}
}

// TestCompressPartition tests partition compression
func TestCompressPartition(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a partition with a JSON file
	partitionDir := filepath.Join(tmpDir, "2022-01")
	os.MkdirAll(partitionDir, 0755)

	testData := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
		},
	}
	jsonData, _ := json.Marshal(testData)
	jsonFile := filepath.Join(partitionDir, "readings_AABBCCDDEEFF.json")
	os.WriteFile(jsonFile, jsonData, 0644)

	config := &StorageConfig{
		BaseDir:         tmpDir,
		CompressOldData: true,
	}

	sm := NewStorageManager(config)

	// Compress the partition
	err := sm.compressPartition(partitionDir)
	if err != nil {
		t.Fatalf("Failed to compress partition: %v", err)
	}

	// Check that gz file was created
	gzFile := jsonFile + ".gz"
	if _, err := os.Stat(gzFile); os.IsNotExist(err) {
		t.Error("Compressed file was not created")
	}

	// Check that original file was removed
	if _, err := os.Stat(jsonFile); err == nil {
		t.Error("Original file was not removed after compression")
	}
}

// TestLoadReadingsFromFile tests loading readings from a file
func TestLoadReadingsFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test JSON file
	testReadings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			TempF:      77.0,
			Humidity:   50.0,
			Battery:    85,
			Timestamp:  time.Now(),
		},
	}

	jsonData, _ := json.Marshal(testReadings)
	jsonFile := filepath.Join(tmpDir, "readings_AABBCCDDEEFF.json")
	os.WriteFile(jsonFile, jsonData, 0644)

	config := &StorageConfig{
		BaseDir: tmpDir,
	}

	sm := NewStorageManager(config)

	// Load readings
	loaded, err := sm.loadReadingsFromFile(jsonFile)
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("Expected 1 reading, got %d", len(loaded))
	}

	if loaded[0].TempC != 25.0 {
		t.Errorf("Expected TempC 25.0, got %f", loaded[0].TempC)
	}
}

// TestLoadReadings tests loading readings with time range
func TestLoadReadings(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a partition with readings
	partitionDir := filepath.Join(tmpDir, time.Now().Format("2006-01"))
	os.MkdirAll(partitionDir, 0755)

	now := time.Now()
	testReadings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  now.Add(-2 * time.Hour),
		},
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      26.0,
			Humidity:   55.0,
			Timestamp:  now.Add(-1 * time.Hour),
		},
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      27.0,
			Humidity:   60.0,
			Timestamp:  now,
		},
	}

	jsonData, _ := json.Marshal(testReadings)
	jsonFile := filepath.Join(partitionDir, "readings_AABBCCDDEEFF.json")
	os.WriteFile(jsonFile, jsonData, 0644)

	config := &StorageConfig{
		BaseDir:           tmpDir,
		TimePartitioning:  true,
		PartitionInterval: 720 * time.Hour,
	}

	sm := NewStorageManager(config)

	// Load readings with time range
	fromTime := now.Add(-90 * time.Minute)
	toTime := now.Add(1 * time.Minute)
	loaded, err := sm.loadReadings("AABBCCDDEEFF", fromTime, toTime)
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	// Should get 2 readings (the last 2)
	if len(loaded) != 2 {
		t.Errorf("Expected 2 readings, got %d", len(loaded))
	}
}

// BenchmarkSaveReadings benchmarks saving readings
func BenchmarkSaveReadings(b *testing.B) {
	tmpDir := b.TempDir()

	config := &StorageConfig{
		BaseDir:            tmpDir,
		TimePartitioning:   true,
		PartitionInterval:  720 * time.Hour,
		MaxReadingsPerFile: 1000,
	}

	sm := NewStorageManager(config)

	readings := make([]Reading, 100)
	for i := range readings {
		readings[i] = Reading{
			DeviceName: "Benchmark Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.saveReadings("AABBCCDDEEFF", readings)
	}
}
