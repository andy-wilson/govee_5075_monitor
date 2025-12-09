# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a client-server system for monitoring Govee H5075 temperature and humidity sensors via Bluetooth Low Energy (BLE). The system includes:
- **Client** (Go): BLE scanner that discovers H5075 devices and sends readings to server
- **Server** (Go): Data aggregation, storage, API endpoints, and web dashboard
- **Dashboard** (JavaScript): Web UI for visualization

## Build and Run Commands

### Building

```bash
# Build server
cd server && go build -o govee-server govee-server.go

# Build client
cd client && go build -o govee-client govee-client.go
```

### Running Locally

```bash
# Run server (default port 8080)
cd server && ./govee-server -port=8080 -log=govee-server.log

# Run client in discovery mode (no server needed)
cd client && ./govee-client -discover -duration=1m

# Run client in standalone mode (no server needed)
cd client && ./govee-client -local=true

# Run client connected to server
cd client && ./govee-client -server=http://localhost:8080/readings -apikey=YOUR_KEY -continuous=true
```

### Docker

```bash
# Start server
docker-compose -f server/docker-compose.yaml up -d

# Start client
docker-compose -f client/docker-compose.yaml up -d
```

### Testing

There are no automated tests in the repository yet. Testing is manual via running the client/server.

## Architecture

### Data Flow
1. **Client** uses BLE to scan for Govee H5075 devices broadcasting advertisement data
2. Client decodes manufacturer-specific data containing temp/humidity/battery/RSSI
3. Client calculates derived metrics (absolute humidity, dew point, steam pressure)
4. Client POSTs readings to server's `/readings` endpoint with API key authentication
5. **Server** validates API key, stores readings in memory and on disk
6. Server provides REST API endpoints for querying data
7. **Dashboard** (static HTML/JS) fetches data via `/dashboard/data` endpoint and displays charts

### Project Structure
```
.
├── client/
│   ├── govee-client.go      # BLE scanner + HTTP client
│   ├── Dockerfile
│   └── docker-compose.yaml
├── server/
│   ├── govee-server.go      # HTTP server + storage manager
│   ├── Dockerfile
│   └── docker-compose.yaml
├── static/
│   ├── index.html           # Dashboard UI
│   └── dashboard.js         # Chart rendering logic
├── data/                    # Storage directory (JSON files)
├── docs/                    # Guides for auth, storage, metrics
├── openapi/
│   └── openapi.yaml         # API specification
└── go.mod                   # Shared Go dependencies
```

### Key Components

**client/govee-client.go**
- Uses `github.com/go-ble/ble` library for BLE scanning
- Decodes Govee H5075 manufacturer data (3-byte temp, 3-byte humidity, 1-byte battery)
- Supports three modes: discovery (scan only), standalone (local logging), connected (send to server)
- Calculates derived metrics: absolute humidity, dew point (both C/F), steam pressure
- Supports temperature/humidity offset calibration

**server/govee-server.go**
- Single-file HTTP server (no external web framework)
- In-memory storage with periodic disk persistence
- Time-based data partitioning (daily/weekly/monthly directories)
- Automatic data retention and compression
- API key authentication with admin/client-specific/default keys
- Serves static dashboard files

**Data Storage**
- **Partitioning**: Data organized into time-based directories (`data/2023-04/`)
- **Retention**: Configurable automatic deletion of old data
- **Compression**: Older partitions compressed to `.gz` format
- **Format**: JSON files per device (`readings_A4C13825A1E3.json`)

### Authentication System

The server uses API key-based authentication:
- **Admin API Key**: Full access including key management via `/api/keys` endpoints
- **Client-Specific Keys**: Tied to specific client IDs
- **Default API Key**: Optional shared key (less secure)

API keys are stored in `data/auth.json` and validated via `X-API-Key` header.

### API Endpoints

- `POST /readings` - Submit new reading (requires API key)
- `GET /readings?device=<addr>` - Get readings for device with optional `from`/`to` time range
- `GET /devices` - List all devices with latest status
- `GET /clients` - List all connected clients
- `GET /stats?device=<addr>` - Get statistics for device
- `GET /dashboard/data` - Get all data for dashboard
- `GET /api/keys` - List API keys (admin only)
- `POST /api/keys` - Create API key (admin only)
- `DELETE /api/keys?key=<key>` - Delete API key (admin only)
- `GET /health` - Health check (no auth)

Full API specification: `openapi/openapi.yaml`

## Development Notes

### Data Types

**Reading**: Single measurement from a device
- Temperature (C/F), humidity (relative/absolute), battery %, RSSI
- Derived metrics: dew point, steam pressure
- Timestamp and client ID

**DeviceStatus**: Latest known state of a device
- Last reading values + last seen timestamp + reading count

**ClientStatus**: Connection state of a client
- Last seen, device count, reading count, active status

### Storage Manager

The `StorageManager` in `govee-server.go` handles:
- **Partitioning logic**: Determines directory based on time and partition interval
- **Persistence**: Periodic saving to JSON files
- **Retention**: Automatic cleanup of old partitions
- **Compression**: Gzipping old JSON files
- **Loading**: Reads partitioned data on startup

### BLE Scanning

The client uses `go-ble/ble` to scan for BLE advertisements. Govee H5075 devices broadcast manufacturer-specific data:
- Manufacturer ID: `0x0001` (Govee)
- Data format: 6 bytes (3 temp + 3 humidity + 1 battery)
- Decoding: Temperature and humidity are 3-byte little-endian values / 1000

### Calibration

Both client and server support offset calibration:
- `-temp-offset` and `-humidity-offset` flags on client
- Offsets applied before sending to server
- Used to correct sensor drift or environmental factors

## Common Gotchas

1. **BLE Permissions**: Client needs raw network capabilities on Linux (`setcap cap_net_raw,cap_net_admin=eip`)
2. **API Keys**: Generated randomly if not specified; check server logs for auto-generated keys
3. **Data Directory**: Server creates `data/` directory automatically but ensure write permissions
4. **HTTPS/TLS**: Server supports TLS with `-cert` and `-key` flags; client supports `-insecure` and `-ca-cert`
5. **Client ID**: Auto-generated from hostname if not specified via `-id` flag
6. **Partition Intervals**: Default 720h (30 days); adjust with `-partition-interval` based on data volume

## Configuration Flags

See README.md for complete flag reference. Key flags:

**Server:**
- `-port` (default 8080)
- `-storage` (default ./data)
- `-auth` (default true)
- `-time-partition` (default true)
- `-partition-interval` (default 720h)
- `-retention` (default 0 = unlimited)
- `-compress` (default true)

**Client:**
- `-server` (default http://localhost:8080/readings)
- `-apikey` (required for connected mode)
- `-continuous` (default false)
- `-duration` (default 30s per scan)
- `-discover` (discovery mode)
- `-local` (standalone mode)
