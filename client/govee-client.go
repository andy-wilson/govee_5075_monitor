package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"sync"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
	"github.com/pkg/errors"
)

// GoveeDevice represents a Govee H5075 device
type GoveeDevice struct {
	Address        string    `json:"address"`
	Name           string    `json:"name"`
	RSSI           int       `json:"rssi"`
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
	RawData        string    `json:"raw_data"`
	LastUpdate     time.Time `json:"last_update"`
	ClientID       string    `json:"client_id"`
}

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

// Scanner tracks last seen values with thread-safety
type Scanner struct {
	lastValues map[string]int
	mu         sync.RWMutex
}

// NewScanner creates a new scanner
func NewScanner() *Scanner {
	return &Scanner{
		lastValues: make(map[string]int),
	}
}

// HasValueChanged checks if a value has changed for a device (thread-safe)
func (sc *Scanner) HasValueChanged(addr string, value int) bool {
	sc.mu.RLock()
	lastVal, exists := sc.lastValues[addr]
	sc.mu.RUnlock()

	if !exists || lastVal != value {
		sc.mu.Lock()
		sc.lastValues[addr] = value
		sc.mu.Unlock()
		return true
	}
	return false
}

// SendQueue manages worker pool for sending readings to server
type SendQueue struct {
	queue       chan Reading
	wg          sync.WaitGroup
	serverURL   string
	apiKey      string
	insecure    bool
	caCertFile  string
	httpTimeout time.Duration
}

// NewSendQueue creates a new send queue with worker pool
func NewSendQueue(workers int, serverURL, apiKey string, insecure bool, caCertFile string, httpTimeout time.Duration) *SendQueue {
	sq := &SendQueue{
		queue:       make(chan Reading, 100),
		serverURL:   serverURL,
		apiKey:      apiKey,
		insecure:    insecure,
		caCertFile:  caCertFile,
		httpTimeout: httpTimeout,
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		sq.wg.Add(1)
		go sq.worker()
	}

	return sq
}

// Enqueue adds a reading to the send queue
func (sq *SendQueue) Enqueue(reading Reading) {
	select {
	case sq.queue <- reading:
	default:
		log.Printf("Send queue full, dropping reading for device %s", reading.DeviceAddr)
	}
}

// Close stops the send queue
func (sq *SendQueue) Close() {
	close(sq.queue)
	sq.wg.Wait()
}

