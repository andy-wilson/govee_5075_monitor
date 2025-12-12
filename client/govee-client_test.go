package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

// TestCToF tests Celsius to Fahrenheit conversion
func TestCToF(t *testing.T) {
	tests := []struct {
		name     string
		celsius  float64
		expected float64
	}{
		{"Freezing point", 0.0, 32.0},
		{"Boiling point", 100.0, 212.0},
		{"Room temperature", 20.0, 68.0},
		{"Body temperature", 37.0, 98.6},
		{"Negative temperature", -40.0, -40.0}, // -40°C = -40°F
		{"Typical indoor", 22.5, 72.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CToF(tt.celsius)
			if math.Abs(result-tt.expected) > 0.1 {
				t.Errorf("CToF(%v) = %v, expected %v", tt.celsius, result, tt.expected)
			}
		})
	}
}

// TestCalculateAbsoluteHumidity tests absolute humidity calculation
func TestCalculateAbsoluteHumidity(t *testing.T) {
	tests := []struct {
		name        string
		tempC       float64
		relHumidity float64
		minExpected float64
		maxExpected float64
	}{
		{"Room temp 50% RH", 20.0, 50.0, 8.0, 9.0},
		{"Room temp 100% RH", 20.0, 100.0, 17.0, 18.0},
		{"Hot humid", 30.0, 80.0, 24.0, 25.0},
		{"Cold dry", 0.0, 30.0, 1.0, 2.0},
		{"Typical indoor", 22.0, 45.0, 8.0, 9.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateAbsoluteHumidity(tt.tempC, tt.relHumidity)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("CalculateAbsoluteHumidity(%v, %v) = %v, expected between %v and %v",
					tt.tempC, tt.relHumidity, result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

// TestCalculateDewPoint tests dew point calculation
func TestCalculateDewPoint(t *testing.T) {
	tests := []struct {
		name        string
		tempC       float64
		relHumidity float64
		minExpected float64
		maxExpected float64
	}{
		{"Room temp 50% RH", 20.0, 50.0, 9.0, 10.0},
		{"Room temp 100% RH", 20.0, 100.0, 19.5, 20.5}, // At 100% RH, dew point = temp
		{"Hot humid", 30.0, 80.0, 25.0, 27.0},
		{"Cold dry", 0.0, 30.0, -18.0, -14.0},
		{"Typical indoor", 22.0, 45.0, 8.0, 11.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDewPoint(tt.tempC, tt.relHumidity)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("CalculateDewPoint(%v, %v) = %v, expected between %v and %v",
					tt.tempC, tt.relHumidity, result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

// TestCalculateSteamPressure tests steam pressure calculation
func TestCalculateSteamPressure(t *testing.T) {
	tests := []struct {
		name        string
		tempC       float64
		relHumidity float64
		minExpected float64
		maxExpected float64
	}{
		{"Room temp 50% RH", 20.0, 50.0, 11.0, 12.0},
		{"Room temp 100% RH", 20.0, 100.0, 23.0, 24.0},
		{"Hot humid", 30.0, 80.0, 33.0, 35.0},
		{"Cold dry", 0.0, 30.0, 1.5, 2.0},
		{"Typical indoor", 22.0, 45.0, 11.0, 13.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSteamPressure(tt.tempC, tt.relHumidity)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("CalculateSteamPressure(%v, %v) = %v, expected between %v and %v",
					tt.tempC, tt.relHumidity, result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

// TestCalculateDerivedValues tests the combined derived values calculation
func TestCalculateDerivedValues(t *testing.T) {
	tests := []struct {
		name        string
		tempC       float64
		humidity    float64
		expectValid bool
	}{
		{"Room temperature", 20.0, 50.0, true},
		{"Hot humid", 30.0, 80.0, true},
		{"Cold dry", 5.0, 30.0, true},
		{"Typical indoor", 22.0, 45.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absHumidity, dewPointC, dewPointF, steamPressure := CalculateDerivedValues(tt.tempC, tt.humidity)

			if tt.expectValid {
				// Check absolute humidity is positive and reasonable
				if absHumidity <= 0 || absHumidity > 50 {
					t.Errorf("Absolute humidity %v is out of reasonable range", absHumidity)
				}

				// Check dew point is less than or equal to temperature
				if dewPointC > tt.tempC+1 { // +1 for rounding
					t.Errorf("Dew point %v should be <= temperature %v", dewPointC, tt.tempC)
				}

				// Check dew point F is correct conversion
				expectedDewPointF := CToF(dewPointC)
				if math.Abs(dewPointF-expectedDewPointF) > 1 {
					t.Errorf("Dew point F %v doesn't match expected %v", dewPointF, expectedDewPointF)
				}

				// Check steam pressure is positive and reasonable
				if steamPressure <= 0 || steamPressure > 100 {
					t.Errorf("Steam pressure %v is out of reasonable range", steamPressure)
				}
			}
		})
	}
}

// TestNewScanner tests scanner creation
func TestNewScanner(t *testing.T) {
	scanner := NewScanner()
	if scanner == nil {
		t.Error("NewScanner returned nil")
	}
	// lastValues is initialized internally
}

// TestHasValueChanged tests the value change detection
func TestHasValueChanged(t *testing.T) {
	scanner := NewScanner()

	// First value should always be "changed"
	if !scanner.HasValueChanged("device1", 25) {
		t.Error("First value should be marked as changed")
	}

	// Same value should not be "changed"
	if scanner.HasValueChanged("device1", 25) {
		t.Error("Same value should not be marked as changed")
	}

	// Different value should be "changed"
	if !scanner.HasValueChanged("device1", 26) {
		t.Error("Different value should be marked as changed")
	}

	// Different device, same value should be "changed"
	if !scanner.HasValueChanged("device2", 25) {
		t.Error("Different device should be independent")
	}
}

// TestNewSendQueue tests send queue creation
func TestNewSendQueue(t *testing.T) {
	queue := NewSendQueue(
		1, // workers
		"http://localhost:8080",
		"test-api-key",
		false, // insecure skip verify
		"",    // CA cert file
		10*time.Second,
	)
	defer queue.Close()

	if queue == nil {
		t.Error("NewSendQueue returned nil")
	}
	if queue.serverURL != "http://localhost:8080" {
		t.Error("Server URL not set correctly")
	}
	if queue.apiKey != "test-api-key" {
		t.Error("API key not set correctly")
	}
}

// TestSendQueueEnqueue tests enqueuing readings
func TestSendQueueEnqueue(t *testing.T) {
	// Create a queue with a small buffer
	queue := NewSendQueue(
		1, // workers
		"http://localhost:9999", // Non-existent server
		"test-api-key",
		false,
		"",
		1*time.Second, // 1 second timeout
	)
	defer queue.Close()

	reading := Reading{
		DeviceName: "Test Device",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.0,
		TempF:      77.0,
		Humidity:   50.0,
	}

	// Enqueue should not block
	queue.Enqueue(reading)

	// Enqueue more to test buffer
	queue.Enqueue(reading)

	// Give worker time to process
	// The requests will fail since the server doesn't exist, but it shouldn't panic
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkCToF benchmarks temperature conversion
func BenchmarkCToF(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CToF(25.5)
	}
}

// BenchmarkCalculateAbsoluteHumidity benchmarks absolute humidity calculation
func BenchmarkCalculateAbsoluteHumidity(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateAbsoluteHumidity(25.0, 60.0)
	}
}

// BenchmarkCalculateDewPoint benchmarks dew point calculation
func BenchmarkCalculateDewPoint(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateDewPoint(25.0, 60.0)
	}
}

// BenchmarkCalculateSteamPressure benchmarks steam pressure calculation
func BenchmarkCalculateSteamPressure(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateSteamPressure(25.0, 60.0)
	}
}

// BenchmarkCalculateDerivedValues benchmarks all derived calculations
func BenchmarkCalculateDerivedValues(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateDerivedValues(25.0, 60.0)
	}
}

// TestSendQueueClose tests closing the send queue
func TestSendQueueClose(t *testing.T) {
	queue := NewSendQueue(
		2, // workers
		"http://localhost:9999",
		"test-api-key",
		false,
		"",
		1*time.Second,
	)

	// Close should not panic
	queue.Close()
	// Note: Double close would panic - the implementation doesn't handle it
}

// TestSendQueueEnqueueNoBlock tests that enqueue doesn't block
func TestSendQueueEnqueueNoBlock(t *testing.T) {
	queue := NewSendQueue(
		1,
		"http://localhost:9999",
		"test-api-key",
		false,
		"",
		10*time.Millisecond, // Very short timeout
	)

	// Enqueue should not block - test that channel is not full
	done := make(chan bool)
	go func() {
		queue.Enqueue(Reading{
			DeviceName: "Test Device",
			DeviceAddr: "AA:BB:CC:DD:EE:FF",
			TempC:      25.0,
		})
		done <- true
	}()

	select {
	case <-done:
		// Enqueue completed without blocking
	case <-time.After(100 * time.Millisecond):
		t.Error("Enqueue blocked too long")
	}

	queue.Close()
}

// TestHasValueChangedEdgeCases tests edge cases for value change detection
func TestHasValueChangedEdgeCases(t *testing.T) {
	scanner := NewScanner()

	// Test with negative values
	if !scanner.HasValueChanged("device-neg", -10) {
		t.Error("First negative value should be marked as changed")
	}

	if scanner.HasValueChanged("device-neg", -10) {
		t.Error("Same negative value should not be changed")
	}

	// Test with zero
	if !scanner.HasValueChanged("device-zero", 0) {
		t.Error("First zero value should be marked as changed")
	}

	// Test with large values
	if !scanner.HasValueChanged("device-large", 999999) {
		t.Error("First large value should be marked as changed")
	}
}

// TestCalculationsEdgeCases tests edge cases for environmental calculations
func TestCalculationsEdgeCases(t *testing.T) {
	// Test with extreme low temperature
	ah := CalculateAbsoluteHumidity(-10.0, 80.0)
	if ah <= 0 {
		t.Errorf("Absolute humidity should be positive at -10C, got %f", ah)
	}

	// Test with extreme high temperature
	ah = CalculateAbsoluteHumidity(40.0, 80.0)
	if ah <= 0 {
		t.Errorf("Absolute humidity should be positive at 40C, got %f", ah)
	}

	// Test dew point at 0% humidity (edge case)
	dp := CalculateDewPoint(25.0, 1.0) // Very low humidity
	if dp > 25.0 {
		t.Errorf("Dew point should be less than temp at low humidity, got %f", dp)
	}

	// Test steam pressure at high humidity
	sp := CalculateSteamPressure(25.0, 95.0)
	if sp <= 0 {
		t.Errorf("Steam pressure should be positive, got %f", sp)
	}
}

// TestDerivedValuesConsistency tests consistency between individual and combined calculations
func TestDerivedValuesConsistency(t *testing.T) {
	tempC := 25.0
	humidity := 60.0

	// Calculate individually
	absHumidity := CalculateAbsoluteHumidity(tempC, humidity)
	dewPointC := CalculateDewPoint(tempC, humidity)
	steamPressure := CalculateSteamPressure(tempC, humidity)

	// Calculate using combined function
	combinedAH, combinedDPC, combinedDPF, combinedSP := CalculateDerivedValues(tempC, humidity)

	// Compare results
	if math.Abs(absHumidity-combinedAH) > 0.01 {
		t.Errorf("Absolute humidity mismatch: individual=%f, combined=%f", absHumidity, combinedAH)
	}

	if math.Abs(dewPointC-combinedDPC) > 0.01 {
		t.Errorf("Dew point C mismatch: individual=%f, combined=%f", dewPointC, combinedDPC)
	}

	if math.Abs(steamPressure-combinedSP) > 0.01 {
		t.Errorf("Steam pressure mismatch: individual=%f, combined=%f", steamPressure, combinedSP)
	}

	// Check dew point F conversion
	expectedDPF := CToF(combinedDPC)
	if math.Abs(expectedDPF-combinedDPF) > 0.1 {
		t.Errorf("Dew point F mismatch: expected=%f, got=%f", expectedDPF, combinedDPF)
	}
}

// TestCToFPrecision tests temperature conversion precision
func TestCToFPrecision(t *testing.T) {
	tests := []struct {
		celsius  float64
		expected float64
	}{
		{0, 32},
		{100, 212},
		{-40, -40},
		{37, 98.6},
	}

	for _, tt := range tests {
		result := CToF(tt.celsius)
		if math.Abs(result-tt.expected) > 0.1 {
			t.Errorf("CToF(%v) = %v, expected %v", tt.celsius, result, tt.expected)
		}
	}
}

// BenchmarkNewScanner benchmarks scanner creation
func BenchmarkNewScanner(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewScanner()
	}
}

// BenchmarkHasValueChanged benchmarks value change detection
func BenchmarkHasValueChanged(b *testing.B) {
	scanner := NewScanner()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.HasValueChanged("device", i%100)
	}
}

// TestReadingStruct tests Reading struct serialization
func TestReadingStruct(t *testing.T) {
	now := time.Now()
	reading := Reading{
		DeviceName:     "Test Sensor",
		DeviceAddr:     "AA:BB:CC:DD:EE:FF",
		TempC:          25.5,
		TempF:          77.9,
		TempOffset:     0.5,
		Humidity:       60.0,
		HumidityOffset: 1.0,
		AbsHumidity:    13.8,
		DewPointC:      12.5,
		DewPointF:      54.5,
		SteamPressure:  19.1,
		Battery:        85,
		RSSI:           -67,
		Timestamp:      now,
		ClientID:       "test-client",
	}

	// Test that all fields are accessible
	if reading.DeviceName != "Test Sensor" {
		t.Error("DeviceName not set correctly")
	}
	if reading.TempC != 25.5 {
		t.Error("TempC not set correctly")
	}
	if reading.Battery != 85 {
		t.Error("Battery not set correctly")
	}
}

// TestGetDefaultClientID tests default client ID generation
func TestGetDefaultClientID(t *testing.T) {
	clientID := getDefaultClientID()

	// Should start with "client-"
	if len(clientID) < 8 || clientID[:7] != "client-" {
		t.Errorf("Expected client ID to start with 'client-', got %s", clientID)
	}
}

// TestSendToServerInvalidURL tests sendToServer with invalid URL
func TestSendToServerInvalidURL(t *testing.T) {
	reading := Reading{
		DeviceName: "Test",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.0,
		TempF:      77.0,
		Humidity:   50.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test",
	}

	err := sendToServer("http://invalid-server-name-999.example:9999", reading, "test-key", false, "", 1*time.Second)
	if err == nil {
		t.Error("Expected error for invalid server URL")
	}
}

// TestSendToServerInsecure tests sendToServer with insecure skip verify
func TestSendToServerInsecure(t *testing.T) {
	reading := Reading{
		DeviceName: "Test",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.0,
		TempF:      77.0,
		Humidity:   50.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test",
	}

	// This will fail (server doesn't exist) but test insecure path
	err := sendToServer("https://localhost:9999", reading, "test-key", true, "", 1*time.Second)
	// Error is expected (server doesn't exist)
	if err == nil {
		t.Log("Server unexpectedly responded")
	}
}

// TestSendToServerWithCACertNotExist tests sendToServer with non-existent CA cert
func TestSendToServerWithCACertNotExist(t *testing.T) {
	reading := Reading{
		DeviceName: "Test",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.0,
		TempF:      77.0,
		Humidity:   50.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test",
	}

	err := sendToServer("https://localhost:9999", reading, "test-key", false, "/nonexistent/ca.crt", 1*time.Second)
	if err == nil {
		t.Error("Expected error for non-existent CA cert")
	}
}

// TestSendToServerWithInvalidCACert tests sendToServer with invalid CA cert content
func TestSendToServerWithInvalidCACert(t *testing.T) {
	// Create temp file with invalid cert
	tmpFile, err := os.CreateTemp("", "invalid-cert-*.crt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("not a valid certificate")
	tmpFile.Close()

	reading := Reading{
		DeviceName: "Test",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.0,
		TempF:      77.0,
		Humidity:   50.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test",
	}

	err = sendToServer("https://localhost:9999", reading, "test-key", false, tmpFile.Name(), 1*time.Second)
	if err == nil {
		t.Error("Expected error for invalid CA cert")
	}
}

// TestPrintDeviceText tests printDeviceText doesn't panic
func TestPrintDeviceText(t *testing.T) {
	device := &GoveeDevice{
		Name:         "Test Device",
		Address:      "AA:BB:CC:DD:EE:FF",
		TempC:        25.0,
		TempF:        77.0,
		Humidity:     50.0,
		DewPointC:    12.0,
		AbsHumidity:  10.0,
		SteamPressure: 15.0,
		Battery:      85,
		RSSI:         -60,
		LastUpdate:   time.Now(),
	}

	// This should not panic
	printDeviceText(device)
}

// TestReadingJSON tests Reading JSON serialization
func TestReadingJSON(t *testing.T) {
	now := time.Now()
	reading := Reading{
		DeviceName: "Test Sensor",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.5,
		TempF:      77.9,
		Humidity:   60.0,
		Battery:    85,
		Timestamp:  now,
		ClientID:   "test-client",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(reading)
	if err != nil {
		t.Fatalf("Failed to marshal reading: %v", err)
	}

	// Test JSON unmarshaling
	var decoded Reading
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal reading: %v", err)
	}

	if decoded.DeviceName != reading.DeviceName {
		t.Error("DeviceName mismatch after JSON roundtrip")
	}
	if decoded.TempC != reading.TempC {
		t.Error("TempC mismatch after JSON roundtrip")
	}
}

// TestGoveeDeviceStruct tests GoveeDevice struct
func TestGoveeDeviceStruct(t *testing.T) {
	device := GoveeDevice{
		Name:           "Test Device",
		Address:        "AA:BB:CC:DD:EE:FF",
		TempC:          25.0,
		TempF:          77.0,
		TempOffset:     0.5,
		Humidity:       50.0,
		HumidityOffset: 1.0,
		DewPointC:      12.0,
		DewPointF:      53.6,
		AbsHumidity:    10.0,
		SteamPressure:  15.0,
		Battery:        85,
		RSSI:           -60,
		LastUpdate:     time.Now(),
	}

	if device.Name != "Test Device" {
		t.Error("Name not set correctly")
	}
	if device.TempC != 25.0 {
		t.Error("TempC not set correctly")
	}
}

// TestScannerConcurrency tests scanner value change detection with concurrent access
func TestScannerConcurrency(t *testing.T) {
	scanner := NewScanner()
	done := make(chan bool)

	// Run concurrent value changes
	for i := 0; i < 10; i++ {
		go func(idx int) {
			deviceID := "device" + string(rune('0'+idx))
			for j := 0; j < 100; j++ {
				scanner.HasValueChanged(deviceID, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestCalculateDerivedValuesRange tests derived value calculations across different ranges
func TestCalculateDerivedValuesRange(t *testing.T) {
	// Test various temperature and humidity combinations
	testCases := []struct {
		tempC    float64
		humidity float64
	}{
		{-20.0, 30.0}, // Cold, low humidity
		{0.0, 50.0},   // Freezing point
		{25.0, 60.0},  // Room temperature
		{35.0, 80.0},  // Hot and humid
		{45.0, 20.0},  // Very hot, dry
	}

	for _, tc := range testCases {
		absHum, dewC, dewF, steamP := CalculateDerivedValues(tc.tempC, tc.humidity)

		// All values should be finite and reasonable
		if math.IsNaN(absHum) || math.IsInf(absHum, 0) {
			t.Errorf("Invalid absHumidity for temp=%.1f, hum=%.1f", tc.tempC, tc.humidity)
		}
		if math.IsNaN(dewC) || math.IsInf(dewC, 0) {
			t.Errorf("Invalid dewPointC for temp=%.1f, hum=%.1f", tc.tempC, tc.humidity)
		}
		if math.IsNaN(dewF) || math.IsInf(dewF, 0) {
			t.Errorf("Invalid dewPointF for temp=%.1f, hum=%.1f", tc.tempC, tc.humidity)
		}
		if math.IsNaN(steamP) || math.IsInf(steamP, 0) {
			t.Errorf("Invalid steamPressure for temp=%.1f, hum=%.1f", tc.tempC, tc.humidity)
		}
	}
}
