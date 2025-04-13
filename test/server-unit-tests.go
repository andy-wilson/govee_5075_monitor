package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAddReading tests the addReading function
func TestAddReading(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "govee-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with basic config
	config := &Config{
		Port:               8080,
		LogFile:            "",
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  10, // Small value for testing
		StorageDir:         tempDir,
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth: false,
		APIKeys:    make(map[string]string),
	}

	storageConfig := &StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	storageManager := NewStorageManager(storageConfig)
	server := NewServer(config, auth, storageManager)

	// Test reading
	reading := Reading{
		DeviceName:   "TestDevice",
		DeviceAddr:   "AA:BB:CC:DD:EE:FF",
		TempC:        22.5,
		TempF:        72.5,
		Humidity:     45.0,
		Battery:      80,
		RSSI:         -70,
		Timestamp:    time.Now(),
		ClientID:     "test-client",
		AbsHumidity:  9.1,
		DewPointC:    10.2,
		SteamPressure: 12.3,
	}

	// Add the reading
	server.addReading(reading)

	// Verify the device was created
	server.mu.RLock()
	device, exists := server.devices[reading.DeviceAddr]
	server.mu.RUnlock()

	if !exists {
		t.Fatalf("Device not found after adding reading")
	}

	// Check device properties
	if device.DeviceName != reading.DeviceName {
		t.Errorf("Device name mismatch: got %s, want %s", device.DeviceName, reading.DeviceName)
	}

	if device.TempC != reading.TempC {
		t.Errorf("Temperature mismatch: got %f, want %f", device.TempC, reading.TempC)
	}

	if device.Humidity != reading.Humidity {
		t.Errorf("Humidity mismatch: got %f, want %f", device.Humidity, reading.Humidity)
	}

	// Verify the client was created
	server.mu.RLock()
	client, clientExists := server.clients[reading.ClientID]
	server.mu.RUnlock()

	if !clientExists {
		t.Fatalf("Client not found after adding reading")
	}

	if !client.IsActive {
		t.Errorf("Client should be active")
	}

	// Verify readings were stored
	server.mu.RLock()
	readings, readingsExist := server.readings[reading.DeviceAddr]
	server.mu.RUnlock()

	if !readingsExist {
		t.Fatalf("Readings not found for device")
	}

	if len(readings) != 1 {
		t.Errorf("Expected 1 reading, got %d", len(readings))
	}
}

// TestHandleReadingsPost tests the POST handler for readings
func TestHandleReadingsPost(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "govee-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with basic config
	config := &Config{
		Port:               8080,
		LogFile:            "",
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  10,
		StorageDir:         tempDir,
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth: false,
		APIKeys:    make(map[string]string),
	}

	storageConfig := &StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	storageManager := NewStorageManager(storageConfig)
	server := NewServer(config, auth, storageManager)

	// Create a test reading
	reading := Reading{
		DeviceName:   "TestDevice",
		DeviceAddr:   "AA:BB:CC:DD:EE:FF",
		TempC:        22.5,
		TempF:        72.5,
		Humidity:     45.0,
		Battery:      80,
		RSSI:         -70,
		Timestamp:    time.Now(),
		ClientID:     "test-client",
		AbsHumidity:  9.1,
		DewPointC:    10.2,
		SteamPressure: 12.3,
	}

	// Marshal reading to JSON
	jsonReading, err := json.Marshal(reading)
	if err != nil {
		t.Fatalf("Failed to marshal reading: %v", err)
	}

	// Create a request
	req, err := http.NewRequest("POST", "/readings", bytes.NewBuffer(jsonReading))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handler := http.HandlerFunc(server.handleReadings)
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Verify the device was added to the server
	server.mu.RLock()
	_, exists := server.devices[reading.DeviceAddr]
	server.mu.RUnlock()

	if !exists {
		t.Errorf("Device not added to server")
	}
}

