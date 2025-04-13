# Govee Monitoring System Testing Guide

## Testing Strategy Overview

I've created a comprehensive set of tests for your Govee Monitoring System. These tests are designed to ensure the reliability, performance, and correctness of both the client and server components. Let me explain the different test types I've implemented:

### Unit Tests for Server and Client

I've created unit tests for both the server and client components to test individual functions in isolation:

- **Server Unit Tests:** These test core server functionalities like adding readings, retrieving devices, authentication middleware, storage management, and calculating device statistics.

- **Client Unit Tests:** These verify BLE data decoding, temperature conversions, additional metric calculations, API key handling, and calibration functions.

### Integration Tests

The integration tests verify that the client and server components work correctly together:

- **Client-Server Integration:** Tests the complete flow of sending readings from client to server and retrieving them.
- **Authentication Tests:** Verify that unauthorized access is properly rejected.
- **Data Persistence:** Confirm that data is properly saved and loaded from storage.

### BLE Mock Framework

I've created a BLE mock framework to simulate Bluetooth devices without requiring actual hardware:

- **Mock Implementation:** Provides a complete mock of the BLE interface.
- **Test Devices:** Includes predefined test devices with known values.
- **Raw Data Generation:** Generates synthetic BLE data packets with correct format.

### GitHub Actions Workflow

The GitHub Actions workflow automates testing across multiple platforms and Go versions:

- **Test Matrix:** Tests on both Ubuntu and macOS with Go versions 1.18, 1.19, and 1.20.
- **Coverage Analysis:** Generates and uploads test coverage reports.
- **Docker Build:** Builds and pushes Docker images for easy deployment.

### Makefile for Local Testing

The Makefile simplifies running tests locally with various commands:

- `make test` - Run all tests
- `make test-server` - Run server tests only
- `make test-client` - Run client tests only
- `make test-integration` - Run integration tests only
- `make test-coverage` - Generate coverage reports
- `make test-race` - Run tests with race detection

## Key Testing Aspects

The tests cover several critical aspects of the system:

1. **Data Decoding:** Verifies that BLE data is correctly decoded into temperature, humidity, and battery readings
2. **Authentication and Security:** Ensures that API key-based authentication works properly, with tests for admin keys, client-specific keys, and unauthorized access
3. **Storage Management:** Tests for proper data saving, loading, and time-based partitioning
4. **Data Processing:** Verifies calculations for derived metrics like absolute humidity and dew point
5. **Error Handling:** Tests error cases such as invalid data, malformed requests, and network issues

## Implementation Details

### Server Tests

The server unit tests focus on:

- **Adding and retrieving readings:** Ensures readings are correctly stored and associated with devices and clients
- **HTTP handlers:** Tests both POST and GET handlers for the readings endpoint
- **Authentication middleware:** Verifies proper authorization checks for different API keys
- **Storage management:** Tests saving and loading data across time-based partitions
- **Statistics calculations:** Verifies min/max/avg calculations for temperature, humidity, and other metrics

### Client Tests

The client unit tests verify:

- **BLE data decoding:** Tests parsing of raw Bluetooth data packets from Govee devices
- **Temperature conversion:** Tests Celsius to Fahrenheit conversion accuracy
- **Environmental metrics:** Verifies calculations for absolute humidity, dew point, and steam pressure
- **Calibration:** Tests the application of temperature and humidity offsets
- **Server communication:** Tests sending data to the server with proper API key authentication

### Mock BLE Framework

The BLE mock framework provides:

- **Mock devices:** Simulated Govee H5075 devices that produce realistic data
- **Synthetic data generation:** Creates properly formatted manufacturer data packets
- **Scanning simulation:** Simulates the device discovery process
- **Test cases:** Predefined test cases with known values for verification

### Integration Tests

The integration tests cover:

- **End-to-end data flow:** Tests the complete path from reading generation to server storage and retrieval
- **Authentication flow:** Tests access control with different API keys
- **Data persistence:** Verifies data survives server restarts
- **Client registration:** Tests client discovery and registration with the server

## Using the Tests

To make the most of these tests during development:

1. **Frequent Testing:** Run `make test` regularly during development to catch issues early
2. **Coverage Analysis:** Use `make test-coverage` to identify untested code paths
3. **CI/CD Integration:** The GitHub Actions workflow automatically runs tests on every push
4. **Mocked Testing:** Use the BLE mock framework to test without actual hardware
5. **Isolated Component Testing:** Use `make test-server` or `make test-client` to focus on specific components

## Expanding the Tests

As you continue to develop the system, you might want to expand the tests in these areas:

1. **Performance Testing:** Add benchmarks for critical paths to detect performance regressions
2. **Load Testing:** Test behavior under high load with many clients and devices
3. **Edge Cases:** Add more tests for edge cases like device disconnection and reconnection
4. **Dashboard Testing:** Add tests for the web dashboard functionality
5. **End-to-End Testing:** Create more comprehensive end-to-end tests that involve multiple clients and the dashboard

## Final Thoughts

The testing suite I've provided gives you a solid foundation for ensuring the reliability of your Govee Monitoring System. The combination of unit tests, integration tests, and mock frameworks allows for comprehensive testing without requiring physical hardware for every test.

By running these tests regularly and expanding them as you add features, you can maintain high-quality code while confidently making changes and additions to the system.
