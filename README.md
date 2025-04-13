# Govee 5075 Monitoring System

A client-server system for monitoring Govee H5075 temperature and humidity sensors via Bluetooth Low Energy (BLE). This project includes a client for collecting sensor data, a server for aggregating and storing measurements, and a simplified React-based dashboard for basic visualization.

The feature set may seem a little random, and a bit extensive/specific in some specific areas, but I've been developing this while trying to solve some specific problems with sensors at home, and well.. things got a bit out of hand.
Needless to say, this might be utterly broken for you, that old addage "works on my machine" seems appropriate. This comes with absolutely no warranty whatsover (as per the [license](LICENSE.md)license).

I'm not intending to do much with this other than nerd out and use it at home, but if someone finds any of this useful for home use then great. I have a few [TODOs](TODO.md) still, but if anyone finds this and wants/needs a feature, feel free to be harsh with the code critique, and also to submit PRs. 

I might add support for some other models and modes of operation in time. 


## System Architecture

The system consists of the following components:

1. **Client**: Scans for Govee H5075 devices using BLE, decodes the sensor data, and sends it to the server.
2. **Server**: Collects data from multiple clients, stores historical readings, and provides an API for accessing the data.
3. **Dashboard**: Web-based user interface for visualizing temperature, humidity, and device status.
4. **Optional**: InfluxDB and Grafana for advanced data storage and visualization.

## Features

- **Real-time monitoring** of temperature, humidity, battery levels, and signal strength
- **Multi-client support** to collect data from sensors in different locations
- **Centralized logging** of all sensor readings
- **Persistent storage** with automatic data saving
- **Interactive dashboard** with charts and device information
- **API endpoints** for integration with other systems
- **API key authentication** ensures only authorized clients can send data
- **Docker support** for easy deployment
- **Standalone mode** for running clients without a server
- **Discovery mode** to scan and list available devices
- **TLS support** for the server, and client connections

## Requirements

### Client Requirements

- Linux, macOS, or Windows with Bluetooth 4.0+ support. 
- Go 1.18 or later
- Govee H5075 Temperature & Humidity Sensor devices (you can buy these cheaply on Amazon)

### Server Requirements

- Any platform that supports Go (Linux/macOS/Windows)
- Go 1.18 or later
- Suffiecient disk space for data storage (50+ MB)

## Installation

### Option 1: Docker Compose (Recommended)

1. Clone the repository:
   ```bash
   git clone https://github.com/andy-wilson/govee_5075_monitor.git
   cd govee_5075_monitor
   ```

2. Start the server using Docker Compose:
   ```bash
   docker-compose up -d govee-server
   ```

3. Run the client(s) on machines with Bluetooth access:
   ```bash
   docker-compose -f client-compose.yml up -d
   ```

### Option 2: Manual Installation

#### Server Setup

1. Install Go 1.18 or later
2. Clone the repository
3. Navigate to the server directory
4. Build the server:
   ```bash
   go build -o govee-server govee-server.go
   ```
5. Run the server:
   ```bash
   ./govee-server -port=8080 -log=govee-server.log
   ```

#### Client Setup

1. Install Go 1.18 or later and Bluetooth development libraries
2. Clone the repository
3. Navigate to the client directory
4. Build the client:
   ```bash
   go build -o govee-client govee-client.go
   ```
5. Run the client:
   ```bash
   ./govee-client -server=http://server-address:8080/readings -continuous=true -apikey=YOUR_API_KEY
   ```

## Client Modes

The client can operate in multiple modes, Discovery, Standalone, and Connected:

### Discovery Mode

Use discovery mode to find Govee devices in range:

```bash
./govee-client -discover
```

This will scan for nearby Govee H5075 devices for 30 seconds and then display a table of all discovered devices with their names, MAC addresses, and signal strengths.

