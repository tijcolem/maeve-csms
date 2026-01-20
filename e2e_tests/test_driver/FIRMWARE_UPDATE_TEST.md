# Firmware Update E2E Test

This directory contains end-to-end tests for firmware update functionality in the MaEVe CSMS.

## Overview

The `firmware_update_test.go` provides comprehensive end-to-end testing for OCPP firmware update functionality, including:

- Starting a local HTTP firmware server
- Monitoring firmware status notifications via MQTT
- Tracking status progression (Downloading → Downloaded → Installing → Installed)
- Verifying firmware downloads
- Checksum validation

## Running the Test

### Prerequisites

1. **CSMS must be running:**
   ```bash
   cd /path/to/maeve-csms
   ./scripts/run.sh
   ```

2. **Charge station must be connected:**
   - EVSE connected to the CSMS
   - Using OCPP 2.0.1
   - Registered with the system

3. **MQTT broker accessible:**
   - Default: `localhost:1884`
   - Set `MQTT_BROKER_ADDR` to override

### Running the Test

```bash
# Run all e2e tests
cd e2e_tests/test_driver
go test -tags=e2e -v

# Run only firmware update test
go test -tags=e2e -v -run TestFirmwareUpdateOCPP201

# With custom configuration
CHARGE_STATION_ID=cs002 \
FIRMWARE_SERVER_PORT=9999 \
CSMS_API_URL=http://localhost:9410 \
go test -tags=e2e -v -run TestFirmwareUpdateOCPP201
```

### Skip the Test

```bash
# Skip firmware test if not ready
SKIP_FIRMWARE_TEST=1 go test -tags=e2e -v
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CHARGE_STATION_ID` | ID of the charge station to test | `cs001` |
| `FIRMWARE_SERVER_PORT` | Port for test firmware HTTP server | `8888` |
| `CSMS_API_URL` | CSMS API base URL | `http://localhost:9410` |
| `MQTT_BROKER_ADDR` | MQTT broker address | `127.0.0.1:1884` |
| `SKIP_FIRMWARE_TEST` | Skip firmware test if set | (none) |

## Test Components

### FirmwareTestServer

A local HTTP server that serves test firmware files:
- Generates random firmware data (default 1MB)
- Calculates SHA256 checksum
- Tracks download attempts
- Provides health check endpoint

**Endpoints:**
- `GET /firmware.bin` - Download firmware
- `GET /health` - Health check

### FirmwareStatusTracker

Monitors and records firmware status changes:
- Subscribes to MQTT topics for status notifications
- Tracks all status changes with timestamps
- Verifies status progression
- Provides query methods for test assertions

### Test Flow

```
1. Start FirmwareTestServer
   └─> Serve test firmware at http://host.docker.internal:8888/firmware.bin

2. Connect to MQTT broker
   └─> Subscribe to cs/in/ocpp201/{chargeStationId}

3. Send UpdateFirmware OCPP request
   └─> (Currently requires manual trigger - see below)

4. Monitor FirmwareStatusNotification messages
   └─> Idle → Downloading → Downloaded → Installing → Installed

5. Verify firmware was downloaded
   └─> Check download count on test server

6. Assert test success
   └─> All expected statuses received
   └─> Firmware successfully installed
```

## Current Limitations

### Manual Triggering Required

The test currently requires **manual triggering** of the UpdateFirmware OCPP message because:
- The CSMS doesn't yet have a REST API endpoint to trigger firmware updates
- Direct MQTT publishing requires additional setup

**To manually trigger the update:**

1. Run the test - it will start the firmware server and wait:
   ```bash
   go test -tags=e2e -v -run TestFirmwareUpdateOCPP201
   ```

2. In another terminal, send the UpdateFirmware message using the manager's CallMaker:
   ```go
   // Example code to trigger (run separately)
   callMaker := ocpp201.NewCallMaker(emitter)
   request := &ocpp201.UpdateFirmwareRequestJson{
       RequestId: 12345,
       Firmware: ocpp201.FirmwareType{
           Location: "http://host.docker.internal:8888/firmware.bin",
           RetrieveDateTime: "2026-01-20T15:00:00Z",
       },
   }
   callMaker.Send(ctx, "cs001", request)
   ```

3. Or use the Python helper script:
   ```bash
   # In another terminal
   cd /path/to/maeve-csms
   ./scripts/test-firmware-update.py --cs-id cs001
   ```

The test will automatically detect when the firmware update completes.

## Expected Output

### Successful Test

