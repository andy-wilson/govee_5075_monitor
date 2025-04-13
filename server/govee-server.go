package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// Reading represents a single measurement from a Govee device
type Reading struct {
	DeviceName     string    `json:"device_name"`
	DeviceAddr     string    `json:"device_addr"`
	TempC          float64   `json:"temp_c"`
	TempF          float64   `json:"temp_f"`
	TempOffset     float64   `json:"temp_offset"`
	Humidity       float64   `json:"humidity"`
	HumidityOffset float64   `json:"humidity_offset"`
	AbsHumidity    float64   `json:"abs_humidity"`
	DewPointC      float64   `json:"dew_point_c"`
	DewPointF      float64   `json:"dew_point_f"`
	SteamPressure  float64   `json:"steam_pressure"`
	Battery        int       `json:"battery"`
	RSSI           int       `json:"rssi"`
	Timestamp      time.Time `json:"timestamp"`
	ClientID       string    `json:"client_id"`
}

// DeviceStatus represents the latest status of a device
type DeviceStatus struct {
	DeviceName     string    `json:"device_name"`
	DeviceAddr     string    `json:"device_addr"`
	TempC          float64   `json:"temp_c"`
	TempF          float64   `json:"temp_f"`
	TempOffset     float64   `json:"temp_offset"`
	Humidity       float64   `json:"humidity"`
	HumidityOffset float64   `json:"humidity_offset"`
	AbsHumidity    float64   `json:"abs_humidity"`
	DewPointC      float64   `json:"dew_point_c"`
	DewPointF      float64   `json:"dew_point_f"`
	SteamPressure  float64   `json:"steam_pressure"`
	Battery        int       `json:"battery"`
	RSSI           int       `json:"rssi"`
	LastUpdate     time.Time `json:"last_update"`
	ClientID       string    `json:"client_id"`
	LastSeen       time.Time `json:"last_seen"`
	ReadingCount   int       `json:"reading_count"`
}

// ClientStatus represents the latest status of a client
type ClientStatus struct {
	ClientID        string    `json:"client_id"`
	LastSeen        time.Time `json:"last_seen"`
	DeviceCount     int       `json:"device_count"`
	ReadingCount    int       `json:"reading_count"`
	ConnectedSince  time.Time `json:"connected_since"`
	IsActive        bool      `json:"is_active"`
	InactiveTimeout time.Duration
}

// AuthConfig represents configuration for API keys
type AuthConfig struct {
	EnableAuth      bool              `json:"enable_auth"`
	APIKeys         map[string]string `json:"api_keys"` // Map of API key -> client ID
	AdminKey        string            `json:"admin_key"`
	DefaultAPIKey   string            `json:"default_api_key"`
	AllowDefaultKey bool              `json:"allow_default_key"`
}

// Server represents the Govee server
type Server struct {
	// Maps device address to device status
	devices map[string]*DeviceStatus
	// Maps client ID to client status
	clients map[string]*ClientStatus
	// Stores readings for a device
	readings map[string][]Reading
	// Mutex for thread safety
	mu sync.RWMutex
	// File logger
	logger *os.File
	// Configuration settings
	config *Config
	// Authentication configuration
	auth *AuthConfig
}

// Config represents server configuration
type Config struct {
	Port               int           `json:"port"`
	LogFile            string        `json:"log_file"`
	ClientTimeout      time.Duration `json:"client_timeout"`
	ReadingsPerDevice  int           `json:"readings_per_device"`
	StorageDir         string        `json:"storage_dir"`
	PersistenceEnabled bool          `json:"persistence_enabled"`
	SaveInterval       time.Duration `json:"save_interval"`
}

// NewServer creates a new Govee server instance
func NewServer(config *Config, auth *AuthConfig) *Server {
	s := &Server{
		devices:  make(map[string]*DeviceStatus),
		clients:  make(map[string]*ClientStatus),
		readings: make(map[string][]Reading),
		config:   config,
		auth:     auth,
	}

	// Initialize logging if configured
	if config.LogFile != "" {
		logger, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
		} else {
			s.logger = logger
			log.Printf("Logging data to %s", config.LogFile)
		}
	}

	// Start persistence if enabled
	if config.PersistenceEnabled {
		// Create storage directory if it doesn't exist
		if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
			log.Printf("Failed to create storage directory: %v", err)
		}

		// Start background save routine
		go s.startPersistence()
	}

	// Start client timeout check routine
	go s.checkClientTimeouts()

	return s
}

