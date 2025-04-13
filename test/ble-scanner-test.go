package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/andy-wilson/govee_5075_monitor/mock" // Adjust import path
	"github.com/go-ble/ble"
)

// TestBLEScanningWithMock tests the BLE scanning functionality using a mock device
func TestBLEScanningWithMock(t *testing.T) {
	// Skip if testing non-mocked behavior
	if os.Getenv("USE_REAL_BLE") != "" {
		t.Skip("Skipping mock test when USE_REAL_BLE is set")
	}

	// Get standard test devices
	mockDevices := mock.GetStandardTestDevices()

	// Create a mock device
	mockDevice := mock.NewMockDevice(mockDevices)

	// Replace the default BLE device with our mock
	ble.SetDefaultDevice(mockDevice)

	// Create a map to store discovered devices
	discoveredDevices := make(map[string]*GoveeDevice)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Scan for devices
	err := ble.Scan(ctx, true, func(a ble.Advertisement) {
		// Process advertisement
		addr := a.Addr().String()
		name := a.LocalName()
		rssi := a.RSSI()

		// Check if this might be a Govee device by name
		if len(name) >= 7 && name[:7] == "GVH5075" {
			// Get the manufacturer data
			mfrData := a.ManufacturerData()

			// Process Govee data if found
			if len(mfrData) > 6 {
				// Check if this is a valid Govee format (starting with 88EC)
				if len(mfrData) >= 2 && mfrData[0] == 0x88 && mfrData[1] == 0xEC {
					// Make sure we have enough bytes
					if len(mfrData) >= 7 {
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

						// Store device information
						discoveredDevices[addr] = &GoveeDevice{
							Address:    addr,
							Name:       name,
							RSSI:       rssi,
							TempC:      tempC,
							Humidity:   humidity,
							Battery:    battery,
							LastUpdate: time.Now(),
						}
					}
				}
			}
		}
	}, nil)

	// Check for scan errors
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Scan error: %v", err)
	}

	// Verify that we discovered all the expected devices
	if len(discoveredDevices) != len(mockDevices) {
		t.Errorf("Expected to discover %d devices, got %d", len(mockDevices), len(discoveredDevices))
	}

	// Check each discovered device
	for _, mockDevice := range mockDevices {
		addr := mockDevice.Address
		device, found := discoveredDevices[addr]

		if !found {
			t.Errorf("Device %s (%s) not discovered", addr, mockDevice.Name)
			continue
		}

		// Check device properties
		if device.Name != mockDevice.Name {
			t.Errorf("Device name mismatch for %s: got %s, want %s", addr, device.Name, mockDevice.Name)
		}

		if !closeEnough(device.TempC, mockDevice.TempC, 0.1) {
			t.Errorf("Temperature mismatch for %s: got %.2f, want %.2f", addr, device.TempC, mockDevice.TempC)
		}

		if !closeEnough(device.Humidity, mockDevice.Humidity, 0.1) {
			t.Errorf("Humidity mismatch for %s: got %.2f, want %.2f", addr, device.Humidity, mockDevice.Humidity)
		}

		if device.Battery != mockDevice.Battery {
			t.Errorf("Battery mismatch for %s: got %d, want %d", addr, device.Battery, mockDevice.Battery)
		}

		if device.RSSI != mockDevice.RSSI {
			t.Errorf("RSSI mismatch for %s: got %d, want %d", addr, device.RSSI, mockDevice.RSSI)
		}
	}
}

// TestRawDataDecoding tests the decoding of raw manufacturer data
func TestRawDataDecoding(t *testing.T) {
	// Get standard test cases
	testCases := mock.GetStandardTestCases()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Convert hex string to bytes
			rawData, err := mock.HexStringToBytes(tc.RawData)
			if err != nil {
				t.Fatalf("Failed to decode hex data: %v", err)
			}

			// Check if this is a valid Govee format
			if len(rawData) < 2 || rawData[0] != 0x88 || rawData[1] != 0xEC {
				t.Fatalf("Invalid Govee data format: %X", rawData[:2])
			}

			// Ensure we have enough bytes for temperature/humidity and battery
			if len(rawData) < 7 {
				t.Fatalf("Data too short: %d bytes", len(rawData))
			}

			// Convert bytes 3-5 to an integer in big endian
			values := uint32(0)
			for i := 0; i < 3; i++ {
				values = (values << 8) | uint32(rawData[i+3])
			}

			// Calculate temperature and humidity
			tempC := float64(values) / 10000.0
			humidity := float64(values%1000) / 10.0

			// Battery is directly from byte 6
			battery := int(rawData[6])

			// Check if the decoded values match expectations
			if !closeEnough(tempC, tc.TempC, 0.01) {
				t.Errorf("Temperature mismatch: got %.2f, want %.2f", tempC, tc.TempC)
			}

			if !closeEnough(humidity, tc.Humidity, 0.1) {
				t.Errorf("Humidity mismatch: got %.1f, want %.1f", humidity, tc.Humidity)
			}

			if battery != tc.Battery {
				t.Errorf("Battery mismatch: got %d, want %d", battery, tc.Battery)
			}
		})
	}
}

// closeEnough returns true if the values are within epsilon of each other
func closeEnough(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// GoveeDevice struct for testing
type GoveeDevice struct {
	Address    string
	Name       string
	RSSI       int
	TempC      float64
	Humidity   float64
	Battery    int
	LastUpdate time.Time
}