```
=== RUN   TestFirmwareUpdateOCPP201
    firmware_update_test.go:XXX: Starting firmware update test for charge station: cs001
    firmware_update_test.go:XXX: Firmware server started on port 8888
    firmware_update_test.go:XXX: Firmware URL: http://host.docker.internal:8888/firmware.bin
    firmware_update_test.go:XXX: Firmware checksum: a1b2c3d4...
    firmware_update_test.go:XXX: Subscribing to firmware status on topic: cs/in/ocpp201/cs001
    firmware_update_test.go:XXX: UpdateFirmware request:
    {
      "requestId": 1705766400,
      "firmware": {
        "location": "http://host.docker.internal:8888/firmware.bin",
        "retrieveDateTime": "2026-01-20T15:00:00Z",
        "installDateTime": "2026-01-20T15:05:00Z"
      },
      "retries": 3,
      "retryInterval": 30
    }
    firmware_update_test.go:XXX: Waiting for firmware update to complete...
    firmware_update_test.go:XXX: Expected status progression:
    firmware_update_test.go:XXX:   1. Idle
    firmware_update_test.go:XXX:   2. Downloading
    firmware_update_test.go:XXX:   3. Downloaded
    firmware_update_test.go:XXX:   4. Installing
    firmware_update_test.go:XXX:   5. Installed
    firmware_update_test.go:XXX: Firmware status: Downloading
    firmware_update_test.go:XXX: Firmware status: Downloaded
    firmware_update_test.go:XXX: Firmware status: Installing
    firmware_update_test.go:XXX: Firmware status: Installed
    firmware_update_test.go:XXX: Final status: Installed
    firmware_update_test.go:XXX: All statuses: [Downloading Downloaded Installing Installed]
    firmware_update_test.go:XXX: Firmware downloaded 1 time(s)
    firmware_update_test.go:XXX: ✓ Firmware update completed successfully
    firmware_update_test.go:XXX: ✓ Downloading status received
    firmware_update_test.go:XXX: ✓ Downloaded status received
    firmware_update_test.go:XXX: ✓ Installing status received
    firmware_update_test.go:XXX: ✓ Firmware was downloaded from test server
--- PASS: TestFirmwareUpdateOCPP201 (45.23s)
PASS
```

### Test Timeout (No Manual Trigger)

```
=== RUN   TestFirmwareUpdateOCPP201
    firmware_update_test.go:XXX: Starting firmware update test for charge station: cs001
    firmware_update_test.go:XXX: Firmware server started on port 8888
    ...
    firmware_update_test.go:XXX: WARNING: Firmware update timed out after 5m0s
    firmware_update_test.go:XXX: This test requires manual triggering of the UpdateFirmware OCPP message
    firmware_update_test.go:XXX: Recorded statuses: []
--- SKIP: TestFirmwareUpdateOCPP201 (300.01s)
PASS
```

## Future Improvements

1. **Automatic Triggering**: Add REST API endpoint to CSMS for triggering firmware updates
2. **Database Integration**: Query database for firmware update status
3. **Multiple EVSE Testing**: Test firmware updates across multiple charge stations
4. **Failure Scenarios**: Test download failures, invalid signatures, etc.
5. **Signed Firmware**: Test firmware signature verification
6. **Progress Tracking**: Monitor download/install progress percentages
7. **Cancellation**: Test firmware update cancellation

## Troubleshooting

### Test Hangs

**Problem**: Test starts but never receives status notifications

**Solutions**:
1. Check MQTT broker is running:
   ```bash
   docker ps | grep mqtt
   ```

2. Verify charge station is connected:
   ```bash
   docker logs maeve-csms-gateway-1 | grep "charge station connected"
   ```

3. Check MQTT topic subscriptions:
   ```bash
   docker exec -it maeve-csms-mqtt-1 mosquitto_sub -t 'cs/in/#' -v
   ```

### Firmware Not Downloaded

**Problem**: Test completes but download count is 0

**Solutions**:
1. Check firmware server is accessible:
   ```bash
   curl http://localhost:8888/health
   ```

2. Verify charge station can reach host:
   - Use `host.docker.internal` for Docker containers
   - Check network connectivity

3. Review charge station logs for download errors

### Wrong MQTT Topic

**Problem**: Status notifications not detected

**Solutions**:
1. Verify OCPP version matches topic:
   - OCPP 2.0.1: `cs/in/ocpp201/{csId}`
   - OCPP 1.6: `cs/in/ocpp16/{csId}`

2. Check MQTT topic prefix configuration

3. Monitor all MQTT messages:
   ```bash
   docker exec -it maeve-csms-mqtt-1 mosquitto_sub -t '#' -v
   ```

## Integration with CI/CD

### Skip in CI by default

```yaml
# .github/workflows/e2e-tests.yml
- name: Run E2E Tests
  env:
    SKIP_FIRMWARE_TEST: "1"  # Skip firmware test in CI
  run: |
    cd e2e_tests/test_driver
    go test -tags=e2e -v
```

### Run in dedicated firmware test job

```yaml
- name: Run Firmware Update Test
  if: github.event_name == 'workflow_dispatch'  # Manual trigger only
  env:
    CHARGE_STATION_ID: "ci-test-cs001"
    FIRMWARE_SERVER_PORT: "8888"
  run: |
    cd e2e_tests/test_driver
    go test -tags=e2e -v -run TestFirmwareUpdateOCPP201 -timeout 10m
```

## Related Documentation

- [Firmware Update Testing Guide](../../scripts/FIRMWARE_UPDATE_TESTING.md)
- [EVSE Integration Guide](../../scripts/EVSE_INTEGRATION.md)
- [Manager API Documentation](../../manager/api/API.md)
- [OCPP Implementation](../../manager/handlers/ocpp201/)