// startPersistence starts the background routine for data persistence
func (s *Server) startPersistence() {
	ticker := time.NewTicker(s.config.SaveInterval)
	for range ticker.C {
		s.saveData()
	}
}

// saveData saves current server state to disk
func (s *Server) saveData() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Save device statuses
	devicesData, err := json.MarshalIndent(s.devices, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal devices data: %v", err)
	} else {
		if err := os.WriteFile(fmt.Sprintf("%s/devices.json", s.config.StorageDir), devicesData, 0644); err != nil {
			log.Printf("Failed to save devices data: %v", err)
		}
	}

	// Save client statuses
	clientsData, err := json.MarshalIndent(s.clients, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal clients data: %v", err)
	} else {
		if err := os.WriteFile(fmt.Sprintf("%s/clients.json", s.config.StorageDir), clientsData, 0644); err != nil {
			log.Printf("Failed to save clients data: %v", err)
		}
	}

	// Save API keys if auth is enabled
	if s.auth.EnableAuth {
		authData, err := json.MarshalIndent(s.auth, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal auth data: %v", err)
		} else {
			if err := os.WriteFile(fmt.Sprintf("%s/auth.json", s.config.StorageDir), authData, 0644); err != nil {
				log.Printf("Failed to save auth data: %v", err)
			}
		}
	}

	// Save recent readings for each device
	for deviceAddr, deviceReadings := range s.readings {
		if len(deviceReadings) > 0 {
			readingsData, err := json.MarshalIndent(deviceReadings, "", "  ")
			if err != nil {
				log.Printf("Failed to marshal readings for device %s: %v", deviceAddr, err)
			} else {
				deviceFile := fmt.Sprintf("%s/readings_%s.json", s.config.StorageDir, deviceAddr)
				if err := os.WriteFile(deviceFile, readingsData, 0644); err != nil {
					log.Printf("Failed to save readings for device %s: %v", deviceAddr, err)
				}
			}
		}
	}

	log.Println("Data saved to storage")
}

// loadData loads server state from disk
func (s *Server) loadData() {
	// Load device statuses
	devicesData, err := os.ReadFile(fmt.Sprintf("%s/devices.json", s.config.StorageDir))
	if err == nil {
		if err := json.Unmarshal(devicesData, &s.devices); err != nil {
			log.Printf("Failed to unmarshal devices data: %v", err)
		} else {
			log.Printf("Loaded %d devices from storage", len(s.devices))
		}
	}

	// Load client statuses
	clientsData, err := os.ReadFile(fmt.Sprintf("%s/clients.json", s.config.StorageDir))
	if err == nil {
		if err := json.Unmarshal(clientsData, &s.clients); err != nil {
			log.Printf("Failed to unmarshal clients data: %v", err)
		} else {
			log.Printf("Loaded %d clients from storage", len(s.clients))
		}
	}

	// Load auth configuration if auth is enabled
	if s.auth.EnableAuth {
		authData, err := os.ReadFile(fmt.Sprintf("%s/auth.json", s.config.StorageDir))
		if err == nil {
			var loadedAuth AuthConfig
			if err := json.Unmarshal(authData, &loadedAuth); err != nil {
				log.Printf("Failed to unmarshal auth data: %v", err)
			} else {
				// Only update the API keys, preserve other settings from command line
				s.auth.APIKeys = loadedAuth.APIKeys
				log.Printf("Loaded %d API keys from storage", len(s.auth.APIKeys))
			}
		}
	}

	// Mark all clients as inactive initially
	for _, client := range s.clients {
		client.IsActive = false
	}
}

// checkClientTimeouts periodically checks for inactive clients
func (s *Server) checkClientTimeouts() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for clientID, client := range s.clients {
			if now.Sub(client.LastSeen) > s.config.ClientTimeout {
				client.IsActive = false
				log.Printf("Client %s marked as inactive (timeout: %v)", clientID, s.config.ClientTimeout)
			}
		}
		s.mu.Unlock()
	}
}

