# Update client/Dockerfile to include the new environment variables

FROM golang:1.18-alpine

WORKDIR /app

# Install build dependencies and Bluetooth dependencies
RUN apk add --no-cache git bluez-dev linux-headers musl-dev

# Copy go.mod and go.sum first to leverage Docker caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o govee-client ./govee_5075_client.go

# Use a minimal Alpine image for the final image
FROM alpine:3.14

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

# Run the application
CMD ["/bin/sh", "-c", "/app/govee-client -server=${SERVER_URL} -id=${CLIENT_ID} -apikey=${APIKEY} -duration=${SCAN_DURATION} -continuous=${CONTINUOUS} -log=${LOG_FILE} -temp-offset=${TEMP_OFFSET} -humidity-offset=${HUMIDITY_OFFSET}"]
