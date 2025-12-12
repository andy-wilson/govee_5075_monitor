# Govee 5075 Monitor Makefile
# ============================

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod

# Binary names
SERVER_BINARY=govee-server
CLIENT_BINARY=govee-client

# Docker image names
DOCKER_SERVER_IMAGE=govee-server
DOCKER_CLIENT_IMAGE=govee-client
DOCKER_TAG?=latest

# Directories
SERVER_DIR=server
CLIENT_DIR=client
STATIC_DIR=static
DATA_DIR=data

# Build flags
LDFLAGS=-ldflags "-s -w"
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default target
.PHONY: all
all: build

# ============================================================================
# Binary Build Targets
# ============================================================================

.PHONY: build
build: build-server build-client ## Build both server and client binaries

.PHONY: server
server: build-server ## Alias for build-server

.PHONY: client
client: build-client ## Alias for build-client

.PHONY: build-server
build-server: ## Build the server binary
	cd $(SERVER_DIR) && $(GOBUILD) $(LDFLAGS) -o $(SERVER_BINARY) govee-server.go storage.go migrate.go

.PHONY: build-client
build-client: ## Build the client binary
	cd $(CLIENT_DIR) && $(GOBUILD) $(LDFLAGS) -o $(CLIENT_BINARY) govee-client.go

# ============================================================================
# Test targets
# ============================================================================

.PHONY: test
test: ## Run all tests
	$(GOTEST) -v ./server/... ./client/...

.PHONY: test-server
test-server: ## Run server tests only
	$(GOTEST) -v ./server/...

.PHONY: test-client
test-client: ## Run client tests only
	$(GOTEST) -v ./client/...

.PHONY: test-race
test-race: ## Run tests with race detection
	$(GOTEST) -v -race ./server/... ./client/...

.PHONY: test-short
test-short: ## Run tests in short mode
	$(GOTEST) -v -short ./server/... ./client/...

.PHONY: coverage
coverage: ## Generate coverage report
	$(GOTEST) -coverprofile=coverage.out ./server/... ./client/...
	$(GOCMD) tool cover -func=coverage.out

.PHONY: coverage-html
coverage-html: coverage ## Generate HTML coverage report
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: benchmark
benchmark: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./server/... ./client/...

# ============================================================================
# Code quality targets
# ============================================================================

.PHONY: fmt
fmt: ## Format Go code
	$(GOFMT) ./...

.PHONY: vet
vet: ## Run go vet
	$(GOVET) ./...

.PHONY: lint
lint: ## Run golangci-lint (must be installed)
	golangci-lint run ./...

.PHONY: check
check: fmt vet ## Run fmt and vet

# ============================================================================
# Run targets
# ============================================================================

.PHONY: run-server
run-server: build-server ## Build and run the server
	cd $(SERVER_DIR) && ./$(SERVER_BINARY) -port=8080 -static=../$(STATIC_DIR)

.PHONY: run-client-discover
run-client-discover: build-client ## Build and run client in discovery mode
	cd $(CLIENT_DIR) && ./$(CLIENT_BINARY) -discover -duration=30s

.PHONY: run-client-local
run-client-local: build-client ## Build and run client in local/standalone mode
	cd $(CLIENT_DIR) && ./$(CLIENT_BINARY) -local=true -continuous=true

# ============================================================================
# Container Build Targets
# ============================================================================

.PHONY: container
container: container-server container-client ## Build both server and client containers

.PHONY: container-server
container-server: ## Build server Docker image
	docker build -t $(DOCKER_SERVER_IMAGE):$(DOCKER_TAG) -f $(SERVER_DIR)/Dockerfile .

.PHONY: container-client
container-client: ## Build client Docker image
	docker build -t $(DOCKER_CLIENT_IMAGE):$(DOCKER_TAG) -f $(CLIENT_DIR)/Dockerfile .

# Aliases for docker-build
.PHONY: docker-build
docker-build: container ## Alias for container

.PHONY: docker-build-server
docker-build-server: container-server ## Alias for container-server