Example output:
```
=== Discovered Govee Devices (3 found) ===

Device Name           MAC Address      Signal Strength
-------------------- --------------- ---------------
GVH5075_1234         A4:C1:38:25:A1:E3 -67dBm
GVH5075_5678         A4:C1:38:26:B2:F4 -58dBm
GVH5075_9ABC         A4:C1:38:27:C3:G5 -72dBm

Use these device names/addresses in your monitoring configuration.
```

You can adjust the scan duration for discovery mode:

```bash
./govee-client -discover -duration=1m
```

### Standalone Mode

The client can operate in standalone mode without sending data to a server:

```bash
./govee-client -local=true
```

This is useful for:
- Testing the client before setting up the server
- Temporary monitoring setups
- Scenarios where you don't need centralized data collection
- Situations where network connectivity is limited or unavailable

### Connected Mode

In normal operation, the client connects to the server and sends data:

```bash
./govee-client -server=http://server-address:8080/readings -continuous=true -apikey=YOUR_API_KEY
```

## Configuration

### Client Configuration

The client accepts the following command-line arguments:

| Option | Default | Description |
|--------|---------|-------------|
| `-server` | http://localhost:8080/readings | URL of the server API endpoint |
| `-id` | auto-generated from hostname | Unique ID for this client |
| `-apikey` | "" | API key for server authentication |
| `-duration` | 30s | Duration of each scan cycle |
| `-continuous` | false | Run continuously |
| `-runtime` | 0 (unlimited) | Total runtime (e.g., "1h30m") |
| `-verbose` | false | Show detailed debugging information |
| `-log` | "" | File to log data to (empty for no logging) |
| `-local` | false | Local mode (don't send to server) |
| `-discover` | false | Discovery mode - scan and list devices only |

### Server Configuration

The server accepts the following command-line arguments:

| Option | Default | Description |
|--------|---------|-------------|
| `-port` | 8080 | Server port |
| `-log` | govee-server.log | Log file path |
| `-static` | ./static | Static files directory |
| `-storage` | ./data | Data storage directory |
| `-timeout` | 5m | Client inactivity timeout |
| `-readings` | 1000 | Max readings to store per device |
| `-persist` | true | Enable data persistence |
| `-save-interval` | 5m | Interval for saving data |
| `-auth` | true | Enable API key authentication |
| `-admin-key` | auto-generated | Admin API key (generated if empty) |
| `-default-key` | auto-generated | Default API key for all clients (generated if empty) |
| `-allow-default` | false | Allow the default API key to be used |
| `-time-partition` | true | Enable time-based partitioning of data |
| `-partition-interval` | 720h (30 days) | Interval for new data partitions |
| `-retention` | 0 (unlimited) | How long to keep data (e.g., 8760h for 1 year) |
| `-compress` | true | Compress older partitions to save space |

## Data Storage and Retention

The system provides advanced data management features for historical sensor data:

### Time-Based Partitioning

Data is automatically organized into time-based partitions (daily, weekly, or monthly) for efficient storage and retrieval:

```bash
./govee-server -time-partition=true -partition-interval=720h
```

This creates a directory structure like:
```
data/
├── 2023-04/  # Monthly partition
│   ├── readings_A4C13825A1E3.json
│   └── readings_A4C13826B2F4.json
└── 2023-05/  # Next monthly partition
    ├── readings_A4C13825A1E3.json
    └── readings_A4C13826B2F4.json
```

### Retention Policies

Control how long historical data is kept:

```bash
./govee-server -retention=8760h  # Keep data for one year
```

Data older than the specified retention period is automatically removed.

### Data Compression

Older partitions can be automatically compressed to save storage space:

```bash
./govee-server -compress=true
```

This compresses older JSON files to .gz format while keeping current data uncompressed for fast access.

### Time-Range Queries

Access historical data from specific time periods:

```
GET /readings?device=A4C13825A1E3&from=2023-04-01T00:00:00Z&to=2023-04-30T23:59:59Z
```

For more details, see the [Data Storage and Retention Guide](docs/data-storage-guide.md).

## Authentication

The system uses API key authentication to secure the server:

1. **Admin API Key**: Has full access to all server functions including API key management
2. **Client-Specific API Keys**: Tied to specific client IDs
3. **Default API Key**: Optional shared key that can be used by all clients

### Server Authentication Configuration

When starting the server, you can configure authentication with these flags:

```bash
./govee-server -auth=true -admin-key=YOUR_ADMIN_KEY -default-key=DEFAULT_KEY -allow-default=true
```

### Client Authentication

Clients must provide their API key when sending data to the server:

```bash
./govee-client -server=http://server:8080/readings -apikey=YOUR_API_KEY -id=client-name
```

If no API key is provided, the client will warn you that server communications may fail.

### API Key Management

API keys can be managed through the server's API (requires admin API key):

#### List all API keys

```
GET /api/keys
Header: X-API-Key: <admin_key>
```

#### Create a new API key

```
POST /api/keys
Header: X-API-Key: <admin_key>
Body: {"client_id": "client-name"}
```

#### Delete an API key

```
DELETE /api/keys?key=<api_key_to_delete>
Header: X-API-Key: <admin_key>
```

For more details, see the [Authentication Guide](docs/authentication-guide.md).

## API Endpoints

The server provides the following API endpoints:

| Endpoint | Method | Description | Auth Required |
|----------|--------|-------------|--------------|
| `/readings` | POST | Add a new sensor reading | Yes |
| `/readings?device=<addr>` | GET | Get readings for a specific device | Yes |
| `/devices` | GET | Get all devices and their latest status | Yes |
| `/clients` | GET | Get all clients and their status | Yes |
| `/stats?device=<addr>` | GET | Get statistics for a specific device | Yes |
| `/dashboard/data` | GET | Get all data needed for the dashboard | Yes |
| `/api/keys` | GET/POST/DELETE | Manage API keys | Admin key only |
| `/health` | GET | Health check endpoint | No |

## Dashboard

The dashboard is accessible by navigating to `http://server-address:8080/` in a web browser. It provides:

- System status overview
- Device details and selection
- Temperature and humidity charts
- Client connection status
- Automatic refresh options

## Extended Functionality with InfluxDB and Grafana

For more advanced data analysis and visualization, the system includes optional integration with InfluxDB and Grafana:

1. Start InfluxDB and Grafana using Docker Compose:
   ```bash
   docker-compose up -d influxdb grafana
   ```

2. Access Grafana at `http://server-address:3000/` (default credentials: admin/goveepassword)

3. Configure the InfluxDB data source in Grafana using:
   - URL: `http://influxdb:8086`
   - Organization: `govee`
   - Token: `myauthtoken`
   - Default Bucket: `govee_metrics`

4. Create dashboards to visualize your sensor data

## Troubleshooting

### Client Issues

1. **No devices found**:
   - Ensure Bluetooth is enabled
   - Verify Govee devices are powered on and nearby
   - Run with `-verbose` for detailed output
   - Use discovery mode to check for visible devices: `./govee-client -discover`

2. **Permission issues with Bluetooth**:
   - Run with root/admin privileges or configure BLE permissions
   - For Linux: `sudo setcap 'cap_net_raw,cap_net_admin=eip' ./govee-client`

3. **Cannot connect to server**:
   - Check network connectivity
   - Verify server URL is correct
   - Check server is running and accessible
   - Verify API key is correct

### Server Issues

1. **Server won't start**:
   - Check port availability
   - Verify permissions for log and data directories
   - Check Go installation

2. **Missing dashboard**:
   - Verify static files are available in the specified directory
   - Check browser console for JavaScript errors

3. **Authentication failures**:
   - Check API keys have been correctly set
   - Ensure client IDs match the ones registered with API keys
   - Verify HTTP headers are set correctly

## License

This project is open source and available under the GPL v3 License.

