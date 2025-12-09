# Data Storage and Retention Guide

**Version 2.0** - SQLite and Advanced Storage Features

This guide explains how the Govee Monitoring System handles data storage, including the new SQLite backend, storage abstraction layer, time-based partitioning, and retention policies for historical data.

## Overview

The system implements advanced data storage features with v2.0:

1. **Multiple Storage Backends**: SQLite (recommended) or JSON-based storage
2. **Storage Abstraction Layer**: Easy migration path to InfluxDB or TimescaleDB
3. **High Performance**: 10-100x faster queries with SQLite
4. **Time-Based Partitioning**: Data organized into directories based on time periods (JSON mode)
5. **Configurable Retention Policies**: Automatically manage how long data is kept
6. **Automatic Compression**: Older data can be compressed to save storage space (JSON mode)
7. **Time Range Queries**: Retrieve historical data from specific time periods
8. **Migration Tools**: Convert existing JSON data to SQLite

## Storage Backends (New in v2.0)

### SQLite Storage (Recommended)

SQLite provides the best performance for most use cases:

**Benefits:**
- 10-100x faster queries compared to JSON
- Efficient indexing on device_addr, timestamp, client_id
- Built-in transaction support
- Better memory efficiency
- Support for complex filtering and aggregation
- Write-Ahead Logging (WAL) for better concurrency

**Usage:**
```bash
# SQLite is the default storage backend
./govee-server -storage-type=sqlite -db-path=./data/readings.db
```

**Database Schema:**
```sql
CREATE TABLE readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_addr TEXT NOT NULL,
    device_name TEXT,
    timestamp DATETIME NOT NULL,
    temperature REAL,
    humidity REAL,
    battery INTEGER,
    rssi INTEGER,
    client_id TEXT,
    dew_point REAL,
    absolute_humidity REAL,
    steam_pressure REAL
);

-- Optimized indexes
CREATE INDEX idx_device_addr ON readings(device_addr);
CREATE INDEX idx_timestamp ON readings(timestamp);
CREATE INDEX idx_device_timestamp ON readings(device_addr, timestamp);
CREATE INDEX idx_client_id ON readings(client_id);
```

### JSON Storage (Legacy)

JSON-based storage is still supported for backwards compatibility:

**Usage:**
```bash
./govee-server -storage-type=json -storage=./data
```

JSON storage supports time-based partitioning and compression (see sections below).

### Future Migration Path

The storage abstraction layer makes it easy to migrate to time-series databases:

**Planned Support:**
- **InfluxDB**: Purpose-built for time-series data
- **TimescaleDB**: PostgreSQL extension for time-series
- **Prometheus**: For metrics and monitoring

The `StorageBackend` interface is designed to support these backends with minimal code changes.

## Storage Configuration

When starting the server, you can configure data storage with these flags:

### General Storage Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-storage-type` | sqlite | Storage backend: "sqlite" or "json" |
| `-storage` | ./data | Base storage directory |
| `-retention` | 0 (unlimited) | How long to keep data (e.g., 8760h for 1 year) |

### SQLite-Specific Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-db-path` | ./data/readings.db | Path to SQLite database file |

### JSON-Specific Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-time-partition` | true | Enable time-based partitioning |
| `-partition-interval` | 720h (30 days) | Interval for new data partitions |
| `-max-file-readings` | 1000 | Maximum readings per storage file |
| `-compress` | true | Compress older partitions to save space |

## Time-Based Partitioning

When time partitioning is enabled, data is organized into subdirectories based on time periods:

- **Daily Partitioning**: For intervals <= 24 hours, creates directories like `2023-04-10/`
- **Weekly Partitioning**: For intervals <= 7 days, creates directories like `2023-W15/`
- **Monthly Partitioning**: For longer intervals, creates directories like `2023-04/`

Inside each partition directory, data files are organized by device:
```
data/
├── 2023-04/
│   ├── readings_A4C13825A1E3.json
│   └── readings_A4C13826B2F4.json
├── 2023-05/
│   ├── readings_A4C13825A1E3.json.gz
│   └── readings_A4C13826B2F4.json.gz
└── 2023-06/
    ├── readings_A4C13825A1E3.json
    └── readings_A4C13826B2F4.json
```

## Data Retention Policy

The system can automatically manage how long data is kept:

- Set `-retention=0` for unlimited storage (default)
- Set `-retention=8760h` to keep data for one year
- Set `-retention=2160h` to keep data for 90 days

The system automatically removes partitions older than the retention period. This check runs once per day.

## Data Compression

To save storage space, older data partitions can be automatically compressed:

- Current partition is always kept uncompressed for fast access
- Older partitions are compressed with gzip (.gz extension)
- Compressed data is automatically decompressed when accessed

Enable or disable this feature with the `-compress` flag.

## Accessing Historical Data

The API now supports time range queries to access historical data:

```
GET /readings?device=<addr>&from=2023-04-01T00:00:00Z&to=2023-04-30T23:59:59Z
```

Parameters:
- `device`: Device address (required)
- `from`: Start time in RFC3339 format (optional)
- `to`: End time in RFC3339 format (optional)

The system will automatically locate and retrieve data across all relevant time partitions.

## Storage Management Examples

### Example 1: Default Settings (30-day partitions, unlimited retention)

```bash
./govee-server
```

- Data will be organized in monthly partitions
- All historical data will be kept indefinitely
- Older partitions will be compressed

### Example 2: Daily Partitions with 90-Day Retention

```bash
./govee-server -partition-interval=24h -retention=2160h
```

- Data will be organized in daily partitions
- Data older than 90 days will be automatically removed
- Older partitions will be compressed

### Example 3: Weekly Partitions without Compression

```bash
./govee-server -partition-interval=168h -compress=false
```

- Data will be organized in weekly partitions
- All historical data will be kept indefinitely
- No compression will be applied

## Performance Considerations

- **Time Partitioning**: Improves query performance for time-based queries
- **Compression**: Reduces storage requirements but may slightly increase CPU usage
- **Partition Interval**: Smaller intervals create more files but can improve query speed for specific time ranges
- **Max Readings Per File**: Controls memory usage when loading data

## Backup Recommendations

Even with retention policies, it's recommended to periodically back up your data:

1. **Database Exports**: Use the `/readings` endpoint with time ranges to export data
2. **Directory Backups**: Back up the entire data directory including all partitions
3. **InfluxDB Integration**: For critical long-term storage, consider the InfluxDB integration

## Migrating from JSON to SQLite (New in v2.0)

If you're upgrading from v1.x or using JSON storage, you can migrate to SQLite for better performance:

### Step 1: Create Migration Script

Create a file named `migrate.go`:

```go
package main

import (
    "log"
    "path/filepath"

    // Import from your server package
    // Adjust the import path to match your setup
)

func main() {
    jsonDir := "./data"
    sqlitePath := "./data/readings.db"

    log.Println("Starting migration from JSON to SQLite...")
    err := MigrateJSONToSQLite(jsonDir, sqlitePath)
    if err != nil {
        log.Fatalf("Migration failed: %v", err)
    }

    log.Println("Verifying migration...")
    err = VerifyMigration(jsonDir, sqlitePath)
    if err != nil {
        log.Fatalf("Verification failed: %v", err)
    }

    log.Println("Migration completed successfully!")
    log.Println("You can now start the server with: ./govee-server -storage-type=sqlite")
}
```

### Step 2: Run Migration

```bash
cd server
go run migrate.go
```

The migration tool will:
1. Read all JSON files from the data directory
2. Insert readings into SQLite in batches of 1000
3. Verify that all data was migrated correctly
4. Report any errors or mismatches

### Step 3: Switch to SQLite

After successful migration, update your server startup:

```bash
# Old (JSON storage)
./govee-server -storage=./data

# New (SQLite storage)
./govee-server -storage-type=sqlite -db-path=./data/readings.db
```

### Step 4: Backup JSON Files (Optional)

Once you've verified SQLite is working correctly, you can archive the old JSON files:

```bash
mkdir -p ./data/json-backup
mv ./data/*.json ./data/json-backup/
# Or if using time partitions:
mv ./data/2023-* ./data/json-backup/
```

**Note:** Keep the JSON backup until you're confident the migration was successful.

## Migrating Existing Data (Time Partitions)

When upgrading JSON-based storage to use time partitioning, existing data will be automatically migrated:

1. The system checks for data in the old format
2. If found, it's converted to the new partitioned format
3. The original data files are preserved until migration is confirmed successful

To manually migrate data:
```bash
./govee-server -migrate-data=true
```