// addReading adds a new reading to the server
func (s *Server) addReading(reading Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceAddr := reading.DeviceAddr
	clientID := reading.ClientID

	// Update device status
	if device, exists := s.devices[deviceAddr]; exists {
		device.TempC = reading.TempC
		device.TempF = reading.TempF
		device.TempOffset = reading.TempOffset
		device.Humidity = reading.Humidity
		device.HumidityOffset = reading.HumidityOffset
		device.AbsHumidity = reading.AbsHumidity
		device.DewPointC = reading.DewPointC
		device.DewPointF = reading.DewPointF
		device.SteamPressure = reading.SteamPressure
		device.Battery = reading.Battery
		device.RSSI = reading.RSSI
		device.LastUpdate = reading.Timestamp
		device.LastSeen = time.Now()
		device.ClientID = clientID
		device.ReadingCount++
	} else {
		s.devices[deviceAddr] = &DeviceStatus{
			DeviceName:     reading.DeviceName,
			DeviceAddr:     deviceAddr,
			TempC:          reading.TempC,
			TempF:          reading.TempF,
			TempOffset:     reading.TempOffset,
			Humidity:       reading.Humidity,
			HumidityOffset: reading.HumidityOffset,
			AbsHumidity:    reading.AbsHumidity,
			DewPointC:      reading.DewPointC,
			DewPointF:      reading.DewPointF,
			SteamPressure:  reading.SteamPressure,
			Battery:        reading.Battery,
			RSSI:           reading.RSSI,
			LastUpdate:     reading.Timestamp,
			LastSeen:       time.Now(),
			ClientID:       clientID,
			ReadingCount:   1,
		}
	}

	// Update client status
	if client, exists := s.clients[clientID]; exists {
		client.LastSeen = time.Now()
		client.ReadingCount++
		client.IsActive = true
	} else {
		s.clients[clientID] = &ClientStatus{
			ClientID:        clientID,
			LastSeen:        time.Now(),
			DeviceCount:     1,
			ReadingCount:    1,
			ConnectedSince:  time.Now(),
			IsActive:        true,
			InactiveTimeout: s.config.ClientTimeout,
		}
	}

	// Update client's device count
	devicesByClient := make(map[string]map[string]bool)
	for addr, device := range s.devices {
		if _, exists := devicesByClient[device.ClientID]; !exists {
			devicesByClient[device.ClientID] = make(map[string]bool)
		}
		devicesByClient[device.ClientID][addr] = true
	}

	for cID, devices := range devicesByClient {
		if client, exists := s.clients[cID]; exists {
			client.DeviceCount = len(devices)
		}
	}

	// Store reading
	if _, exists := s.readings[deviceAddr]; !exists {
		s.readings[deviceAddr] = make([]Reading, 0)
	}

	// Append reading and maintain maximum size
	readings := append(s.readings[deviceAddr], reading)
	if len(readings) > s.config.ReadingsPerDevice {
		readings = readings[len(readings)-s.config.ReadingsPerDevice:]
	}
	s.readings[deviceAddr] = readings

	// Log reading if logger is available
	if s.logger != nil {
		logEntry, _ := json.Marshal(reading)
		s.logger.WriteString(string(logEntry) + "\n")
	}
}

// getDevices returns all device statuses
func (s *Server) getDevices() []*DeviceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*DeviceStatus, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}
	return devices
}

// getClients returns all client statuses
func (s *Server) getClients() []*ClientStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*ClientStatus, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	return clients
}

// getDeviceReadings returns readings for a specific device
func (s *Server) getDeviceReadings(deviceAddr string) []Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if readings, exists := s.readings[deviceAddr]; exists {
		return readings
	}
	return []Reading{}
}

