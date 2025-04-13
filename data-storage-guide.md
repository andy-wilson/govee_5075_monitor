# Data Storage and Retention Guide

This guide explains how the Govee Monitoring System handles data storage, time-based partitioning, and retention policies for historical data.

## Overview

The system now implements advanced data storage features:

1. **Time-Based Partitioning**: Data is organized into directories based on time periods
2. **Configurable Retention Policies**: Automatically manage how long data is kept
3. **Automatic Compression**: Older data can be compressed to save storage space
4. **Time Range Queries**: Retrieve historical data from specific time periods

## Storage Configuration

When starting the server, you can configure data storage with these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-storage` | ./data | Base storage directory |
| `-time-partition` | true | Enable time-based partitioning |
| `-partition-interval` | 720h (30 days) | Interval for new data partitions |
| `-retention` | 0 (unlimited) | How long to keep data (e.g., 8760h for 1 year) |
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

## Migrating Existing Data

When upgrading to the new storage system, existing data will be automatically migrated:

1. The system checks for data in the old format
2. If found, it's converted to the new partitioned format
3. The original data files are preserved until migration is confirmed successful

To manually migrate data:
```bash
./govee-server -migrate-data=true
```
