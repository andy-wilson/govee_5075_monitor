.PHONY: test test-server test-client test-integration test-coverage build clean

# Default Go command
GO=go

# Server and client main files
SERVER_MAIN=./cmd/server/govee_server.go
CLIENT_MAIN=./cmd/client/govee_client.go

# Output directories
BIN_DIR=./bin
COVERAGE_DIR=./coverage

# Build all binaries
build:
	@echo "Building Govee Monitoring System..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/govee-server $(SERVER_MAIN)
	$(GO) build -o $(BIN_DIR)/govee-client $(CLIENT_MAIN)
	@echo "Build completed. Binaries are in $(BIN_DIR) directory."

# Run all tests
test:
	@echo "Running all tests..."
	$(GO) test -v ./...

# Run server tests only
test-server:
	@echo "Running server tests..."
	$(GO) test -v ./server/...

# Run client tests only
test-client:
	@echo "Running client tests..."
	$(GO) test -v ./client/...

# Run integration tests only
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v ./integration/...

# Run mock-based BLE tests only
test-ble-mock:
	@echo "Running BLE tests with mocks..."
	$(GO) test -v -run TestBLEScan ./client/...

# Run tests with coverage reporting
test-coverage:
	@echo "Running tests with coverage analysis..."
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated at $(COVERAGE_DIR)/coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GO) test -race -v ./...

# Lint the code
lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Clean up binaries and test artifacts
clean:
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -rf $(COVERAGE_DIR)
	$(GO) clean
	@echo "Clean complete."

# Help target
help:
	@echo "Govee Monitoring System Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build all binaries"
	@echo "  test           - Run all tests"
	@echo "  test-server    - Run server tests only"
	@echo "  test-client    - Run client tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-ble-mock  - Run BLE tests with mocks"
	@echo "  test-coverage  - Run tests with coverage analysis"
	@echo "  test-race      - Run tests with race detection"
	@echo "  lint           - Lint the code"
	@echo "  clean          - Clean up build artifacts"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make test"
	@echo "  make test-coverage"
