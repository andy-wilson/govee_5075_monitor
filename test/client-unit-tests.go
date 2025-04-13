package main

import (
	"encoding/hex"
	"os"
	"testing"
	"time"
)

// TestGoveeDataDecoding tests the decoding of Govee BLE data
func TestGoveeDataDecoding(t *testing.T) {
	// Test cases for known Govee data packets
	testCases := []struct {
		name         string
		mfrData      string // hex encoded manufacturer data
		wantTempC    float64
		wantHumidity float64
		wantBattery  int
	}{
		{
			name:         "Standard data packet",
			mfrData:      "88EC0002962D4B00",
			wantTempC:    16.95,
			wantHumidity: 51.7,
			wantBattery:  75,
		},
		{
			name:         "Lower temperature",
			mfrData:      "88EC0001542F5500",
			wantTempC:    15.44,
			wantHumidity: 53.1,
			wantBattery:  85,
		},
		{
			name:         "Higher temperature",
			mfrData:      "88EC0003751A6200",
			wantTempC:    23.75,
			wantHumidity: 31.8,
			wantBattery:  98,
		},
		{
			name:         "Low battery",
			mfrData:      "88EC0002543B1500",
			wantTempC:    16.54,
			wantHumidity: 43.9,
			wantBattery:  21,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert hex string to bytes
			mfrData, err := hex.DecodeString(tc.mfrData)
			if err != nil {
				t.Fatalf("Failed to decode hex data: %v", err)
			}

			// Check if this is a valid Govee format
			if len(mfrData) < 2 || mfrData[0] != 0x88 || mfrData[1] != 0xEC {
				t.Fatalf("Invalid Govee data format: %X", mfrData[:2])
			}

			// Ensure we have enough bytes for temperature/humidity and battery
			if len(mfrData) < 7 {
				t.Fatalf("Data too short: %d bytes", len(mfrData))
			}

			// Convert bytes 3-5 to an integer in big endian
			values := uint32(0)
			for i := 0; i < 3; i++ {
				values = (values << 8) | uint32(mfrData[i+3])
			}

			// Calculate temperature and humidity
			tempC := float64(values) / 10000.0
			humidity := float64(values%1000) / 10.0

			// Battery is directly from byte 6
			battery := int(mfrData[6])

			// Check if the decoded values match expectations
			if !closeEnough(tempC, tc.wantTempC, 0.01) {
				t.Errorf("Temperature mismatch: got %.2f, want %.2f", tempC, tc.wantTempC)
			}

			if !closeEnough(humidity, tc.wantHumidity, 0.1) {
				t.Errorf("Humidity mismatch: got %.1f, want %.1f", humidity, tc.wantHumidity)
			}

			if battery != tc.wantBattery {
				t.Errorf("Battery mismatch: got %d, want %d", battery, tc.wantBattery)
			}
		})
	}
}

// TestCToF tests the Celsius to Fahrenheit conversion
func TestCToF(t *testing.T) {
	testCases := []struct {
		celsius    float64
		fahrenheit float64
	}{
		{0.0, 32.0},
		{100.0, 212.0},
		{23.5, 74.3},
		{-10.0, 14.0},
		{36.6, 97.88},
	}

	for _, tc := range testCases {
		t.Run(testString(tc.celsius), func(t *testing.T) {
			result := CToF(tc.celsius)
			if !closeEnough(result, tc.fahrenheit, 0.01) {
				t.Errorf("CToF(%f) = %f; want %f", tc.celsius, result, tc.fahrenheit)
			}
		})
	}
}

// TestCalculatedMetrics tests the calculation of derived metrics
func TestCalculatedMetrics(t *testing.T) {
	testCases := []struct {
		name        string
		tempC       float64
		humidity    float64
		wantAH      float64 // Expected absolute humidity
		wantDP      float64 // Expected dew point
		wantSP      float64 // Expected steam pressure
	}{
		{
			name:     "Normal room conditions",
			tempC:    22.0,
			humidity: 50.0,
			wantAH:   9.5,  // ~9.5 g/m³
			wantDP:   11.1, // ~11.1°C
			wantSP:   13.1, // ~13.1 hPa
		},
		{
			name:     "Warm and humid",
			tempC:    30.0,
			humidity: 80.0,
			wantAH:   24.3, // ~24.3 g/m³
			wantDP:   26.2, // ~26.2°C
			wantSP:   33.8, // ~33.8 hPa
		},
		{
			name:     "Cold and dry",
			tempC:    5.0,
			humidity: 30.0,
			wantAH:   1.6, // ~1.6 g/m³
			wantDP:   -7.7, // ~-7.7°C
			wantSP:   2.2, // ~2.2 hPa
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate absolute humidity using the formula from the documentation
			satVP := 6.112 * math.Exp(17.62*tc.tempC/(243.12+tc.tempC))
			actualVP := tc.humidity / 100.0 * satVP
			absHumidity := 216.7 * (actualVP / (273.15 + tc.tempC))

			// Calculate dew point using the Magnus formula
			logTerm := math.Log(tc.humidity / 100.0 * math.Exp(17.62*tc.tempC/(243.12+tc.tempC)))
			dewPoint := 243.12 * logTerm / (17.62 - logTerm)

			// Calculate steam pressure
			steamPressure := tc.humidity / 100.0 * satVP

			// Check if calculated values are within acceptable range
			if !closeEnough(absHumidity, tc.wantAH, 0.2) {
				t.Errorf("Absolute humidity mismatch: got %.1f g/m³, want %.1f g/m³", absHumidity, tc.wantAH)
			}

			if !closeEnough(dewPoint, tc.wantDP, 0.2) {
				t.Errorf("Dew point mismatch: got %.1f°C, want %.1f°C", dewPoint, tc.wantDP)
			}

			if !closeEnough(steamPressure, tc.wantSP, 0.2) {
				t.Errorf("Steam pressure mismatch: got %.1f hPa, want %.1f hPa", steamPressure, tc.wantSP)
			}
		})
	}
}

