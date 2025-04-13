package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
	"github.com/pkg/errors"
)

// GoveeDevice represents a Govee H5075 device
type GoveeDevice struct {
	Address    string    `json:"address"`
	Name       string    `json:"name"`
	RSSI       int       `json:"rssi"`
	TempC      float64   `json:"temp_c"`
	TempF      float64   `json:"temp_f"`
	Humidity   float64   `json:"humidity"`
	Battery    int       `json:"battery"`
	RawData    string    `json:"raw_data"`
	LastUpdate time.Time `json:"last_update"`
	ClientID   string    `json:"client_id"`
}

// Reading represents a single measurement from a Govee device
type Reading struct {
	DeviceName string    `json:"device_name"`
	DeviceAddr string    `json:"device_addr"`
	TempC      float64   `json:"temp_c"`
	TempF      float64   `json:"temp_f"`
	Humidity   float64   `json:"humidity"`
	Battery    int       `json:"battery"`
	RSSI       int       `json:"rssi"`
	Timestamp  time.Time `json:"timestamp"`
	ClientID   string    `json:"client_id"`
}

// Last seen values to avoid duplicate prints
var lastValues = make(map[string]int)

func main() {
	// Parse command line arguments
	duration := flag.Duration("duration", 30*time.Second, "scanning duration for each cycle")
	serverURL := flag.String("server", "http://localhost:8080/readings", "URL of the server API endpoint")
	clientID := flag.String("id", getDefaultClientID(), "unique ID for this client")
	apiKey := flag.String("apikey", "", "API key for server authentication")
	continuous := flag.Bool("continuous", false, "continuous scanning")
	runTime := flag.Duration("runtime", 0, "total running time (0 for unlimited)")
	verbose := flag.Bool("verbose", false, "print verbose debug information")
	logFile := flag.String("log", "", "file to log data to (empty for no logging)")
	localOnly := flag.Bool("local", false, "local mode (don't send to server)")
	discoveryMode := flag.Bool("discover", false, "discovery mode - only scan for devices and print a list")
	flag.Parse()

	// Check if API key is provided when not in local mode
	if !*localOnly && !*discoveryMode && *apiKey == "" {
		log.Println("Warning: No API key provided. Server communications may fail. Use -apikey flag to provide one or use -local=true for local mode.")
	}

	// Initialize logging if requested
	var logger *os.File
	var err error
	if *logFile != "" {
		logger, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logger.Close()
		log.Printf("Logging data to %s", *logFile)
	}

	// Initialize BLE device
	d, err := dev.NewDevice("default")
	if err != nil {
		log.Fatalf("Failed to open device: %v", err)
	}
	ble.SetDefaultDevice(d)

	// Handle Ctrl-C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		fmt.Println("\nReceived interrupt signal. Shutting down gracefully...")
		cancel()
	}()

	// Map to store discovered devices
	devices := make(map[string]*GoveeDevice)

	// Calculate end time if runtime is specified
	var endTime time.Time
	if *runTime > 0 {
		endTime = time.Now().Add(*runTime)
		log.Printf("Will run until %s", endTime.Format("15:04:05"))
	}

	// Start scanning
	if *discoveryMode {
		fmt.Printf("Govee Client %s: Discovery mode - scanning for Govee H5075 devices for %s...\n", *clientID, duration.String())
	} else {
		fmt.Printf("Govee Client %s: Scanning for Govee H5075 devices...\n", *clientID)
	}
	
	scanCount := 0
	startTime := time.Now()
	
	for {
		// Check if we've exceeded the total runtime
		if *runTime > 0 && time.Now().After(endTime) {
			fmt.Printf("Reached specified runtime of %s. Exiting.\n", runTime.String())
			break
		}

		scanCtx, scanCancel := context.WithTimeout(ctx, *duration)
		scanCount++
		
		if *verbose && scanCount > 1 {
			runningFor := time.Since(startTime).Round(time.Second)
			fmt.Printf("Starting scan cycle %d (running for %s)...\n", scanCount, runningFor)
		}
		defer scanCancel()

		if err := ble.Scan(scanCtx, true, func(a ble.Advertisement) {
			// Get device info
			addr := a.Addr().String()
			name := a.LocalName()
			rssi := a.RSSI()
			
			// Check if this might be a Govee device by name
			isGoveeDevice := strings.HasPrefix(name, "GVH5075")
			
			// Get the manufacturer data
			mfrData := a.ManufacturerData()
			mfrDataHex := hex.EncodeToString(mfrData)
			
			// Process Govee data if found
			if isGoveeDevice && len(mfrData) > 6 {
				// Check if this is a valid Govee format (starting with 88EC)
				if len(mfrData) >= 2 && mfrData[0] == 0x88 && mfrData[1] == 0xEC {
					// In discovery mode, just record the device without processing values
					if *discoveryMode {
						if _, exists := devices[addr]; !exists {
							devices[addr] = &GoveeDevice{
								Address:    addr,
								Name:       name,
								RSSI:       rssi,
								RawData:    mfrDataHex,
								LastUpdate: time.Now(),
							}
						} else {
							// Update RSSI for existing device
							devices[addr].RSSI = rssi
						}
						return
					}
					
					// Make sure we have enough bytes for normal mode
					if len(mfrData) >= 7 {
						// Convert bytes 3-5 to an integer in big endian
						values := uint32(0)
						for i := 0; i < 3; i++ {
							values = (values << 8) | uint32(mfrData[i+3])
						}
						
						// Only process if the value has changed
						if lastVal, exists := lastValues[addr]; !exists || lastVal != int(values) {
							lastValues[addr] = int(values)
							
							// Calculate temperature and humidity
							tempC := float64(values) / 10000.0
							humidity := float64(values%1000) / 10.0
							
							// Battery is directly from byte 6
							battery := int(mfrData[6])
							
							if *verbose {
								fmt.Printf("DEBUG: Device: %s (%s) RSSI: %d\n", addr, name, rssi)
								fmt.Printf("  Raw data: %s\n", mfrDataHex)
								fmt.Printf("  Bytes 3-5: %02x %02x %02x\n", mfrData[3], mfrData[4], mfrData[5])
								fmt.Printf("  Values int: %d\n", values)
								fmt.Printf("  Decoded: Temp: %.1f°C, Humidity: %.1f%%, Battery: %d%%\n", 
									tempC, humidity, battery)
							}
							
							// Calculate temperature in Fahrenheit
							tempF := CToF(tempC)
							
							// Store or update device information
							if _, exists := devices[addr]; !exists {
								devices[addr] = &GoveeDevice{
									Address:    addr,
									Name:       name,
									RSSI:       rssi,
									TempC:      tempC,
									TempF:      tempF,
									Humidity:   humidity,
									Battery:    battery,
									RawData:    mfrDataHex,
									LastUpdate: time.Now(),
									ClientID:   *clientID,
								}
							} else {
								devices[addr].RSSI = rssi
								devices[addr].TempC = tempC
								devices[addr].TempF = tempF
								devices[addr].Humidity = humidity
								devices[addr].Battery = battery
								devices[addr].RawData = mfrDataHex
								devices[addr].LastUpdate = time.Now()
							}
							
							// Create a reading object
							reading := Reading{
								DeviceName: name,
								DeviceAddr: addr,
								TempC:      tempC,
								TempF:      tempF,
								Humidity:   humidity,
								Battery:    battery,
								RSSI:       rssi,
								Timestamp:  time.Now(),
								ClientID:   *clientID,
							}
							
							// Log data if requested
							if logger != nil {
								logTime := time.Now().Format("2006-01-02T15:04:05.000")
								logData := fmt.Sprintf("%s,%s,%s,%.1f,%.1f,%d,%d,%s\n",
									logTime, name, addr, tempC, tempF, humidity, battery, rssi, *clientID)
								if _, err := logger.WriteString(logData); err != nil {
									log.Printf("Failed to write to log file: %v", err)
								}
							}
							
							// Send to server if not in local mode
							if !*localOnly {
								go sendToServer(*serverURL, reading, *apiKey)
							}
							
							// Print device information
							printDeviceText(devices[addr])
						}
					}
				}
			}
		}, nil); err != nil {
			// Only log errors that aren't from context deadlines
			if !errors.Is(err, context.DeadlineExceeded) {
				log.Printf("Scan error: %v", errors.Wrap(err, "scanning failed"))
			} else if *verbose {
				fmt.Println("Scan cycle completed.")
			}
		}

		scanCancel() // Clean up the scan context

		// In discovery mode, print device list after scan completes
		if *discoveryMode {
			fmt.Printf("\n=== Discovered Govee Devices (%d found) ===\n\n", len(devices))
			fmt.Printf("%-20s %-15s %s\n", "Device Name", "MAC Address", "Signal Strength")
			fmt.Printf("%-20s %-15s %s\n", "--------------------", "---------------", "---------------")
			
			for _, device := range devices {
				fmt.Printf("%-20s %-15s %ddBm\n", 
					device.Name, 
					device.Address,
					device.RSSI)
			}
			fmt.Println("\nUse these device names/addresses in your monitoring configuration.")
			break // Exit after one scan in discovery mode
		}

		if !*continuous {
			break
		}
		
		// Check if context was canceled (e.g. by Ctrl-C)
		select {
		case <-ctx.Done():
			return
		default:
			// Continue with next iteration
		}
	}
	
	if !*discoveryMode {
		fmt.Printf("Scan completed after %s. Discovered %d devices.\n", 
			time.Since(startTime).Round(time.Second), len(devices))
	}
}

