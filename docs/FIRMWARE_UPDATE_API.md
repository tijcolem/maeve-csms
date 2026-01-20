# Firmware Update API - Now Available

## Implementation Complete ✓

The REST API endpoint for firmware updates has been successfully implemented and is now available in the MaEVe CSMS.

## API Endpoint

```
POST /api/v0/cs/{csId}/firmware
```

### Request Body

```json
{
  "location": "https://example.com/firmware/v1.2.3.bin",
  "retrieveDateTime": "2026-01-20T12:00:00Z",
  "installDateTime": "2026-01-20T13:00:00Z",
  "requestId": 12345,
  "retries": 3,
  "retryInterval": 30
}
```

**Parameters:**
- `location` (required): URL where firmware can be downloaded
- `retrieveDateTime` (required): When to start downloading
- `installDateTime` (optional): When to install after download (OCPP 2.0.1 only)
- `requestId` (optional): Unique identifier for this request (OCPP 2.0.1 only)
- `retries` (optional): Number of retry attempts if download fails
- `retryInterval` (optional): Seconds between retry attempts

### Response

- **201 Created**: Firmware update request sent successfully
- **400 Bad Request**: Invalid request or OCPP version not configured
- **500 Internal Server Error**: Failed to send request to charge station

## Usage Examples

### OCPP 2.0.1 (with install time and request ID)

```bash
curl -X POST http://localhost:9410/api/v0/cs/cs001/firmware \
  -H "Content-Type: application/json" \
  -d '{
    "location": "http://localhost:8888/firmware.bin",
    "retrieveDateTime": "2026-01-20T12:00:00Z",
    "installDateTime": "2026-01-20T13:00:00Z",
    "requestId": 12345,
    "retries": 3,
    "retryInterval": 30
  }'
```

### OCPP 1.6 (basic firmware update)

```bash
curl -X POST http://localhost:9410/api/v0/cs/cs001/firmware \
  -H "Content-Type: application/json" \
  -d '{
    "location": "http://localhost:8888/firmware.bin",
    "retrieveDateTime": "2026-01-20T12:00:00Z",
    "retries": 3,
    "retryInterval": 30
  }'
```

### Python Example

```python
import requests
from datetime import datetime, timedelta

csms_url = "http://localhost:9410"
charge_station_id = "cs001"
firmware_url = "http://localhost:8888/firmware.bin"

# Calculate times
retrieve_time = (datetime.utcnow() + timedelta(seconds=30)).isoformat() + 'Z'
install_time = (datetime.utcnow() + timedelta(minutes=2)).isoformat() + 'Z'

request = {
    "location": firmware_url,
    "retrieveDateTime": retrieve_time,
    "installDateTime": install_time,
    "requestId": 12345,
    "retries": 3,
    "retryInterval": 30
}

response = requests.post(
    f"{csms_url}/api/v0/cs/{charge_station_id}/firmware",
    json=request
)

if response.status_code == 201:
    print("Firmware update initiated successfully")
else:
    print(f"Error: {response.status_code} - {response.text}")
```

## Architecture

The endpoint integrates seamlessly with the existing MaEVe CSMS architecture:

```
HTTP Client → REST API → CallMaker → MQTT → Gateway → EVSE
                ↓                        ↑
            Validation              Status Updates
```

### Flow

1. **API Request**: Client sends POST request with firmware details
2. **Validation**: API validates request body and parameters
3. **Version Detection**: Determines OCPP version based on parameters
   - If `requestId` or `installDateTime` present → OCPP 2.0.1
   - Otherwise → OCPP 1.6
4. **CallMaker**: Sends OCPP message via appropriate CallMaker
5. **MQTT**: Message published to `cs/out/ocpp{version}/{csId}` topic
6. **Gateway**: Routes message to connected charge station
7. **EVSE**: Processes UpdateFirmware request and begins download
8. **Status Updates**: EVSE sends FirmwareStatusNotification messages back

## Implementation Details

### Files Modified

1. **manager/api/api-spec.yaml**
   - Added `/cs/{csId}/firmware` endpoint definition
   - Added `ChargeStationFirmwareUpdate` schema

2. **manager/api/api.gen.go**
   - Auto-generated types and handlers (via `oapi-codegen`)

3. **manager/api/server.go**
   - Added CallMaker fields to `Server` struct
   - Updated `NewServer()` to accept CallMakers
   - Added `UpdateChargeStationFirmware()` handler

4. **manager/api/firmware.go** (new)
   - Helper functions for OCPP 2.0.1 and 1.6 firmware updates
   - `sendOCPP201FirmwareUpdate()`
   - `sendOCPP16FirmwareUpdate()`

5. **manager/api/render.go**
   - Added `Bind()` method for `ChargeStationFirmwareUpdate`

6. **manager/server/api.go**
   - Updated `NewApiHandler()` to create and pass CallMakers
   - Imports OCPP handler packages

7. **manager/cmd/serve.go**
   - Updated `NewApiHandler()` call to pass `MsgEmitter`

8. **Test files**
   - Updated all test files to pass `nil` for CallMakers when not needed

### E2E Test Integration

The E2E test (`e2e_tests/test_driver/firmware_update_test.go`) now fully automates firmware updates:

```go
// Automatically triggers via API - no manual intervention needed
func sendUpdateFirmwareRequest(t *testing.T, chargeStationID, firmwareURL string, requestID int) error {
    apiURL := fmt.Sprintf("%s/api/v0/cs/%s/firmware", csmsAPIURL, chargeStationID)
    resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(requestJSON))
    // ...
}
```

## Benefits

### ✅ Automated Testing
E2E tests run without manual intervention

### ✅ External Integration
Systems can trigger firmware updates via simple HTTP calls

### ✅ Operational Simplicity
Operators can use curl/Postman for testing and emergency updates

### ✅ Future-Ready
Foundation for building web UI or management dashboards

## Migration from Previous Workarounds

If you were using any of the previous workarounds (Python scripts, direct MQTT publishing, custom Go code), you can now simplify to a single API call:

**Before (Python script):**
```python
# Complex MQTT publishing code...
```

**After (API call):**
```python
requests.post(f"{csms_url}/api/v0/cs/{cs_id}/firmware", json=request)
```

## Next Steps

Potential enhancements:
- [ ] Add database tracking of firmware update status
- [ ] Build web UI for firmware management
- [ ] Add batch firmware update capability
- [ ] Implement firmware signing/verification
- [ ] Add firmware rollback capability

## Testing

Run the complete E2E test:

```bash
cd e2e_tests
go test -tags=e2e ./test_driver -v -run TestFirmwareUpdateOCPP201
```

The test will:
1. Start a local firmware HTTP server
2. Trigger firmware update via API
3. Monitor MQTT for status notifications
4. Validate complete update flow
5. Verify firmware download occurred
