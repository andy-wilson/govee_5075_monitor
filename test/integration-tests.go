package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	// These imports would be adjusted to match your actual project structure
	// In practice, you might need to refactor your code to support better testing
	// by moving shared types to a package that can be imported by tests
	"github.com/andy-wilson/govee_5075_monitor/client"
	"github.com/andy-wilson/govee_5075_monitor/server"
)

// TestClientServerIntegration tests that a client can successfully send
// data to a server and the server processes it correctly
func TestClientServerIntegration(t *testing.T) {
	// Create a temporary directory for the server data
	tempDir, err := os.MkdirTemp("", "govee-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create server config
	serverConfig := &server.Config{
		Port:               8081, // Different from default to avoid conflicts
		StorageDir:         tempDir,
		ReadingsPerDevice:  100,
		PersistenceEnabled: false,
		ClientTimeout:      5 * time.Minute,
	}

	// Create auth config
	authConfig := &server.AuthConfig{
		EnableAuth:      true,
		APIKeys:         make(map[string]string),
		AdminKey:        "test-admin-key",
		DefaultAPIKey:   "test-default-key",
		AllowDefaultKey: true,
	}

	// Add a client-specific API key
	clientID := "test-integration-client"
	clientAPIKey := "test-client-key"
	authConfig.APIKeys[clientAPIKey] = clientID

	// Create storage config
	storageConfig := &server.StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	// Create and start the server
	storageManager := server.NewStorageManager(storageConfig)
	serverInstance := server.NewServer(serverConfig, authConfig, storageManager)

	// Create a test HTTP server using the server's handlers
	mux := http.NewServeMux()
	mux.Handle("/readings", serverInstance.AuthMiddleware(http.HandlerFunc(serverInstance.HandleReadings)))
	mux.Handle("/devices", serverInstance.AuthMiddleware(http.HandlerFunc(serverInstance.HandleDevices)))
	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Create test reading data
	reading := client.Reading{
		DeviceName:    "IntegrationTestDevice",
		DeviceAddr:    "AA:BB:CC:DD:EE:FF",
		TempC:         22.5,
		TempF:         72.5,
		Humidity:      45.0,
		Battery:       80,
		RSSI:          -70,
		Timestamp:     time.Now(),
		ClientID:      clientID,
		AbsHumidity:   9.1,
		DewPointC:     10.2,
		SteamPressure: 12.3,
	}

	// Send reading to server
	if err := client.SendToServer(testServer.URL+"/readings", reading, clientAPIKey, false, ""); err != nil {
		t.Fatalf("Failed to send reading to server: %v", err)
	}

	// Verify that the server received and processed the reading
	req, err := http.NewRequest("GET", testServer.URL+"/devices", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", authConfig.AdminKey)

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get devices, status code: %d", resp.StatusCode)
	}

	// Decode the response
	var devices []server.DeviceStatus
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("Failed to decode devices response: %v", err)
	}

	// Check that our device is in the list
	found := false
	for _, device := range devices {
		if device.DeviceAddr == reading.DeviceAddr {
			found = true

			// Check device properties
			if device.DeviceName != reading.DeviceName {
				t.Errorf("Device name mismatch: got %s, want %s", device.DeviceName, reading.DeviceName)
			}

			if device.TempC != reading.TempC {
				t.Errorf("Temperature mismatch: got %.2f, want %.2f", device.TempC, reading.TempC)
			}

			if device.Humidity != reading.Humidity {
				t.Errorf("Humidity mismatch: got %.2f, want %.2f", device.Humidity, reading.Humidity)
			}

			if device.ClientID != reading.ClientID {
				t.Errorf("Client ID mismatch: got %s, want %s", device.ClientID, reading.ClientID)
			}

			break
		}
	}

	if !found {
		t.Errorf("Device not found in server response")
	}
}