// worker processes readings from the queue
func (sq *SendQueue) worker() {
	defer sq.wg.Done()

	for reading := range sq.queue {
		// Retry logic with exponential backoff
		maxRetries := 3
		backoff := time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			err := sendToServer(sq.serverURL, reading, sq.apiKey, sq.insecure, sq.caCertFile, sq.httpTimeout)
			if err == nil {
				break
			}

			if attempt < maxRetries-1 {
				log.Printf("Failed to send reading (attempt %d/%d): %v. Retrying in %v...", attempt+1, maxRetries, err, backoff)
				time.Sleep(backoff)
				backoff *= 2
			} else {
				log.Printf("Failed to send reading after %d attempts: %v", maxRetries, err)
			}
		}
	}
}

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
	tempOffset := flag.Float64("temp-offset", 0.0, "temperature offset calibration (°C)")
	humidityOffset := flag.Float64("humidity-offset", 0.0, "humidity offset calibration (%)")
	// HTTPS flags
	insecureSkipVerify := flag.Bool("insecure-skip-tls-verify-dangerous", false, "DANGEROUS: skip TLS certificate verification (vulnerable to MITM attacks)")
	caCertFile := flag.String("ca-cert", "", "path to CA certificate file for TLS verification")
	httpTimeout := flag.Duration("http-timeout", 10*time.Second, "HTTP request timeout")
	flag.Parse()

	// Check if API key is provided when not in local mode
	if !*localOnly && !*discoveryMode && *apiKey == "" {
		log.Println("Warning: No API key provided. Server communications may fail. Use -apikey flag to provide one or use -local=true for local mode.")
	}

	// Warn about insecure TLS
	if *insecureSkipVerify {
		log.Println("========================================")
		log.Println("WARNING: TLS certificate verification is DISABLED")
		log.Println("WARNING: This makes you vulnerable to man-in-the-middle attacks")
		log.Println("WARNING: DO NOT USE IN PRODUCTION")
		log.Println("========================================")
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

	// Create thread-safe scanner
	scanner := NewScanner()

	// Create send queue with worker pool (5 concurrent senders)
	var sendQueue *SendQueue
	if !*localOnly {
		sendQueue = NewSendQueue(5, *serverURL, *apiKey, *insecureSkipVerify, *caCertFile, *httpTimeout)
		defer sendQueue.Close()
	}

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

						// Only process if the value has changed (thread-safe)
						if scanner.HasValueChanged(addr, int(values)) {

							// Calculate temperature with offset
							tempC := float64(values)/10000.0 + *tempOffset
							humidityRaw := float64(values%1000) / 10.0

							// Calculate humidity with offset
							humidity := humidityRaw + *humidityOffset

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

							// Calculate additional values
							absHumidity, dewPointC, dewPointF, steamPressure := CalculateDerivedValues(tempC, humidity)

							// Store or update device information
							if _, exists := devices[addr]; !exists {
								devices[addr] = &GoveeDevice{
									Address:        addr,
									Name:           name,
									RSSI:           rssi,
									TempC:          tempC,
									TempF:          tempF,
									TempOffset:     *tempOffset,
									Humidity:       humidity,
									HumidityOffset: *humidityOffset,
									AbsHumidity:    absHumidity,
									DewPointC:      dewPointC,
									DewPointF:      dewPointF,
									SteamPressure:  steamPressure,
									Battery:        battery,
									RawData:        mfrDataHex,
									LastUpdate:     time.Now(),
									ClientID:       *clientID,
								}
							} else {
								device := devices[addr]
								device.RSSI = rssi
								device.TempC = tempC
								device.TempF = tempF
								device.TempOffset = *tempOffset
								device.Humidity = humidity
								device.HumidityOffset = *humidityOffset
								device.AbsHumidity = absHumidity
								device.DewPointC = dewPointC
								device.DewPointF = dewPointF
								device.SteamPressure = steamPressure
								device.Battery = battery
								device.RawData = mfrDataHex
								device.LastUpdate = time.Now()
							}

							// Create a reading object
							reading := Reading{
								DeviceName:     name,
								DeviceAddr:     addr,
								TempC:          tempC,
								TempF:          tempF,
								TempOffset:     *tempOffset,
								Humidity:       humidity,
								HumidityOffset: *humidityOffset,
								AbsHumidity:    absHumidity,
								DewPointC:      dewPointC,
								DewPointF:      dewPointF,
								SteamPressure:  steamPressure,
								Battery:        battery,
								RSSI:           rssi,
								Timestamp:      time.Now(),
								ClientID:       *clientID,
							}

							// Log data if requested
							if logger != nil {
								logTime := time.Now().Format("2006-01-02T15:04:05.000")
								logData := fmt.Sprintf("%s,%s,%s,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%d,%d,%s\n",
									logTime, name, addr, tempC, tempF, humidity, absHumidity, dewPointC, dewPointF,
									steamPressure, battery, rssi, *clientID)
								if _, err := logger.WriteString(logData); err != nil {
									log.Printf("Failed to write to log file: %v", err)
								}
							}

							// Send to server if not in local mode (using worker pool)
							if !*localOnly && sendQueue != nil {
								sendQueue.Enqueue(reading)
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
	return math.Round((32.0+9.0*celsius/5.0)*100) / 100
}

// CalculateDerivedValues calculates additional values based on temperature and humidity
func CalculateDerivedValues(tempC, humidity float64) (float64, float64, float64, float64) {
	// Calculate absolute humidity (g/m³)
	absHumidity := CalculateAbsoluteHumidity(tempC, humidity)

	// Calculate dew point (°C)
	dewPointC := CalculateDewPoint(tempC, humidity)

	// Convert dew point to Fahrenheit
	dewPointF := CToF(dewPointC)

	// Calculate steam pressure (hPa)
	steamPressure := CalculateSteamPressure(tempC, humidity)

	return absHumidity, dewPointC, dewPointF, steamPressure
}

// CalculateAbsoluteHumidity calculates absolute humidity in g/m³
// Formula: absHumidity = 216.7 * (relHumidity/100 * 6.112 * exp(17.62*tempC/(243.12+tempC)) / (273.15+tempC))
func CalculateAbsoluteHumidity(tempC, relHumidity float64) float64 {
	// Saturation vapor pressure (hPa)
	satVaporPressure := 6.112 * math.Exp(17.62*tempC/(243.12+tempC))

	// Vapor pressure (hPa)
	vaporPressure := relHumidity / 100.0 * satVaporPressure

	// Absolute humidity (g/m³)
	absHumidity := 216.7 * (vaporPressure / (273.15 + tempC))

	return math.Round(absHumidity*10) / 10 // Round to 1 decimal place
}

// CalculateDewPoint calculates dew point in °C
// Formula: dewPoint = 243.12 * ln(relHumidity/100 * exp(17.62*tempC/(243.12+tempC))) / (17.62 - ln(relHumidity/100 * exp(17.62*tempC/(243.12+tempC))))
func CalculateDewPoint(tempC, relHumidity float64) float64 {
	// Parameter of Magnus formula
	alpha := math.Log(relHumidity / 100.0 * math.Exp((17.62*tempC)/(243.12+tempC)))

	// Dew point (°C)
	dewPoint := 243.12 * alpha / (17.62 - alpha)

	return math.Round(dewPoint*10) / 10 // Round to 1 decimal place
}

// CalculateSteamPressure calculates steam pressure in hPa
// Formula: steamPressure = relHumidity/100 * 6.112 * exp(17.62*tempC/(243.12+tempC))
func CalculateSteamPressure(tempC, relHumidity float64) float64 {
	// Saturation vapor pressure (hPa)
	satVaporPressure := 6.112 * math.Exp(17.62*tempC/(243.12+tempC))

	// Steam pressure (hPa)
	steamPressure := relHumidity / 100.0 * satVaporPressure

	return math.Round(steamPressure*10) / 10 // Round to 1 decimal place
}

func printDeviceText(device *GoveeDevice) {
	fmt.Printf("%s %s Temp: %.1f°C/%.1f°F, Humidity: %.1f%%, Dew Point: %.1f°C, AH: %.1f g/m³, SP: %.1f hPa, Battery: %d%%, RSSI: %ddBm\n",
		device.LastUpdate.Format("2006-01-02T15:04:05"),
		device.Name,
		device.TempC,
		device.TempF,
		device.Humidity,
		device.DewPointC,
		device.AbsHumidity,
		device.SteamPressure,
		device.Battery,
		device.RSSI,
	)
}

func sendToServer(serverURL string, reading Reading, apiKey string, insecureSkipVerify bool, caCertFile string, httpTimeout time.Duration) error {
	// Convert reading to JSON
	jsonData, err := json.Marshal(reading)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	// Create HTTP client with TLS configuration
	tlsConfig := &tls.Config{}

	// Handle certificate verification options
	if insecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	} else if caCertFile != "" {
		// Load CA cert if specified
		caCert, err := os.ReadFile(caCertFile)
		if err != nil {
			return fmt.Errorf("error loading CA certificate: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return fmt.Errorf("failed to append CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	// Create transport and client
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Timeout:   httpTimeout,
		Transport: transport,
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add API key for authentication
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending data to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: Invalid API key")
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server responded with status %d", resp.StatusCode)
	}

	return nil
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