// getDeviceStats returns statistics for a specific device
func (s *Server) getDeviceStats(deviceAddr string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})
	if readings, exists := s.readings[deviceAddr]; exists && len(readings) > 0 {
		// Calculate min, max, avg for primary metrics
		var sumTempC, sumHumidity, sumAbsHumidity, sumDewPointC, sumSteamPressure float64
		var minTempC, maxTempC = readings[0].TempC, readings[0].TempC
		var minHumidity, maxHumidity = readings[0].Humidity, readings[0].Humidity
		var minDewPointC, maxDewPointC = readings[0].DewPointC, readings[0].DewPointC
		var minAbsHumidity, maxAbsHumidity = readings[0].AbsHumidity, readings[0].AbsHumidity
		var minSteamPressure, maxSteamPressure = readings[0].SteamPressure, readings[0].SteamPressure

		for _, r := range readings {
			sumTempC += r.TempC
			sumHumidity += r.Humidity
			sumDewPointC += r.DewPointC
			sumAbsHumidity += r.AbsHumidity
			sumSteamPressure += r.SteamPressure

			if r.TempC < minTempC {
				minTempC = r.TempC
			}
			if r.TempC > maxTempC {
				maxTempC = r.TempC
			}
			if r.Humidity < minHumidity {
				minHumidity = r.Humidity
			}
			if r.Humidity > maxHumidity {
				maxHumidity = r.Humidity
			}
			if r.DewPointC < minDewPointC {
				minDewPointC = r.DewPointC
			}
			if r.DewPointC > maxDewPointC {
				maxDewPointC = r.DewPointC
			}
			if r.AbsHumidity < minAbsHumidity {
				minAbsHumidity = r.AbsHumidity
			}
			if r.AbsHumidity > maxAbsHumidity {
				maxAbsHumidity = r.AbsHumidity
			}
			if r.SteamPressure < minSteamPressure {
				minSteamPressure = r.SteamPressure
			}
			if r.SteamPressure > maxSteamPressure {
				maxSteamPressure = r.SteamPressure
			}
		}

		count := float64(len(readings))
		stats["count"] = len(readings)

		// Temperature stats
		stats["temp_c_min"] = minTempC
		stats["temp_c_max"] = maxTempC
		stats["temp_c_avg"] = sumTempC / count

		// Humidity stats
		stats["humidity_min"] = minHumidity
		stats["humidity_max"] = maxHumidity
		stats["humidity_avg"] = sumHumidity / count

		// Dew point stats
		stats["dew_point_c_min"] = minDewPointC
		stats["dew_point_c_max"] = maxDewPointC
		stats["dew_point_c_avg"] = sumDewPointC / count

		// Absolute humidity stats
		stats["abs_humidity_min"] = minAbsHumidity
		stats["abs_humidity_max"] = maxAbsHumidity
		stats["abs_humidity_avg"] = sumAbsHumidity / count

		// Steam pressure stats
		stats["steam_pressure_min"] = minSteamPressure
		stats["steam_pressure_max"] = maxSteamPressure
		stats["steam_pressure_avg"] = sumSteamPressure / count

		// Add first and last readings timestamps
		stats["first_reading"] = readings[0].Timestamp
		stats["last_reading"] = readings[len(readings)-1].Timestamp
	}
	return stats
}

// Authentication middleware
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if disabled
		if !s.auth.EnableAuth {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for GET requests to public endpoints
		if r.Method == "GET" && (strings.HasPrefix(r.URL.Path, "/") || r.URL.Path == "/health") {
			next.ServeHTTP(w, r)
			return
		}

		// Check for API key in header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
			log.Printf("Authentication failed: No API key provided from %s", r.RemoteAddr)
			return
		}

		// Check if it's the admin key
		if apiKey == s.auth.AdminKey {
			// Admin key has access to everything
			next.ServeHTTP(w, r)
			return
		}

		// Check if it's the default key (if allowed)
		if s.auth.AllowDefaultKey && apiKey == s.auth.DefaultAPIKey {
			next.ServeHTTP(w, r)
			return
		}

		// Check if the API key is valid
		clientID, valid := s.auth.APIKeys[apiKey]
		if !valid {
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			log.Printf("Authentication failed: Invalid API key from %s", r.RemoteAddr)
			return
		}

		// For POST to /readings, check if the client ID in the request matches the one associated with the API key
		if r.Method == "POST" && r.URL.Path == "/readings" {
			var reading Reading
			if err := json.NewDecoder(r.Body).Decode(&reading); err == nil {
				if reading.ClientID != clientID {
					http.Error(w, "Unauthorized: Client ID mismatch", http.StatusUnauthorized)
					log.Printf("Authentication failed: Client ID mismatch (key: %s, got: %s, expected: %s) from %s",
						apiKey, reading.ClientID, clientID, r.RemoteAddr)
					return
				}
				// Rewind the body for the actual handler
				r.Body.Close()
				jsonData, _ := json.Marshal(reading)
				r.Body = &readCloser{Reader: strings.NewReader(string(jsonData))}
			}
		}

		// API key is valid
		next.ServeHTTP(w, r)
	})
}