// TestUnauthorizedAccess verifies that unauthorized requests are properly rejected
func TestUnauthorizedAccess(t *testing.T) {
	// Create a temporary directory for the server data
	tempDir, err := os.MkdirTemp("", "govee-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create server config
	serverConfig := &server.Config{
		Port:               8082, // Different from default to avoid conflicts
		StorageDir:         tempDir,
		ReadingsPerDevice:  100,
		PersistenceEnabled: false,
		ClientTimeout:      5 * time.Minute,
	}

	// Create auth config with authentication enabled
	authConfig := &server.AuthConfig{
		EnableAuth:      true,
		APIKeys:         make(map[string]string),
		AdminKey:        "test-admin-key",
		DefaultAPIKey:   "test-default-key",
		AllowDefaultKey: false, // Default key not allowed
	}

	// Create storage config
	storageConfig := &server.StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	// Create and start the server
	storageManager := server.NewStorageManager(storageConfig)
	serverInstance := server.NewServer(serverConfig, authConfig, storageManager)

	// Create a test HTTP server using the server's handlers
	mux := http.NewServeMux()
	mux.Handle("/readings", serverInstance.AuthMiddleware(http.HandlerFunc(serverInstance.HandleReadings)))
	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Create test reading data
	reading := client.Reading{
		DeviceName:    "IntegrationTestDevice",
		DeviceAddr:    "AA:BB:CC:DD:EE:FF",
		TempC:         22.5,
		TempF:         72.5,
		Humidity:      45.0,
		Battery:       80,
		RSSI:          -70,
		Timestamp:     time.Now(),
		ClientID:      "unauthorized-client",
		AbsHumidity:   9.1,
		DewPointC:     10.2,
		SteamPressure: 12.3,
	}

	// Marshal reading to JSON
	jsonReading, err := json.Marshal(reading)
	if err != nil {
		t.Fatalf("Failed to marshal reading: %v", err)
	}

	// Test cases for different authentication scenarios
	testCases := []struct {
		name       string
		apiKey     string
		wantStatus int
	}{
		{"No API key", "", http.StatusUnauthorized},
		{"Invalid API key", "invalid-key", http.StatusUnauthorized},
		{"Default API key (not allowed)", "test-default-key", http.StatusUnauthorized},
		{"Admin key (always allowed)", "test-admin-key", http.StatusCreated},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the request
			req, err := http.NewRequest("POST", testServer.URL+"/readings", bytes.NewBuffer(jsonReading))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Add API key if present
			if tc.apiKey != "" {
				req.Header.Set("X-API-Key", tc.apiKey)
			}

			// Send the request
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("Wrong status code: got %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// TestDataPersistence tests that data is properly saved and loaded
func TestDataPersistence(t *testing.T) {
	// Create a temporary directory for the server data
	tempDir, err := os.MkdirTemp("", "govee-persistence-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create server config with persistence enabled
	serverConfig := &server.Config{
		Port:               8083,
		StorageDir:         tempDir,
		ReadingsPerDevice:  100,
		PersistenceEnabled: true,
		SaveInterval:       1 * time.Second, // Short interval for testing
		ClientTimeout:      5 * time.Minute,
	}

	// Create auth config
	authConfig := &server.AuthConfig{
		EnableAuth: false, // Disable auth for simplicity
		APIKeys:    make(map[string]string),
	}

	// Create storage config
	storageConfig := &server.StorageConfig{
		BaseDir:          tempDir,
		TimePartitioning: false,
	}

	// Create the first server instance
	storageManager := server.NewStorageManager(storageConfig)
	serverInstance := server.NewServer(serverConfig, authConfig, storageManager)

	// Create a test HTTP server
	mux := http.NewServeMux()
	mux.Handle("/readings", http.HandlerFunc(serverInstance.HandleReadings))
	mux.Handle("/devices", http.HandlerFunc(serverInstance.HandleDevices))
	testServer := httptest.NewServer(mux)

	// Add some test data
	readings := []client.Reading{
		{
			DeviceName:    "PersistenceTestDevice1",
			DeviceAddr:    "AA:BB:CC:DD:EE:01",
			TempC:         21.5,
			TempF:         70.7,
			Humidity:      45.0,
			Battery:       80,
			RSSI:          -70,
			Timestamp:     time.Now(),
			ClientID:      "test-client-1",
			AbsHumidity:   9.1,
			DewPointC:     10.2,
			SteamPressure: 12.3,
		},
		{
			DeviceName:    "PersistenceTestDevice2",
			DeviceAddr:    "AA:BB:CC:DD:EE:02",
			TempC:         22.5,
			TempF:         72.5,
			Humidity:      50.0,
			Battery:       85,
			RSSI:          -65,
			Timestamp:     time.Now(),
			ClientID:      "test-client-2",
			AbsHumidity:   10.1,
			DewPointC:     11.5,
			SteamPressure: 13.4,
		},
	}

	// Send readings to server
	for _, reading := range readings {
		jsonReading, _ := json.Marshal(reading)
		req, _ := http.NewRequest("POST", testServer.URL+"/readings", bytes.NewBuffer(jsonReading))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to send reading: %v", err)
		}
		resp.Body.Close()
	}

	// Force data save
	serverInstance.SaveData()

	// Close the first server
	testServer.Close()

	// Create a second server instance that should load the saved data
	storageManager2 := server.NewStorageManager(storageConfig)
	serverInstance2 := server.NewServer(serverConfig, authConfig, storageManager2)

	// Load previously saved data
	serverInstance2.LoadData()

	// Create a new test server with the second server instance
	mux2 := http.NewServeMux()
	mux2.Handle("/devices", http.HandlerFunc(serverInstance2.HandleDevices))
	testServer2 := httptest.NewServer(mux2)
	defer testServer2.Close()

	// Get devices from the second server
	resp, err := http.Get(testServer2.URL + "/devices")
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}
	defer resp.Body.Close()

	// Decode the response
	var devices []server.DeviceStatus
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("Failed to decode devices response: %v", err)
	}

	// Verify that all devices are present
	if len(devices) != len(readings) {
		t.Errorf("Device count mismatch: got %d, want %d", len(devices), len(readings))
	}

	// Check that device data was preserved correctly
	deviceMap := make(map[string]server.DeviceStatus)
	for _, device := range devices {
		deviceMap[device.DeviceAddr] = device
	}

	for _, reading := range readings {
		device, exists := deviceMap[reading.DeviceAddr]
		if !exists {
			t.Errorf("Device %s not found after reload", reading.DeviceAddr)
			continue
		}

		if device.DeviceName != reading.DeviceName {
			t.Errorf("Device name mismatch: got %s, want %s", device.DeviceName, reading.DeviceName)
		}

		if device.TempC != reading.TempC {
			t.Errorf("Temperature mismatch: got %.2f, want %.2f", device.TempC, reading.TempC)
		}

		if device.Humidity != reading.Humidity {
			t.Errorf("Humidity mismatch: got %.2f, want %.2f", device.Humidity, reading.Humidity)
		}
	}
}