// TestHandleReadingsGet tests the GET handler for readings
func TestHandleReadingsGet(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "govee-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with basic config
	config := &Config{
		Port:               8080,
		LogFile:            "",
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  10,
		StorageDir:         tempDir,
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth: false,
		APIKeys:    make(map[string]string),
	}

	storageConfig := &StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	storageManager := NewStorageManager(storageConfig)
	server := NewServer(config, auth, storageManager)

	// Add some test readings
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	deviceName := "TestDevice"
	clientID := "test-client"

	for i := 0; i < 5; i++ {
		reading := Reading{
			DeviceName:    deviceName,
			DeviceAddr:    deviceAddr,
			TempC:         20.0 + float64(i),
			TempF:         68.0 + float64(i)*1.8,
			Humidity:      40.0 + float64(i),
			Battery:       80 - i,
			RSSI:          -70,
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Hour),
			ClientID:      clientID,
			AbsHumidity:   8.0 + float64(i)*0.2,
			DewPointC:     9.0 + float64(i)*0.2,
			SteamPressure: 11.0 + float64(i)*0.2,
		}
		server.addReading(reading)
	}

	// Create a request to get readings for the device
	req, err := http.NewRequest("GET", fmt.Sprintf("/readings?device=%s", deviceAddr), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handler := http.HandlerFunc(server.handleReadings)
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Decode the response
	var retrievedReadings []Reading
	if err := json.NewDecoder(rr.Body).Decode(&retrievedReadings); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify the correct number of readings
	if len(retrievedReadings) != 5 {
		t.Errorf("Expected 5 readings, got %d", len(retrievedReadings))
	}

	// Verify readings are for the correct device
	for _, r := range retrievedReadings {
		if r.DeviceAddr != deviceAddr {
			t.Errorf("Reading has wrong device address: got %s, want %s", r.DeviceAddr, deviceAddr)
		}
		if r.DeviceName != deviceName {
			t.Errorf("Reading has wrong device name: got %s, want %s", r.DeviceName, deviceName)
		}
	}
}

// TestAuthMiddleware tests the authentication middleware
func TestAuthMiddleware(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "govee-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with auth enabled
	config := &Config{
		Port:               8080,
		LogFile:            "",
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  10,
		StorageDir:         tempDir,
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth:      true,
		APIKeys:         make(map[string]string),
		AdminKey:        "admin-key-123",
		DefaultAPIKey:   "default-key-456",
		AllowDefaultKey: true,
	}

	// Add a client-specific key
	auth.APIKeys["client-key-789"] = "test-client"

	storageConfig := &StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	storageManager := NewStorageManager(storageConfig)
	server := NewServer(config, auth, storageManager)

	// Test handler that always returns 200 OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware handler
	handler := server.authMiddleware(testHandler)

	// Test cases
	testCases := []struct {
		name       string
		apiKey     string
		clientID   string
		endpoint   string
		method     string
		wantStatus int
	}{
		{"No API key", "", "", "/devices", "GET", http.StatusUnauthorized},
		{"Admin key", "admin-key-123", "", "/devices", "GET", http.StatusOK},
		{"Default key", "default-key-456", "", "/devices", "GET", http.StatusOK},
		{"Client key", "client-key-789", "test-client", "/devices", "GET", http.StatusOK},
		{"Client key - wrong client ID", "client-key-789", "wrong-client", "/readings", "POST", http.StatusUnauthorized},
		{"Invalid key", "invalid-key", "", "/devices", "GET", http.StatusUnauthorized},
		{"Health endpoint - no auth", "", "", "/health", "GET", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			var body []byte

			if tc.method == "POST" && tc.endpoint == "/readings" && tc.clientID != "" {
				// Create a reading with the specified client ID
				reading := Reading{
					DeviceName: "TestDevice",
					DeviceAddr: "AA:BB:CC:DD:EE:FF",
					ClientID:   tc.clientID,
				}
				body, _ = json.Marshal(reading)
				req, _ = http.NewRequest(tc.method, tc.endpoint, bytes.NewBuffer(body))
			} else {
				req, _ = http.NewRequest(tc.method, tc.endpoint, nil)
			}

			// Add API key if present
			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.wantStatus {
				t.Errorf("Handler returned wrong status code: got %v want %v", status, tc.wantStatus)
			}
		})
	}
}

