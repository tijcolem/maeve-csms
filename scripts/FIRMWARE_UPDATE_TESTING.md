# Firmware Update Testing

This directory contains test scripts and tools for testing the firmware update functionality in the MaEVe CSMS.

## Overview

The firmware update feature allows the CSMS to send firmware updates to connected charge stations (EVSEs) using OCPP UpdateFirmware messages. The charge station downloads the firmware from a specified URL and reports its progress through FirmwareStatusNotification messages.

## Test Scripts

### 1. Bash Test Script (`test-firmware-update.sh`)

A simple bash script that:
- Creates a test firmware file (1MB random data)
- Starts a local HTTP server to serve the firmware
- Registers a charge station with the CSMS
- Generates an UpdateFirmware request JSON

**Usage:**
```bash
./scripts/test-firmware-update.sh
```

**Environment Variables:**
- `CSMS_API_URL` - CSMS API endpoint (default: http://localhost:9410/api/v0)
- `CHARGE_STATION_ID` - Charge station identifier (default: cs001)
- `FIRMWARE_SERVER_PORT` - Port for firmware HTTP server (default: 8888)
- `OCPP_VERSION` - OCPP version to use: "1.6" or "2.0.1" (default: 2.0.1)

**Example:**
```bash
OCPP_VERSION=1.6 CHARGE_STATION_ID=cs002 ./scripts/test-firmware-update.sh
```

### 2. Python E2E Test Script (`test-firmware-update.py`)

A comprehensive Python script that provides more control and monitoring:
- Creates test firmware with configurable size
- Calculates SHA256 checksum
- Starts local firmware server
- Registers charge station
- Generates OCPP requests for both 1.6 and 2.0.1
- Provides monitoring instructions

**Requirements:**
```bash
pip install requests
```

**Usage:**
```bash
./scripts/test-firmware-update.py --help
```

**Options:**
- `--cs-id` - Charge station ID (default: cs001)
- `--ocpp-version` - OCPP version: "1.6" or "2.0.1" (default: 2.0.1)
- `--firmware-size` - Firmware size in MB (default: 1)
- `--port` - Firmware server port (default: 8888)
- `--no-server` - Don't start firmware server
- `--firmware-url` - Use external firmware URL

**Examples:**
```bash
# Basic test with OCPP 2.0.1
./scripts/test-firmware-update.py

# Test with OCPP 1.6 and 5MB firmware
./scripts/test-firmware-update.py --ocpp-version 1.6 --firmware-size 5

# Use external firmware URL
./scripts/test-firmware-update.py --no-server --firmware-url https://example.com/firmware.bin
```

### 3. Go Integration Tests (`manager/handlers/integration_test/firmware_update_test.go`)

Unit and integration tests for the firmware update functionality:
- Tests OCPP 2.0.1 UpdateFirmware request creation
- Tests OCPP 1.6 UpdateFirmware request creation
- Tests request with retry parameters
- Tests signed firmware updates
- Tests multiple concurrent update requests
- Tests firmware server availability and download

**Running the tests:**
```bash
cd manager
go test -tags=integration ./handlers/integration_test/firmware_update_test.go -v
```

**With external firmware server:**
```bash
FIRMWARE_SERVER_URL=http://localhost:8888/test_firmware.bin \
  go test -tags=integration ./handlers/integration_test/firmware_update_test.go -v
```

## Firmware Update Flow

### OCPP 2.0.1 Flow

1. **CSMS sends UpdateFirmware request:**
   ```json
   {
     "requestId": 12345,
     "firmware": {
       "location": "https://example.com/firmware.bin",
       "retrieveDateTime": "2026-01-13T15:00:00Z",
       "installDateTime": "2026-01-13T15:10:00Z"
     },
     "retries": 3,
     "retryInterval": 60
   }
   ```

2. **Charge station responds:**
   ```json
   {
     "status": "Accepted"
   }
   ```

3. **Charge station sends status notifications:**
   - `Idle` - Waiting to start
   - `Downloading` - Downloading firmware
   - `Downloaded` - Download complete
   - `Installing` - Installing firmware
   - `Installed` - Installation complete

4. **Or error states:**
   - `DownloadFailed` - Download failed
   - `InstallationFailed` - Installation failed
   - `InvalidSignature` - Signature verification failed

### OCPP 1.6 Flow

Similar to 2.0.1 but with simpler request format:
```json
{
  "location": "https://example.com/firmware.bin",
  "retrieveDate": "2026-01-13T15:00:00Z",
  "retries": 3,
  "retryInterval": 60
}
```

## Sending UpdateFirmware Programmatically

### Using Go CallMaker

```go
import (
    "context"
    "time"
    "github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
)

// Create call maker
callMaker := ocpp201.NewCallMaker(emitter)

// Prepare request
retrieveTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
installTime := time.Now().Add(10 * time.Minute).Format(time.RFC3339)

request := &ocpp201.UpdateFirmwareRequestJson{
    RequestId: 12345,
    Firmware: ocpp201.FirmwareType{
        Location:         "https://example.com/firmware.bin",
        RetrieveDateTime: retrieveTime,
        InstallDateTime:  &installTime,
    },
}

// Send to charge station
err := callMaker.Send(context.Background(), "cs001", request)
```

### For OCPP 1.6

```go
import (
    "github.com/thoughtworks/maeve-csms/manager/handlers/ocpp16"
    "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp16"
)

callMaker := ocpp16.NewCallMaker(emitter)

retrieveDate := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
retries := 3
retryInterval := 60

request := &ocpp16.UpdateFirmwareJson{
    Location:      "https://example.com/firmware.bin",
    RetrieveDate:  retrieveDate,
    Retries:       &retries,
    RetryInterval: &retryInterval,
}

err := callMaker.Send(context.Background(), "cs001", request)
```

## Monitoring Firmware Updates

### View CSMS Logs
```bash
# Follow all logs
docker logs -f maeve-csms-manager-1

# Filter for firmware-related messages
docker logs -f maeve-csms-manager-1 | grep -i firmware

# View last 100 lines
docker logs --tail 100 maeve-csms-manager-1 | grep -i firmware
```

### Expected Log Messages
- `sending message action=UpdateFirmware chargeStationId=cs001`
- `firmware_status.status=Downloading`
- `firmware_status.status=Downloaded`
- `firmware_status.status=Installing`
- `firmware_status.status=Installed`

## Troubleshooting

### Firmware Server Not Accessible

If running in Docker, use `host.docker.internal` instead of `localhost`:
```
http://host.docker.internal:8888/firmware.bin
```

### Charge Station Not Registered

Register the charge station first:
```bash
curl -X POST http://localhost:9410/api/v0/cs/cs001 \
  -H "Content-Type: application/json" \
  -d '{
    "securityProfile": 1,
    "base64SHA256Password": "test123",
    "invalidUsernameAllowed": true
  }'
```

### UpdateFirmware Message Not Sent

Ensure the charge station is connected and the CSMS is running:
```bash
docker ps | grep maeve-csms
```

### Download Fails

Check:
1. Firmware URL is accessible from charge station network
2. Firmware server is running
3. File exists at the specified path
4. No firewall blocking the connection

## Security Considerations

### Signed Firmware (OCPP 2.0.1)

For production deployments, use signed firmware:

```go
signingCert := "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
signature := "base64_encoded_signature"

request := &ocpp201.UpdateFirmwareRequestJson{
    RequestId: 12345,
    Firmware: ocpp201.FirmwareType{
        Location:           "https://example.com/firmware.bin",
        RetrieveDateTime:   retrieveTime,
        SigningCertificate: &signingCert,
        Signature:          &signature,
    },
}
```

The charge station will verify the signature before installing the firmware.

## Next Steps

1. **Add API Endpoint**: Create a REST API endpoint to trigger firmware updates from external systems
2. **Database Tracking**: Store firmware update status in the database
3. **Progress Monitoring**: Track download/install progress percentages
4. **Rollback Support**: Implement firmware rollback in case of failures
5. **Bulk Updates**: Support updating multiple charge stations simultaneously

## Files Created

- `manager/ocpp/ocpp201/update_firmware_request.go` - OCPP 2.0.1 request types
- `manager/ocpp/ocpp201/update_firmware_response.go` - OCPP 2.0.1 response types
- `manager/ocpp/ocpp16/update_firmware.go` - OCPP 1.6 request types
- `manager/ocpp/ocpp16/update_firmware_response.go` - OCPP 1.6 response types
- `manager/handlers/ocpp201/update_firmware_result.go` - OCPP 2.0.1 result handler
- `manager/handlers/ocpp16/update_firmware_result.go` - OCPP 1.6 result handler
- `manager/handlers/examples/update_firmware_example.go` - Usage examples

## References

- [OCPP 2.0.1 Specification](https://www.openchargealliance.org/protocols/ocpp-201/)
- [OCPP 1.6 Specification](https://www.openchargealliance.org/protocols/ocpp-16/)
- [MaEVe CSMS Documentation](../README.md)