// CToF converts Celsius to Fahrenheit
func CToF(celsius float64) float64 {
	return math.Round((32.0 + 9.0*celsius/5.0) * 100) / 100
}

func printDeviceText(device *GoveeDevice) {
	fmt.Printf("%s %s Temp: %.1f°C/%.1f°F, Humidity: %.1f%%, Battery: %d%%, RSSI: %ddBm\n",
		device.LastUpdate.Format("2006-01-02T15:04:05"),
		device.Name,
		device.TempC,
		device.TempF,
		device.Humidity,
		device.Battery,
		device.RSSI,
	)
}

func sendToServer(serverURL string, reading Reading, apiKey string) {
	// Convert reading to JSON
	jsonData, err := json.Marshal(reading)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}
	
	// Create HTTP request
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Add API key for authentication
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	
	// Send the request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending data to server: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		log.Printf("Authentication failed: Invalid API key")
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("Server responded with status %d", resp.StatusCode)
	}
}

// getDefaultClientID generates a default client ID based on hostname
func getDefaultClientID() string {
	hostname, err := os.Hostname()
	if err != nil {
		// If hostname can't be determined, use a timestamp
		return fmt.Sprintf("client-%d", time.Now().Unix())
	}
	return fmt.Sprintf("client-%s", hostname)
}