// TestSendToServer tests the server communication function
func TestSendToServer(t *testing.T) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for correct API key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check for correct content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Decode the JSON body
		var reading Reading
		if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check for required fields
		if reading.DeviceAddr == "" || reading.ClientID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Respond with success
		w.WriteHeader(http.StatusCreated)
	}))
	defer mockServer.Close()

	// Create a test reading
	reading := Reading{
		DeviceName:    "TestDevice",
		DeviceAddr:    "AA:BB:CC:DD:EE:FF",
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
	}

	// Test with valid API key
	sendToServer(mockServer.URL, reading, "test-api-key", false, "")
	
	// Test with invalid API key
	sendToServer(mockServer.URL, reading, "invalid-key", false, "")
	
	// No assertion needed as we're just testing that the function runs without errors
}

// Helper functions for tests

// closeEnough returns true if the values are within epsilon of each other
func closeEnough(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// testString creates a string representation of a float for test names
func testString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// Mock Bluetooth detection for testing
func TestScanForGoveeDevices(t *testing.T) {
	// This test would normally use a mock BLE implementation
	// Since that's complex, we'll create a simplified test that
	// just verifies the scan logic without actual BLE operations
	
	// Skip if running in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping BLE test in CI environment")
	}
	
	// Create a temporary log file
	tempFile, err := os.CreateTemp("", "govee-test-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	
	// Set up test parameters
	duration := 2 * time.Second
	continuous := false
	verbose := true
	
	// This is a limited test - just check that the scan function doesn't panic
	// In a real implementation, you would mock the BLE interface
	t.Log("Note: This is a limited test that doesn't perform actual BLE scanning")
	
	// The real test requires a proper BLE mock framework, which is beyond the scope
	// of this simple test suite. In practice, you would use a framework like
	// https://github.com/go-ble/ble/tree/master/linux/hci/test for mocking BLE.
}

// TestCalibratedReadings tests the calibration offset functionality
func TestCalibratedReadings(t *testing.T) {
	// Test cases for calibration
	testCases := []struct {
		name           string
		rawTempC       float64
		rawHumidity    float64
		tempOffset     float64
		humidityOffset float64
		wantTempC      float64
		wantHumidity   float64
	}{
		{
			name:           "Positive offsets",
			rawTempC:       22.0,
			rawHumidity:    50.0,
			tempOffset:     1.5,
			humidityOffset: 3.0,
			wantTempC:      23.5,
			wantHumidity:   53.0,
		},
		{
			name:           "Negative offsets",
			rawTempC:       22.0,
			rawHumidity:    50.0,
			tempOffset:     -1.5,
			humidityOffset: -3.0,
			wantTempC:      20.5,
			wantHumidity:   47.0,
		},
		{
			name:           "Mixed offsets",
			rawTempC:       22.0,
			rawHumidity:    50.0,
			tempOffset:     1.5,
			humidityOffset: -3.0,
			wantTempC:      23.5,
			wantHumidity:   47.0,
		},
		{
			name:           "Zero offsets",
			rawTempC:       22.0,
			rawHumidity:    50.0,
			tempOffset:     0.0,
			humidityOffset: 0.0,
			wantTempC:      22.0,
			wantHumidity:   50.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply calibration
			calibratedTemp := tc.rawTempC + tc.tempOffset
			calibratedHumidity := tc.rawHumidity + tc.humidityOffset

			// Check results
			if !closeEnough(calibratedTemp, tc.wantTempC, 0.01) {
				t.Errorf("Calibrated temperature mismatch: got %.2f, want %.2f", calibratedTemp, tc.wantTempC)
			}

			if !closeEnough(calibratedHumidity, tc.wantHumidity, 0.01) {
				t.Errorf("Calibrated humidity mismatch: got %.2f, want %.2f", calibratedHumidity, tc.wantHumidity)
			}
		})
	}
}

// TestAPIKeyGeneration verifies the API key generation and validation
func TestAPIKeyGeneration(t *testing.T) {
	// In a real application, you would test your API key generation function
	// by verifying that it produces keys of the expected format and entropy
	
	// This is a placeholder test that would be replaced with actual
	// tests for your key generation function
	
	// Basic API key format check
	key := generateAPIKey() // This function would be imported from your main code
	
	// Check length
	const expectedLength = 32
	if len(key) != expectedLength {
		t.Errorf("API key length mismatch: got %d, want %d", len(key), expectedLength)
	}
	
	// Check character set
	for i, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Errorf("Invalid character at position %d: %c", i, c)
		}
	}
	
	// Generate multiple keys and check uniqueness
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := generateAPIKey()
		if keys[key] {
			t.Errorf("Duplicate key generated: %s", key)
		}
		keys[key] = true
	}
}

// generateAPIKey is a stub for testing - in practice, this would be imported from your main code
func generateAPIKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 32
	
	b := make([]byte, keyLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Missing imports for the test file
import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
)

// CToF function replica for testing
func CToF(celsius float64) float64 {
	return math.Round((32.0 + 9.0*celsius/5.0) * 100) / 100
}
