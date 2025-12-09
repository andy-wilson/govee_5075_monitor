package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGenerateAPIKey tests API key generation
func TestGenerateAPIKey(t *testing.T) {
	keys := make(map[string]bool)

	// Generate 1000 keys and check for uniqueness
	for i := 0; i < 1000; i++ {
		key := generateAPIKey()

		if len(key) == 0 {
			t.Fatal("Generated empty API key")
		}

		if keys[key] {
			t.Fatalf("Duplicate key generated: %s", key)
		}

		keys[key] = true
	}

	t.Logf("Successfully generated %d unique API keys", len(keys))
}

// TestSanitizeDeviceAddr tests device address sanitization
func TestSanitizeDeviceAddr(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		{
			name:      "Valid MAC address with colons",
			input:     "AA:BB:CC:DD:EE:FF",
			expected:  "aabbccddeeff",
			wantError: false,
		},
		{
			name:      "Valid MAC address without colons",
			input:     "AABBCCDDEEFF",
			expected:  "aabbccddeeff",
			wantError: false,
		},
		{
			name:      "Path traversal attempt",
			input:     "../../../etc/passwd",
			expected:  "",
			wantError: true,
		},
		{
			name:      "Invalid characters",
			input:     "AA:BB:CC:DD:EE:ZZ",
			expected:  "",
			wantError: true,
		},
		{
			name:      "Empty string",
			input:     "",
			expected:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizeDeviceAddr(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

// TestSanitizeDeviceName tests device name sanitization
func TestSanitizeDeviceName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		{
			name:      "Valid device name",
			input:     "Living Room Sensor",
			expected:  "Living Room Sensor",
			wantError: false,
		},
		{
			name:      "Device name with numbers",
			input:     "Sensor-123",
			expected:  "Sensor-123",
			wantError: false,
		},
		{
			name:      "Device name with parentheses",
			input:     "Bedroom (Main)",
			expected:  "Bedroom (Main)",
			wantError: false,
		},
		{
			name:      "XSS attempt",
			input:     "<script>alert('xss')</script>",
			expected:  "",
			wantError: true,
		},
		{
			name:      "SQL injection attempt",
			input:     "'; DROP TABLE devices; --",
			expected:  "",
			wantError: true,
		},
		{
			name:      "Empty string",
			input:     "",
			expected:  "",
			wantError: true,
		},
		{
			name:      "Too long",
			input:     string(make([]byte, 101)),
			expected:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizeDeviceName(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

// TestValidateReading tests reading validation
func TestValidateReading(t *testing.T) {
	tests := []struct {
		name      string
		reading   Reading
		wantError bool
	}{
		{
			name: "Valid reading",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      25.5,
				Humidity:   60.0,
				Battery:    85,
				Timestamp:  time.Now(),
				ClientID:   "test-client",
			},
			wantError: false,
		},
		{
			name: "Temperature too high",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      150.0,
				Humidity:   60.0,
				Battery:    85,
				Timestamp:  time.Now(),
				ClientID:   "test-client",
			},
			wantError: true,
		},
		{
			name: "Temperature too low",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      -100.0,
				Humidity:   60.0,
				Battery:    85,
				Timestamp:  time.Now(),
				ClientID:   "test-client",
			},
			wantError: true,
		},
		{
			name: "Humidity out of range",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      25.0,
				Humidity:   150.0,
				Battery:    85,
				Timestamp:  time.Now(),
				ClientID:   "test-client",
			},
			wantError: true,
		},
		{
			name: "Battery out of range",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      25.0,
				Humidity:   60.0,
				Battery:    150,
				Timestamp:  time.Now(),
				ClientID:   "test-client",
			},
			wantError: true,
		},
		{
			name: "Timestamp in future",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      25.0,
				Humidity:   60.0,
				Battery:    85,
				Timestamp:  time.Now().Add(2 * time.Hour),
				ClientID:   "test-client",
			},
			wantError: true,
		},
		{
			name: "Timestamp too old",
			reading: Reading{
				DeviceName: "Test Sensor",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      25.0,
				Humidity:   60.0,
				Battery:    85,
				Timestamp:  time.Now().Add(-25 * time.Hour),
				ClientID:   "test-client",
			},
			wantError: true,
		},
		{
			name: "Invalid device name",
			reading: Reading{
				DeviceName: "<script>alert('xss')</script>",
				DeviceAddr: "AA:BB:CC:DD:EE:FF",
				TempC:      25.0,
				Humidity:   60.0,
				Battery:    85,
				Timestamp:  time.Now(),
				ClientID:   "test-client",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReading(&tt.reading)

			if tt.wantError {
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

// TestHealthCheckEndpoint tests the health check HTTP endpoint
func TestHealthCheckEndpoint(t *testing.T) {
	// Create test server
	config := &Config{
		Port:               8080,
		ClientTimeout:      5 * time.Minute,
		ReadingsPerDevice:  100,
		StorageDir:         "/tmp/test-storage",
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth: false,
	}

	storageConfig := &StorageConfig{
		BaseDir: "/tmp/test-storage",
	}
	storageManager := NewStorageManager(storageConfig)

	server := NewServer(config, auth, storageManager)
	defer server.logger.Close()

	// Create request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleHealthCheck(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var health HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	// Validate response
	if health.Status == "" {
		t.Error("Expected non-empty status")
	}

	if health.Version == "" {
		t.Error("Expected non-empty version")
	}

	if health.Goroutines == 0 {
		t.Error("Expected non-zero goroutine count")
	}

	if len(health.Checks) == 0 {
		t.Error("Expected health checks")
	}

	t.Logf("Health status: %s, Uptime: %s, Goroutines: %d",
		health.Status, health.Uptime, health.Goroutines)
}

// TestDashboardCache tests the dashboard caching mechanism
func TestDashboardCache(t *testing.T) {
	cache := &DashboardCache{
		ttl: 100 * time.Millisecond,
	}

	// Initially should be empty
	if data := cache.Get(); data != nil {
		t.Error("Expected nil from empty cache")
	}

	// Set data
	testData := &DashboardData{
		Devices:       make([]*DeviceStatus, 0),
		Clients:       make([]*ClientStatus, 0),
		ActiveClients: 5,
	}
	cache.Set(testData)

	// Should retrieve the same data
	if data := cache.Get(); data == nil {
		t.Error("Expected data from cache")
	} else if data.ActiveClients != 5 {
		t.Errorf("Expected ActiveClients=5, got %d", data.ActiveClients)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	if data := cache.Get(); data != nil {
		t.Error("Expected nil from expired cache")
	}
}

// TestRateLimiter tests the rate limiting functionality
func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter()

	// Get limiter for IP
	ipLimiter := limiter.GetLimiter("192.168.1.1")

	// Should allow initial requests
	allowed := 0
	for i := 0; i < 25; i++ {
		if ipLimiter.Allow() {
			allowed++
		}
	}

	// Should allow up to burst (20) requests
	if allowed < 15 {
		t.Errorf("Rate limiter too restrictive: only allowed %d/25 requests", allowed)
	}

	t.Logf("Rate limiter allowed %d/25 requests initially", allowed)

	// Different IP should have different limiter
	ipLimiter2 := limiter.GetLimiter("192.168.1.2")
	if !ipLimiter2.Allow() {
		t.Error("Expected different IP to have separate rate limit")
	}
}

// TestSecurityHeaders tests that security headers are set correctly
func TestSecurityHeaders(t *testing.T) {
	config := &Config{
		Port:               8080,
		ClientTimeout:      5 * time.Minute,
		StorageDir:         "/tmp/test-storage",
		PersistenceEnabled: false,
	}

	auth := &AuthConfig{
		EnableAuth: false,
	}

	storageConfig := &StorageConfig{
		BaseDir: "/tmp/test-storage",
	}
	storageManager := NewStorageManager(storageConfig)

	server := NewServer(config, auth, storageManager)
	defer server.logger.Close()

	// Create a test handler
	handler := server.securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check security headers
	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Content-Security-Policy": "",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "",
	}

	for header, expectedValue := range headers {
		actual := w.Header().Get(header)
		if expectedValue != "" && actual != expectedValue {
			t.Errorf("Expected %s=%q, got %q", header, expectedValue, actual)
		} else if expectedValue == "" && actual == "" {
			t.Errorf("Expected %s to be set, but it's empty", header)
		}
	}

	t.Log("All security headers properly set")
}

// BenchmarkGenerateAPIKey benchmarks API key generation
func BenchmarkGenerateAPIKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateAPIKey()
	}
}

// BenchmarkValidateReading benchmarks reading validation
func BenchmarkValidateReading(b *testing.B) {
	reading := Reading{
		DeviceName: "Test Sensor",
		DeviceAddr: "AA:BB:CC:DD:EE:FF",
		TempC:      25.5,
		Humidity:   60.0,
		Battery:    85,
		Timestamp:  time.Now(),
		ClientID:   "test-client",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateReading(&reading)
	}
}

// BenchmarkDashboardCache benchmarks cache performance
func BenchmarkDashboardCache(b *testing.B) {
	cache := &DashboardCache{
		ttl: 30 * time.Second,
	}

	testData := &DashboardData{
		Devices:       make([]*DeviceStatus, 0),
		Clients:       make([]*ClientStatus, 0),
		ActiveClients: 5,
	}
	cache.Set(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get()
	}
}
