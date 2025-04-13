FROM golang:1.18-alpine

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first to leverage Docker caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 go build -o govee-server ./govee-server.go

# Create necessary directories
RUN mkdir -p /app/data /app/logs

# Use a minimal Alpine image for the final image
FROM alpine:3.14

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=0 /app/govee-server /app/
COPY --from=0 /app/static /app/static

# Create necessary directories
RUN mkdir -p /app/data /app/logs

# Set executable permissions
RUN chmod +x /app/govee-server

# Set environment variables
ENV PORT=8080
ENV LOG_FILE=/app/logs/govee-server.log
ENV STORAGE_DIR=/app/data
ENV CLIENT_TIMEOUT=5m

# Expose the server port
EXPOSE 8080

# Run the application
CMD ["/app/govee-server", "-port=${PORT}", "-log=${LOG_FILE}", "-storage=${STORAGE_DIR}", "-timeout=${CLIENT_TIMEOUT}", "-static=/app/static"]
