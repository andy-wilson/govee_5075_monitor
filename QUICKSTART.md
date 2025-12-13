# Quick Start Guide - Govee 5075 Monitor

Get up and running with the Govee 5075 Monitor in 5 minutes.

## Prerequisites

- Docker and Docker Compose installed
- Govee H5075 sensor(s)
- Linux/macOS/Windows with Bluetooth 4.0+

---

## 1. Clone the Repository

```bash
git clone https://github.com/andy-wilson/govee_5075_monitor.git
cd govee_5075_monitor
```

---

## 2. Start the Server

### Using Docker (Recommended)

```bash
# Start the server
cd server
docker-compose up -d

# Check it's running
curl http://localhost:8080/health
```

You should see:
```json
{
  "status": "healthy",
  "timestamp": "2025-12-09T...",
  "uptime": "10s",
  "version": "2.0.0",
  ...
}
```

### Using Go (Alternative)

```bash
cd server
go build -o govee-server .
./govee-server
```

---

## 3. Get Your API Keys

The server auto-generates API keys on first start. Check the logs:

```bash
# View server logs
docker-compose logs govee-server | grep "API key"
```

Or read from the auth file:
```bash
cat data/auth.json
```

Copy the `admin_key` for managing the system and create client keys as needed.

---

## 4. Discover Your Sensors

Find nearby Govee sensors:

```bash
cd client
go build -o govee-client .
./govee-client -discover
```

Output:
```
=== Discovered Govee Devices (2 found) ===

Device Name           MAC Address       Signal Strength
-------------------- ----------------- ---------------
GVH5075_1234         A4:C1:38:25:A1:E3 -67dBm
GVH5075_Living       A4:C1:38:26:B2:F4 -58dBm
```

---

## 5. Start the Client

### Using Docker (Recommended)

Edit `client/docker-compose.yaml`:

```yaml
environment:
  - SERVER_URL=http://YOUR_SERVER_IP:8080/readings
  - CLIENT_ID=livingroom-sensor
  - APIKEY=your_api_key_here
```

Then start:
```bash
cd client
docker-compose up -d
```

### Using Go (Alternative)

```bash
./govee-client \
  -server=http://YOUR_SERVER_IP:8080/readings \
  -id=livingroom-sensor \
  -apikey=YOUR_API_KEY \
  -continuous=true
```

---

## 6. View the Dashboard

Open your browser:
```
http://YOUR_SERVER_IP:8080/
```

You should see:
- List of discovered devices
- Real-time temperature/humidity charts
- Battery levels and signal strength
- Client connection status

---

## 7. Verify Everything Works

### Check Server Health
```bash
curl http://localhost:8080/health | jq
```

### View Devices
```bash
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/devices | jq
```

### View Readings
```bash
curl -H "X-API-Key: YOUR_API_KEY" \
  "http://localhost:8080/readings?device=A4C13825A1E3" | jq
```

---

## Common Issues

### Client can't find sensors
```bash
# Check Bluetooth is working
hcitool dev

# Run discovery with verbose output
./govee-client -discover -verbose
```

### Permission denied (Linux)
```bash
# Give Bluetooth permissions
sudo setcap 'cap_net_raw,cap_net_admin=eip' ./govee-client

# Or run with sudo
sudo ./govee-client -discover
```

### Can't connect to server
```bash
# Check server is running
docker ps

# Check server logs
docker-compose logs govee-server

# Verify network connectivity
ping YOUR_SERVER_IP
```

### Authentication failures
- Verify API key is correct
- Check the `X-API-Key` header is set
- Ensure `-auth=true` on server (default)

---

## Next Steps

### Optimize Storage (Recommended)

Migrate to SQLite for better performance:

```go
// Create migration script: migrate.go
package main

import (
    "log"
    // Import from server package
)

func main() {
    err := MigrateJSONToSQLite("./data", "./data/readings.db")
    if err != nil {
        log.Fatal(err)
    }
    log.Println("Migration complete!")
}
```

Run:
```bash
cd server
go run migrate.go
```

Benefits:
- 10-100x faster queries
- Better scalability
- Efficient filtering

### Enable HTTPS

Generate certificates:
```bash
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout server/certs/key.pem \
  -out server/certs/cert.pem \
  -days 365
```

Start server with HTTPS:
```bash
./govee-server -https=true \
  -cert=./certs/cert.pem \
  -key=./certs/key.pem
```

### Add More Clients

Run clients on multiple machines:
```bash
# Kitchen
./govee-client -id=kitchen -server=... -apikey=...

# Bedroom
./govee-client -id=bedroom -server=... -apikey=...

# Garage
./govee-client -id=garage -server=... -apikey=...
```

### Set Up Retention

Keep data for 1 year:
```bash
./govee-server -retention=8760h
```

### Configure Alerts (Optional)

Use InfluxDB + Grafana:
```bash
cd server
docker-compose up -d influxdb grafana
```

Access Grafana at `http://localhost:3000` (admin/admin)

---

## Performance Tips

### 1. Use SQLite Storage
- 10-100x faster than JSON
- Better for large datasets
- See migration guide above

### 2. Adjust Scan Duration
```bash
# Faster scans (less accurate)
./govee-client -duration=10s

# Longer scans (more reliable)
./govee-client -duration=60s
```

### 3. Enable Compression
Already enabled by default in v2.0:
- 80% bandwidth reduction
- Faster dashboard loads

### 4. Monitor Health
```bash
# Check system status
curl http://localhost:8080/health | jq '.stats'

# Check goroutine count
curl http://localhost:8080/health | jq '.goroutines'
```

---

## Documentation

- **README.md** - Complete system documentation
- **docs/OPTIMIZATION_GUIDE.md** - Performance tuning and upgrades
- **docs/IMPLEMENTATION_SUMMARY.md** - Technical details
- **docs/authentication-guide.md** - Security setup
- **CODE_REVIEW.md** - Security audit
- **FIXES_APPLIED.md** - Changelog

---

## Getting Help

- **Issues**: https://github.com/andy-wilson/govee_5075_monitor/issues
- **Discussions**: https://github.com/andy-wilson/govee_5075_monitor/discussions

---

## Quick Commands Cheatsheet

```bash
# Server
docker-compose up -d                    # Start server
docker-compose logs -f                  # View logs
curl http://localhost:8080/health       # Check health
docker-compose down                     # Stop server

# Client
./govee-client -discover                    # Find sensors
./govee-client -local                       # Test locally (all readings)
./govee-client -local -single               # One reading per device
./govee-client -local -device=GVH5075_8F19  # Filter by device
./govee-client -server=... -apikey=...      # Connect to server
docker-compose up -d                        # Run in Docker

# Testing
go test -v ./server/...                 # Run tests
go test -bench=. ./server/...           # Run benchmarks

# Maintenance
docker-compose restart                  # Restart services
docker system prune                     # Clean up Docker
docker-compose pull                     # Update images
```

---

**You're all set! Your Govee monitoring system is now running. 🎉**

For advanced configuration, see [README.md](README.md) and [docs/OPTIMIZATION_GUIDE.md](docs/OPTIMIZATION_GUIDE.md).
