# Govee Monitoring System - Quick Start Guide

This guide will help you get the Govee Monitoring System up and running quickly.

## Prerequisites

- One or more Govee H5075 temperature and humidity sensors
- A machine with Bluetooth capabilities for the client
- A server or computer to run the central server
- Go 1.18+ installed on both machines (or Docker for containerized setup)

## Step 1: Discover Your Govee Devices

1. Build the client:
   ```bash
   go build -o govee-client govee-client.go
   ```

2. Run the client in discovery mode:
   ```bash
   ./govee-client -discover
   ```

3. Note the device names and addresses from the output:
   ```
   === Discovered Govee Devices (2 found) ===

   Device Name           MAC Address      Signal Strength
   -------------------- --------------- ---------------
   GVH5075_1234         A4:C1:38:25:A1:E3 -67dBm
   GVH5075_5678         A4:C1:38:26:B2:F4 -58dBm
   ```

## Step 2: Set Up the Server

1. Build the server:
   ```bash
   go build -o govee-server govee-server.go
   ```

2. Create required directories:
   ```bash
   mkdir -p data logs static/js
   ```

3. Copy the static files to the appropriate directories:
   ```bash
   cp index.html static/
   cp dashboard.js static/js/
   ```

4. Start the server:
   ```bash
   ./govee-server
   ```

5. Note the generated admin API key that appears in the logs:
   ```
   2023/04/09 15:30:45 Generated admin API key: abcdef123456789
   ```

## Step 3: Create API Keys for Clients

1. Create a client-specific API key using the admin key:
   ```bash
   curl -X POST -H "X-API-Key: YOUR_ADMIN_KEY" -H "Content-Type: application/json" \
     -d '{"client_id": "client-livingroom"}' \
     http://localhost:8080/api/keys
   ```

2. Note the newly created API key from the response:
   ```json
   {"api_key":"xyz789abc123def","client_id":"client-livingroom"}
   ```

## Step 4: Start the Client

1. Run the client with the API key and server address:
   ```bash
   ./govee-client -server=http://server-address:8080/readings -apikey=xyz789abc123def -id=client-livingroom -continuous=true
   ```

2. You should see temperature and humidity readings start to appear:
   ```
   2023-04-09T15:35:20 GVH5075_1234 Temp: 22.5°C/72.5°F, Humidity: 45.5%, Battery: 87%, RSSI: -67dBm
   ```

## Step 5: Access the Dashboard

1. Open a web browser and navigate to:
   ```
   http://server-address:8080/
   ```

2. You should see the dashboard with your devices and readings.

3. Select a device from the dropdown to view detailed information and charts.

## Optional: Run in Docker

### Server

1. Build the Docker image:
   ```bash
   docker build -t govee-server ./server
   ```

2. Run the container:
   ```bash
   docker run -d -p 8080:8080 -v $(pwd)/data:/app/data -v $(pwd)/logs:/app/logs -v $(pwd)/static:/app/static govee-server
   ```

### Client

1. Build the Docker image:
   ```bash
   docker build -t govee-client ./client
   ```

2. Run the container with host network to access Bluetooth:
   ```bash
   docker run --net=host -e SERVER_URL=http://server-address:8080/readings -e CLIENT_ID=client-livingroom -e APIKEY=xyz789abc123def govee-client
   ```

## Troubleshooting

- **No devices found**: Ensure Bluetooth is enabled and Govee devices are powered on
- **Authentication failed**: Verify the API key and client ID match what's registered on the server
- **Cannot access dashboard**: Check that static files are in the correct location and server is running
- **Bluetooth permission issues**: Try running the client with sudo or set appropriate capabilities

## What's Next?

- Add more clients to monitor different areas
- Set up InfluxDB and Grafana for advanced data visualization
- Configure Docker Compose for easier deployment

For more detailed information, refer to the full [README.md](README.md) and [Authentication Guide](Authentication-Guide.md).