// TestStorageManager tests the storage manager component
func TestStorageManager(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "govee-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a storage config
	storageConfig := &StorageConfig{
		BaseDir:           tempDir,
		TimePartitioning:  true,
		PartitionInterval: 24 * time.Hour,
		CompressOldData:   false,
	}

	// Create a storage manager
	manager := NewStorageManager(storageConfig)

	// Create test readings
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	readings := []Reading{
		{
			DeviceName:    "TestDevice",
			DeviceAddr:    deviceAddr,
			TempC:         22.5,
			TempF:         72.5,
			Humidity:      45.0,
			Battery:       80,
			RSSI:          -70,
			Timestamp:     time.Now(),
			ClientID:      "test-client",
			AbsHumidity:   9.1,
			DewPointC:     10.2,
			SteamPressure: 12.3,
		},
		{
			DeviceName:    "TestDevice",
			DeviceAddr:    deviceAddr,
			TempC:         23.0,
			TempF:         73.4,
			Humidity:      46.0,
			Battery:       79,
			RSSI:          -71,
			Timestamp:     time.Now().Add(-1 * time.Hour),
			ClientID:      "test-client",
			AbsHumidity:   9.3,
			DewPointC:     10.5,
			SteamPressure: 12.5,
		},
	}

	// Save readings
	if err := manager.saveReadings(deviceAddr, readings); err != nil {
		t.Fatalf("Failed to save readings: %v", err)
	}

	// Load the readings back
	loaded, err := manager.loadReadings(deviceAddr, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Failed to load readings: %v", err)
	}

	// Verify loaded readings
	if len(loaded) != len(readings) {
		t.Errorf("Expected %d readings, got %d", len(readings), len(loaded))
	}

	// Check that the partition directory was created
	partitionDir := manager.getCurrentPartitionDir()
	info, err := os.Stat(partitionDir)
	if err != nil {
		t.Fatalf("Partition directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", partitionDir)
	}

	// Check that the device file exists
	deviceFile := filepath.Join(partitionDir, fmt.Sprintf("readings_%s.json", deviceAddr))
	_, err = os.Stat(deviceFile)
	if err != nil {
		t.Fatalf("Device file not created: %v", err)
	}
}

// TestDeviceStats tests the calculation of device statistics
func TestDeviceStats(t *testing.T) {
	// Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "govee-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with basic config
	config := &Config{
		Port:               8080,
		LogFile:            "",
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  100,
		StorageDir:         tempDir,
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth: false,
		APIKeys:    make(map[string]string),
	}

	storageConfig := &StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	storageManager := NewStorageManager(storageConfig)
	server := NewServer(config, auth, storageManager)

	// Add some test readings with known values
	deviceAddr := "AA:BB:CC:DD:EE:FF"
	deviceName := "TestDevice"
	clientID := "test-client"

	// Create readings with min/max values we can test against
	temps := []float64{20.0, 22.0, 24.0, 26.0, 28.0}
	humidities := []float64{40.0, 45.0, 50.0, 55.0, 60.0}
	
	for i, temp := range temps {
		humidity := humidities[i]
		reading := Reading{
			DeviceName:    deviceName,
			DeviceAddr:    deviceAddr,
			TempC:         temp,
			TempF:         temp*1.8 + 32.0,
			Humidity:      humidity,
			Battery:       80 - i,
			RSSI:          -70,
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Hour),
			ClientID:      clientID,
			AbsHumidity:   8.0 + float64(i)*0.5,
			DewPointC:     9.0 + float64(i)*0.4,
			SteamPressure: 11.0 + float64(i)*0.3,
		}
		server.addReading(reading)
	}

	// Get stats
	stats := server.getDeviceStats(deviceAddr)

	// Verify count
	if count, ok := stats["count"].(int); !ok || count != 5 {
		t.Errorf("Expected count of 5, got %v", stats["count"])
	}

	// Verify min/max/avg temperature
	if tempMin, ok := stats["temp_c_min"].(float64); !ok || tempMin != 20.0 {
		t.Errorf("Expected min temp of 20.0, got %v", stats["temp_c_min"])
	}

	if tempMax, ok := stats["temp_c_max"].(float64); !ok || tempMax != 28.0 {
		t.Errorf("Expected max temp of 28.0, got %v", stats["temp_c_max"])
	}

	if tempAvg, ok := stats["temp_c_avg"].(float64); !ok || tempAvg != 24.0 {
		t.Errorf("Expected avg temp of 24.0, got %v", stats["temp_c_avg"])
	}

	// Verify min/max/avg humidity
	if humMin, ok := stats["humidity_min"].(float64); !ok || humMin != 40.0 {
		t.Errorf("Expected min humidity of 40.0, got %v", stats["humidity_min"])
	}

	if humMax, ok := stats["humidity_max"].(float64); !ok || humMax != 60.0 {
		t.Errorf("Expected max humidity of 60.0, got %v", stats["humidity_max"])
	}

	if humAvg, ok := stats["humidity_avg"].(float64); !ok || humAvg != 50.0 {
		t.Errorf("Expected avg humidity of 50.0, got %v", stats["humidity_avg"])
	}
}