// readCloser is a helper type that implements io.ReadCloser for request body rewinding
type readCloser struct {
	io.Reader
}

func (rc *readCloser) Close() error {
	return nil
}

// handlers for HTTP endpoints

func (s *Server) handleReadings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Add a new reading
		var reading Reading
		if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		s.addReading(reading)
		w.WriteHeader(http.StatusCreated)

	case "GET":
		// Get readings for a specific device
		deviceAddr := r.URL.Query().Get("device")
		if deviceAddr == "" {
			http.Error(w, "Missing device parameter", http.StatusBadRequest)
			return
		}
		readings := s.getDeviceReadings(deviceAddr)
		json.NewEncoder(w).Encode(readings)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	devices := s.getDevices()
	json.NewEncoder(w).Encode(devices)
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clients := s.getClients()
	json.NewEncoder(w).Encode(clients)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceAddr := r.URL.Query().Get("device")
	if deviceAddr == "" {
		http.Error(w, "Missing device parameter", http.StatusBadRequest)
		return
	}

	stats := s.getDeviceStats(deviceAddr)
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleDashboardData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Prepare dashboard data
	dashboardData := struct {
		Devices         []*DeviceStatus      `json:"devices"`
		Clients         []*ClientStatus      `json:"clients"`
		ActiveClients   int                  `json:"active_clients"`
		TotalReadings   int                  `json:"total_readings"`
		RecentReadings  map[string][]Reading `json:"recent_readings"`
		ServerStartTime time.Time            `json:"server_start_time"`
	}{
		Devices:         make([]*DeviceStatus, 0, len(s.devices)),
		Clients:         make([]*ClientStatus, 0, len(s.clients)),
		RecentReadings:  make(map[string][]Reading),
		ServerStartTime: time.Now().Add(-time.Since(time.Now())), // Get server start time
	}

	// Add devices
	for _, device := range s.devices {
		dashboardData.Devices = append(dashboardData.Devices, device)
	}

	// Add clients and count active ones
	totalReadings := 0
	for _, client := range s.clients {
		dashboardData.Clients = append(dashboardData.Clients, client)
		if client.IsActive {
			dashboardData.ActiveClients++
		}
		totalReadings += client.ReadingCount
	}
	dashboardData.TotalReadings = totalReadings

	// Add recent readings (last 10 for each device)
	for addr, readings := range s.readings {
		if len(readings) > 0 {
			end := len(readings)
			start := end - 10
			if start < 0 {
				start = 0
			}
			dashboardData.RecentReadings[addr] = readings[start:end]
		}
	}

	json.NewEncoder(w).Encode(dashboardData)
}