.PHONY: docker-build-client
docker-build-client: container-client ## Alias for container-client

# ============================================================================
# Docker Compose Targets
# ============================================================================

.PHONY: docker-up-server
docker-up-server: ## Start server with docker-compose
	docker-compose -f $(SERVER_DIR)/docker-compose.yaml up -d

.PHONY: docker-down-server
docker-down-server: ## Stop server docker-compose
	docker-compose -f $(SERVER_DIR)/docker-compose.yaml down

.PHONY: docker-up-client
docker-up-client: ## Start client with docker-compose
	docker-compose -f $(CLIENT_DIR)/docker-compose.yaml up -d

.PHONY: docker-down-client
docker-down-client: ## Stop client docker-compose
	docker-compose -f $(CLIENT_DIR)/docker-compose.yaml down

.PHONY: docker-up
docker-up: docker-up-server ## Start all services with docker-compose

.PHONY: docker-down
docker-down: docker-down-server docker-down-client ## Stop all docker-compose services

.PHONY: docker-logs-server
docker-logs-server: ## Show server container logs
	docker-compose -f $(SERVER_DIR)/docker-compose.yaml logs -f

.PHONY: docker-logs-client
docker-logs-client: ## Show client container logs
	docker-compose -f $(CLIENT_DIR)/docker-compose.yaml logs -f

# ============================================================================
# Dependency targets
# ============================================================================

.PHONY: deps
deps: ## Download dependencies
	$(GOMOD) download

.PHONY: deps-tidy
deps-tidy: ## Tidy dependencies
	$(GOMOD) tidy

.PHONY: deps-verify
deps-verify: ## Verify dependencies
	$(GOMOD) verify

# ============================================================================
# Clean targets
# ============================================================================

.PHONY: clean
clean: ## Remove build artifacts
	$(GOCLEAN)
	rm -f $(SERVER_DIR)/$(SERVER_BINARY)
	rm -f $(CLIENT_DIR)/$(CLIENT_BINARY)
	rm -f coverage.out coverage.html
	rm -f $(SERVER_DIR)/coverage.out $(SERVER_DIR)/coverage.html
	rm -f $(CLIENT_DIR)/coverage.out $(CLIENT_DIR)/coverage.html

.PHONY: clean-data
clean-data: ## Remove data directory (CAUTION: deletes all stored readings)
	rm -rf $(DATA_DIR)

# ============================================================================
# Install targets
# ============================================================================

.PHONY: install-server
install-server: build-server ## Install server to GOPATH/bin
	cp $(SERVER_DIR)/$(SERVER_BINARY) $(GOPATH)/bin/

.PHONY: install-client
install-client: build-client ## Install client to GOPATH/bin
	cp $(CLIENT_DIR)/$(CLIENT_BINARY) $(GOPATH)/bin/

.PHONY: install
install: install-server install-client ## Install both binaries to GOPATH/bin

# ============================================================================
# Help target
# ============================================================================

.PHONY: help
help: ## Show this help message
	@echo "Govee 5075 Monitor - Available targets:"
	@echo ""
	@echo "Build Binaries:"
	@echo "  \033[36mmake build\033[0m            Build both server and client binaries"
	@echo "  \033[36mmake server\033[0m           Build server binary only"
	@echo "  \033[36mmake client\033[0m           Build client binary only"
	@echo ""
	@echo "Build Containers:"
	@echo "  \033[36mmake container\033[0m        Build both server and client containers"
	@echo "  \033[36mmake container-server\033[0m Build server container only"
	@echo "  \033[36mmake container-client\033[0m Build client container only"
	@echo ""
	@echo "All targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make build                    # Build server and client binaries"
	@echo "  make container                # Build server and client containers"
	@echo "  make server                   # Build server binary only"
	@echo "  make container-client         # Build client container only"
	@echo "  make test                     # Run all tests"
	@echo "  make DOCKER_TAG=v2.0 container # Build containers with custom tag"
