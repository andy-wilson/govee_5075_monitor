package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMigrateJSONToSQLite tests the JSON to SQLite migration
func TestMigrateJSONToSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	// Create JSON storage with test data
	jsonStorage := NewJSONStorage(jsonDir)
	if err := jsonStorage.Initialize(); err != nil {
		t.Fatalf("Failed to initialize JSON storage: %v", err)
	}

	// Add test readings
	testReadings := []Reading{
		{
			DeviceName: "Test Device 1",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			TempF:      77.0,
			Humidity:   50.0,
			Battery:    85,
			RSSI:       -60,
			Timestamp:  time.Now().Add(-2 * time.Hour),
			ClientID:   "test-client",
		},
		{
			DeviceName: "Test Device 1",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      26.0,
			TempF:      78.8,
			Humidity:   52.0,
			Battery:    84,
			RSSI:       -62,
			Timestamp:  time.Now().Add(-1 * time.Hour),
			ClientID:   "test-client",
		},
	}

	if err := jsonStorage.SaveReadings("AABBCCDDEEFF", testReadings); err != nil {
		t.Fatalf("Failed to save test readings: %v", err)
	}

	// Run migration
	err := MigrateJSONToSQLite(jsonDir, sqlitePath)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify SQLite has the data
	sqliteStorage := NewSQLiteStorage(sqlitePath)
	if err := sqliteStorage.Initialize(); err != nil {
		t.Fatalf("Failed to initialize SQLite for verification: %v", err)
	}
	defer sqliteStorage.Close()

	count, err := sqliteStorage.GetReadingCount()
	if err != nil {
		t.Fatalf("Failed to get reading count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 readings in SQLite, got %d", count)
	}
}

// TestMigrateJSONToSQLiteEmptyDir tests migration with empty directory
func TestMigrateJSONToSQLiteEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	os.MkdirAll(jsonDir, 0755)

	// Run migration - should succeed with 0 devices
	err := MigrateJSONToSQLite(jsonDir, sqlitePath)
	if err != nil {
		t.Fatalf("Migration with empty dir failed: %v", err)
	}
}

// TestMigrateJSONToSQLiteMultipleDevices tests migration with multiple devices
func TestMigrateJSONToSQLiteMultipleDevices(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	// Create JSON storage with test data
	jsonStorage := NewJSONStorage(jsonDir)
	if err := jsonStorage.Initialize(); err != nil {
		t.Fatalf("Failed to initialize JSON storage: %v", err)
	}

	// Add readings for device 1
	readings1 := []Reading{
		{
			DeviceName: "Device 1",
			DeviceAddr: "AA:BB:CC:DD:EE:01",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
			ClientID:   "client1",
		},
	}
	jsonStorage.SaveReadings("AABBCCDDEE01", readings1)

	// Add readings for device 2
	readings2 := []Reading{
		{
			DeviceName: "Device 2",
			DeviceAddr: "AA:BB:CC:DD:EE:02",
			TempC:      26.0,
			Humidity:   55.0,
			Timestamp:  time.Now(),
			ClientID:   "client1",
		},
		{
			DeviceName: "Device 2",
			DeviceAddr: "AA:BB:CC:DD:EE:02",
			TempC:      27.0,
			Humidity:   60.0,
			Timestamp:  time.Now(),
			ClientID:   "client1",
		},
	}
	jsonStorage.SaveReadings("AABBCCDDEE02", readings2)

	// Run migration
	err := MigrateJSONToSQLite(jsonDir, sqlitePath)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify
	sqliteStorage := NewSQLiteStorage(sqlitePath)
	sqliteStorage.Initialize()
	defer sqliteStorage.Close()

	devices, _ := sqliteStorage.GetDevices()
	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}

	count, _ := sqliteStorage.GetReadingCount()
	if count != 3 {
		t.Errorf("Expected 3 total readings, got %d", count)
	}
}

// TestBackupJSONData tests backup creation
func TestBackupJSONData(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	os.MkdirAll(jsonDir, 0755)

	backupPath, err := BackupJSONData(jsonDir)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Check that backup path contains timestamp format
	if backupPath == "" {
		t.Error("Backup path should not be empty")
	}
}

// TestVerifyMigration tests migration verification
func TestVerifyMigration(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	// Create identical data in both stores
	jsonStorage := NewJSONStorage(jsonDir)
	jsonStorage.Initialize()

	sqliteStorage := NewSQLiteStorage(sqlitePath)
	sqliteStorage.Initialize()
	defer sqliteStorage.Close()

	readings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
			ClientID:   "test",
		},
	}

	jsonStorage.SaveReadings("AABBCCDDEEFF", readings)
	sqliteStorage.SaveReadings("AABBCCDDEEFF", readings)

	// Verify should succeed
	err := VerifyMigration(jsonDir, sqlitePath)
	if err != nil {
		t.Errorf("Verification should succeed: %v", err)
	}
}

// TestVerifyMigrationMismatch tests verification with mismatched data
func TestVerifyMigrationMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	// Create mismatched data
	jsonStorage := NewJSONStorage(jsonDir)
	jsonStorage.Initialize()

	sqliteStorage := NewSQLiteStorage(sqlitePath)
	sqliteStorage.Initialize()
	defer sqliteStorage.Close()

	// Add data only to JSON
	readings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
			ClientID:   "test",
		},
	}
	jsonStorage.SaveReadings("AABBCCDDEEFF", readings)

	// Verify should fail
	err := VerifyMigration(jsonDir, sqlitePath)
	if err == nil {
		t.Error("Verification should fail with mismatched data")
	}
}

// TestRunMigration tests the full migration runner
func TestRunMigration(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	// Create JSON storage with test data
	jsonStorage := NewJSONStorage(jsonDir)
	jsonStorage.Initialize()

	readings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
			ClientID:   "test",
		},
	}
	jsonStorage.SaveReadings("AABBCCDDEEFF", readings)

	// Run with verification
	err := RunMigration(jsonDir, sqlitePath, true)
	if err != nil {
		t.Errorf("RunMigration with verify should succeed: %v", err)
	}
}

// TestRunMigrationWithoutVerify tests migration without verification
func TestRunMigrationWithoutVerify(t *testing.T) {
	tmpDir := t.TempDir()
	jsonDir := filepath.Join(tmpDir, "json")
	sqlitePath := filepath.Join(tmpDir, "test.db")

	// Create JSON storage with test data
	jsonStorage := NewJSONStorage(jsonDir)
	jsonStorage.Initialize()

	readings := []Reading{
		{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
			Humidity:   50.0,
			Timestamp:  time.Now(),
			ClientID:   "test",
		},
	}
	jsonStorage.SaveReadings("AABBCCDDEEFF", readings)

	// Run without verification
	err := RunMigration(jsonDir, sqlitePath, false)
	if err != nil {
		t.Errorf("RunMigration without verify should succeed: %v", err)
	}
}