// handleHealthCheck handles health check requests
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// API key management endpoints
func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	// This endpoint requires admin API key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != s.auth.AdminKey {
		http.Error(w, "Unauthorized: Admin API key required", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case "GET":
		// List all API keys (except admin key)
		keys := make(map[string]string)
		for k, v := range s.auth.APIKeys {
			keys[k] = v
		}
		json.NewEncoder(w).Encode(keys)

	case "POST":
		// Create new API key
		var keyData struct {
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&keyData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if keyData.ClientID == "" {
			http.Error(w, "Client ID is required", http.StatusBadRequest)
			return
		}

		// Generate a new API key
		newKey := generateAPIKey()

		s.mu.Lock()
		s.auth.APIKeys[newKey] = keyData.ClientID
		s.mu.Unlock()

		// Save auth data if persistence is enabled
		if s.config.PersistenceEnabled {
			s.saveData()
		}

		// Return the new key
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"api_key":   newKey,
			"client_id": keyData.ClientID,
		})

	case "DELETE":
		// Delete API key
		apiKeyToDelete := r.URL.Query().Get("key")
		if apiKeyToDelete == "" {
			http.Error(w, "Missing key parameter", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		if _, exists := s.auth.APIKeys[apiKeyToDelete]; exists {
			delete(s.auth.APIKeys, apiKeyToDelete)
			s.mu.Unlock()

			// Save auth data if persistence is enabled
			if s.config.PersistenceEnabled {
				s.saveData()
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("API key deleted"))
		} else {
			s.mu.Unlock()
			http.Error(w, "API key not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// generateAPIKey creates a new random API key
func generateAPIKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 32

	b := make([]byte, keyLength)
	for i := range b {
		b[i] = charset[uint8(os.Getpid()+int(time.Now().UnixNano()))%uint8(len(charset))]
		time.Sleep(1 * time.Nanosecond) // Ensure uniqueness
	}
	return string(b)
}

// handleStaticFiles serves the static files for the dashboard
func handleStaticFiles(dir string) http.Handler {
	return http.FileServer(http.Dir(dir))
}

func main() {
	// Parse command-line flags
	port := flag.Int("port", 8080, "server port")
	logFile := flag.String("log", "govee-server.log", "log file path")
	staticDir := flag.String("static", "./static", "static files directory")
	storageDir := flag.String("storage", "./data", "data storage directory")
	clientTimeout := flag.Duration("timeout", 5*time.Minute, "client inactivity timeout")
	readingsPerDevice := flag.Int("readings", 1000, "max readings to store per device")
	persistenceEnabled := flag.Bool("persist", true, "enable data persistence")
	saveInterval := flag.Duration("save-interval", 5*time.Minute, "interval for saving data")

	// Authentication flags
	enableAuth := flag.Bool("auth", true, "enable API key authentication")
	adminKey := flag.String("admin-key", "", "admin API key (generated if empty)")
	defaultKey := flag.String("default-key", "", "default API key for all clients (generated if empty)")
	allowDefaultKey := flag.Bool("allow-default", false, "allow the default API key to be used")
	flag.Parse()

	// Create authentication configuration
	auth := &AuthConfig{
		EnableAuth:      *enableAuth,
		APIKeys:         make(map[string]string),
		AllowDefaultKey: *allowDefaultKey,
	}

	// Generate admin key if not provided
	if auth.EnableAuth {
		if *adminKey == "" {
			auth.AdminKey = generateAPIKey()
			log.Printf("Generated admin API key: %s", auth.AdminKey)
		} else {
			auth.AdminKey = *adminKey
		}

		// Generate default key if allowed and not provided
		if auth.AllowDefaultKey {
			if *defaultKey == "" {
				auth.DefaultAPIKey = generateAPIKey()
				log.Printf("Generated default API key: %s", auth.DefaultAPIKey)
			} else {
				auth.DefaultAPIKey = *defaultKey
			}
		}
	}

	// Create server configuration
	config := &Config{
		Port:               *port,
		LogFile:            *logFile,
		ClientTimeout:      *clientTimeout,
		ReadingsPerDevice:  *readingsPerDevice,
		StorageDir:         *storageDir,
		PersistenceEnabled: *persistenceEnabled,
		SaveInterval:       *saveInterval,
	}

	// Create and initialize server
	server := NewServer(config, auth)

	// Load data from storage if enabled
	if config.PersistenceEnabled {
		server.loadData()
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Create auth middleware
	authMiddleware := server.authMiddleware

	// API endpoints
	mux.Handle("/readings", authMiddleware(http.HandlerFunc(server.handleReadings)))
	mux.Handle("/devices", authMiddleware(http.HandlerFunc(server.handleDevices)))
	mux.Handle("/clients", authMiddleware(http.HandlerFunc(server.handleClients)))
	mux.Handle("/stats", authMiddleware(http.HandlerFunc(server.handleStats)))
	mux.Handle("/dashboard/data", authMiddleware(http.HandlerFunc(server.handleDashboardData)))
	mux.Handle("/api/keys", authMiddleware(http.HandlerFunc(server.handleAPIKeys)))
	mux.Handle("/health", http.HandlerFunc(server.handleHealthCheck))

	// Serve static files for dashboard
	mux.Handle("/", handleStaticFiles(*staticDir))

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting Govee Server on port %d", config.Port)
		if auth.EnableAuth {
			log.Printf("Authentication is enabled. Admin key: %s", auth.AdminKey)
			if auth.AllowDefaultKey {
				log.Printf("Default API key: %s", auth.DefaultAPIKey)
			}
		} else {
			log.Printf("Authentication is disabled")
		}

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	log.Println("Shutting down server...")

	// Save data before shutting down
	if config.PersistenceEnabled {
		server.saveData()
	}

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server shutdown complete")
}
