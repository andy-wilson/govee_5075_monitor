# Client Dockerfile for Govee BLE scanner

FROM golang:1.21-alpine3.19 AS builder

WORKDIR /app

# Install build dependencies and Bluetooth dependencies
RUN apk add --no-cache git bluez-dev linux-headers musl-dev

# Copy go.mod and go.sum first to leverage Docker caching
COPY ../go.mod ../go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application (FIXED: correct filename)
RUN go build -o govee-client ./govee-client.go

# Use a minimal Alpine image for the final image
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies for Bluetooth
RUN apk add --no-cache bluez-libs

# Copy the binary from the builder stage
COPY --from=0 /app/govee-client /app/

# Set executable permissions
RUN chmod +x /app/govee-client

# Set environment variables with defaults
ENV SERVER_URL=http://govee-server:8080/readings
ENV CLIENT_ID=""
ENV APIKEY=""
ENV SCAN_DURATION=30s
ENV CONTINUOUS=true
ENV LOG_FILE=""
ENV TEMP_OFFSET=0.0
ENV HUMIDITY_OFFSET=0.0
ENV HTTP_TIMEOUT=10s
ENV INSECURE=false
ENV CA_CERT=""

# Add health check (verify process is running)
HEALTHCHECK --interval=60s --timeout=10s --start-period=30s --retries=3 \
  CMD ps aux | grep -v grep | grep govee-client || exit 1

# Run the application
CMD ["/bin/sh", "-c", "/app/govee-client -server=${SERVER_URL} -id=${CLIENT_ID} -apikey=${APIKEY} -duration=${SCAN_DURATION} -continuous=${CONTINUOUS} -log=${LOG_FILE} -temp-offset=${TEMP_OFFSET} -humidity-offset=${HUMIDITY_OFFSET} -http-timeout=${HTTP_TIMEOUT} -insecure-skip-tls-verify-dangerous=${INSECURE} -ca-cert=${CA_CERT}"]
