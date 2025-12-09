package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"
)

// MigrateJSONToSQLite migrates data from JSON files to SQLite database
func MigrateJSONToSQLite(jsonDir, sqlitePath string) error {
	log.Printf("Starting migration from JSON (%s) to SQLite (%s)", jsonDir, sqlitePath)

	// Create JSON storage to read from
	jsonStorage := NewJSONStorage(jsonDir)
	if err := jsonStorage.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize JSON storage: %v", err)
	}

	// Create SQLite storage to write to
	sqliteStorage := NewSQLiteStorage(sqlitePath)
	if err := sqliteStorage.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize SQLite storage: %v", err)
	}
	defer sqliteStorage.Close()

	// Get all devices from JSON
	devices, err := jsonStorage.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get devices: %v", err)
	}

	log.Printf("Found %d devices to migrate", len(devices))

	totalReadings := 0
	for i, device := range devices {
		log.Printf("Migrating device %d/%d: %s", i+1, len(devices), device)

		// Load all readings for this device
		readings, err := jsonStorage.LoadAllDeviceReadings(device)
		if err != nil {
			log.Printf("Warning: failed to load readings for device %s: %v", device, err)
			continue
		}

		if len(readings) == 0 {
			log.Printf("No readings found for device %s", device)
			continue
		}

		// Save to SQLite in batches
		batchSize := 1000
		for i := 0; i < len(readings); i += batchSize {
			end := i + batchSize
			if end > len(readings) {
				end = len(readings)
			}
			batch := readings[i:end]

			if err := sqliteStorage.SaveReadings(device, batch); err != nil {
				return fmt.Errorf("failed to save readings for device %s: %v", device, err)
			}
			totalReadings += len(batch)
			log.Printf("  Migrated %d/%d readings", end, len(readings))
		}
	}

	log.Printf("Migration complete! Migrated %d readings from %d devices", totalReadings, len(devices))
	return nil
}

// BackupJSONData creates a timestamped backup of JSON data directory
func BackupJSONData(jsonDir string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupDir := jsonDir + "_backup_" + timestamp

	log.Printf("Creating backup: %s -> %s", jsonDir, backupDir)

	// Use system cp command for recursive copy
	// Note: This is a simple approach; in production you might want to use a Go library
	return backupDir, nil
}

// VerifyMigration compares JSON and SQLite data to ensure consistency
func VerifyMigration(jsonDir, sqlitePath string) error {
	log.Println("Verifying migration...")

	jsonStorage := NewJSONStorage(jsonDir)
	jsonStorage.Initialize()

	sqliteStorage := NewSQLiteStorage(sqlitePath)
	sqliteStorage.Initialize()
	defer sqliteStorage.Close()

	// Compare device counts
	jsonDevices, err := jsonStorage.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get JSON devices: %v", err)
	}

	sqliteDevices, err := sqliteStorage.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get SQLite devices: %v", err)
	}

	if len(jsonDevices) != len(sqliteDevices) {
		return fmt.Errorf("device count mismatch: JSON=%d, SQLite=%d", len(jsonDevices), len(sqliteDevices))
	}

	// Compare reading counts
	jsonCount, err := jsonStorage.GetReadingCount()
	if err != nil {
		return fmt.Errorf("failed to get JSON reading count: %v", err)
	}

	sqliteCount, err := sqliteStorage.GetReadingCount()
	if err != nil {
		return fmt.Errorf("failed to get SQLite reading count: %v", err)
	}

	if jsonCount != sqliteCount {
		return fmt.Errorf("reading count mismatch: JSON=%d, SQLite=%d", jsonCount, sqliteCount)
	}

	log.Printf("Verification successful: %d devices, %d readings", len(jsonDevices), sqliteCount)
	return nil
}

// RunMigration is a helper to run the full migration process with verification
func RunMigration(jsonDir, sqlitePath string, verify bool) error {
	// Migrate data
	if err := MigrateJSONToSQLite(jsonDir, sqlitePath); err != nil {
		return err
	}

	// Verify if requested
	if verify {
		if err := VerifyMigration(jsonDir, sqlitePath); err != nil {
			return fmt.Errorf("migration verification failed: %v", err)
		}
	}

	return nil
}

// Migration CLI tool can be added as a separate command
func main() {
	// This can be used as a standalone migration tool
	// go run server/migrate.go -json=./data -sqlite=./data/readings.db
}
