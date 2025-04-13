package mock

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-ble/ble"
)

// This file creates a mock implementation of the BLE interface
// for testing purposes without requiring actual Bluetooth hardware

// MockDevice implements the ble.Device interface for testing
type MockDevice struct {
	Devices []MockAdvertisement
}

// NewMockDevice creates a new mock BLE device
func NewMockDevice(goveeDevices []GoveeMockDevice) *MockDevice {
	devices := make([]MockAdvertisement, len(goveeDevices))
	for i, gd := range goveeDevices {
		devices[i] = MockAdvertisement{
			name:   gd.Name,
			addr:   gd.Address,
			rssi:   gd.RSSI,
			mfrData: gd.RawData,
		}
	}
	return &MockDevice{Devices: devices}
}

// AddGoveeDevice adds a new Govee device to the mock
func (d *MockDevice) AddGoveeDevice(device GoveeMockDevice) {
	d.Devices = append(d.Devices, MockAdvertisement{
		name:   device.Name,
		addr:   device.Address,
		rssi:   device.RSSI,
		mfrData: device.RawData,
	})
}

// Scan implements the ble.Device.Scan method
func (d *MockDevice) Scan(ctx context.Context, allowDup bool, h ble.AdvHandler) error {
	// Simulate scanning time
	scanDone := make(chan struct{})
	
	go func() {
		// Process each mock device
		for _, adv := range d.Devices {
			// Check if context is done
			select {
			case <-ctx.Done():
				return
			default:
				// Call the handler with the mock advertisement
				h(adv)
				// Add a small delay to simulate device discovery
				time.Sleep(100 * time.Millisecond)
			}
		}
		scanDone <- struct{}{}
	}()
	
	// Wait for scan completion or context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-scanDone:
		return nil
	}
}

// Stop, Address methods are not needed for our test but required by interface
func (d *MockDevice) Stop() error { return nil }
func (d *MockDevice) Address() ble.Addr { return MockAddr{} }

// Dial, DialContext etc. are not needed for our tests
func (d *MockDevice) Dial(ctx context.Context, addr ble.Addr) (ble.Client, error) {
	return nil, fmt.Errorf("not implemented")
}
func (d *MockDevice) DialWithParams(ctx context.Context, addr ble.Addr, params ble.AdvFilter) (ble.Client, error) {
	return nil, fmt.Errorf("not implemented")
}
func (d *MockDevice) DialContext(ctx context.Context, adv ble.Advertisement) (ble.Client, error) {
	return nil, fmt.Errorf("not implemented")
}

// MockAdvertisement implements the ble.Advertisement interface
type MockAdvertisement struct {
	name     string
	addr     string
	rssi     int
	mfrData  []byte
}

// LocalName returns the device name
func (a MockAdvertisement) LocalName() string { return a.name }

// Addr returns the device address
func (a MockAdvertisement) Addr() ble.Addr { return MockAddr{addr: a.addr} }

// RSSI returns the signal strength
func (a MockAdvertisement) RSSI() int { return a.rssi }

// ManufacturerData returns the manufacturer data
func (a MockAdvertisement) ManufacturerData() []byte { return a.mfrData }

// ServiceData returns mock service data (empty for our tests)
func (a MockAdvertisement) ServiceData() []ble.ServiceData { return nil }

// Services returns mock services (empty for our tests)
func (a MockAdvertisement) Services() []ble.UUID { return nil }

// OverflowService returns a mock overflow service
func (a MockAdvertisement) OverflowService() []ble.UUID { return nil }

// TxPowerLevel returns a mock power level
func (a MockAdvertisement) TxPowerLevel() int { return 0 }

// SolicitedService returns mock solicited services
func (a MockAdvertisement) SolicitedService() []ble.UUID { return nil }

// Connectable returns whether the device is connectable
func (a MockAdvertisement) Connectable() bool { return false }

// MockAddr implements the ble.Addr interface
type MockAddr struct {
	addr string
}

// String returns the address as a string
func (a MockAddr) String() string { return a.addr }

// GoveeMockDevice represents a mocked Govee H5075 device for testing
type GoveeMockDevice struct {
	Address    string
	Name       string
	RSSI       int
	TempC      float64
	Humidity   float64
	Battery    int
	RawData    []byte
}

// NewGoveeMockDevice creates a new mock Govee device with the specified parameters
func NewGoveeMockDevice(name, addr string, tempC, humidity float64, battery, rssi int) GoveeMockDevice {
	// Generate the raw data based on the temperature, humidity, and battery
	rawData := generateGoveeRawData(tempC, humidity, battery)
	
	return GoveeMockDevice{
		Address:  addr,
		Name:     name,
		RSSI:     rssi,
		TempC:    tempC,
		Humidity: humidity,
		Battery:  battery,
		RawData:  rawData,
	}
}

// generateGoveeRawData creates raw manufacturer data based on temp/humidity/battery
func generateGoveeRawData(tempC, humidity float64, battery int) []byte {
	// Format according to Govee H5075 spec:
	// - Header: 0x88, 0xEC
	// - Byte 2: 0x00 (unknown)
	// - Bytes 3-5: Temperature and humidity data (3-byte integer, big-endian)
	// - Byte 6: Battery level
	// - Byte 7: 0x00 (padding)
	
	// Calculate the encoded value for temp and humidity
	// temp * 10000 + humidity * 10
	value := uint32(tempC*10000.0 + humidity*10.0)
	
	// Convert to bytes (big-endian)
	b3 := byte((value >> 16) & 0xFF)
	b4 := byte((value >> 8) & 0xFF)
	b5 := byte(value & 0xFF)
	
	// Create the raw data
	return []byte{0x88, 0xEC, 0x00, b3, b4, b5, byte(battery), 0x00}
}

// GetStandardTestDevices returns a set of standard test devices
func GetStandardTestDevices() []GoveeMockDevice {
	return []GoveeMockDevice{
		NewGoveeMockDevice("GVH5075_A123", "A4:C1:38:25:A1:01", 22.5, 45.0, 87, -65),
		NewGoveeMockDevice("GVH5075_B456", "A4:C1:38:25:A1:02", 19.8, 52.3, 75, -72),
		NewGoveeMockDevice("GVH5075_C789", "A4:C1:38:25:A1:03", 25.3, 38.7, 92, -58),
		NewGoveeMockDevice("GVH5075_D012", "A4:C1:38:25:A1:04", 17.4, 63.1, 63, -80),
	}
}

// HexStringToBytes converts a hex string to bytes for creating raw data
func HexStringToBytes(hexStr string) ([]byte, error) {
	return hex.DecodeString(hexStr)
}

// GoveeTestCase defines a test case for Govee data decoding
type GoveeTestCase struct {
	Name         string
	RawData      string // Hex encoded
	TempC        float64
	Humidity     float64
	Battery      int
}

// GetStandardTestCases returns a set of standard test cases
func GetStandardTestCases() []GoveeTestCase {
	return []GoveeTestCase{
		{
			Name:         "Standard reading",
			RawData:      "88EC0002962D4B00",
			TempC:        16.95,
			Humidity:     51.7,
			Battery:      75,
		},
		{
			Name:         "Low temperature",
			RawData:      "88EC0001542F5500",
			TempC:        15.44,
			Humidity:     53.1,
			Battery:      85,
		},
		{
			Name:         "High temperature",
			RawData:      "88EC0003751A6200",
			TempC:        23.75,
			Humidity:     31.8,
			Battery:      98,
		},
		{
			Name:         "Low battery",
			RawData:      "88EC0002543B1500",
			TempC:        16.54,
			Humidity:     43.9,
			Battery:      21,
		},
	}
}