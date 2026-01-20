# Current Limitation: Missing Firmware Update API Endpoint

## The Problem

Currently, **there is no REST API endpoint to trigger firmware updates** for charge stations. This creates a limitation for:

1. **End-to-end testing** - Tests can't automatically trigger updates
2. **External systems** - Can't integrate firmware updates via HTTP API
3. **Manual operations** - Operators can't trigger updates from a web interface or CLI tools
4. **Automation** - Can't schedule or batch firmware updates

## What Exists Now

### ✅ Backend Infrastructure (Complete)

All the OCPP message handling is implemented:

```
manager/
├── ocpp/
│   ├── ocpp201/
│   │   ├── update_firmware_request.go      ✓ Request types
│   │   └── update_firmware_response.go     ✓ Response types
│   └── ocpp16/
│       ├── update_firmware.go              ✓ Request types
│       └── update_firmware_response.go     ✓ Response types
├── handlers/
│   ├── call_maker.go                       ✓ CallMaker to send messages
│   ├── ocpp201/
│   │   └── update_firmware_result.go       ✓ Response handler
│   └── ocpp16/
│       └── update_firmware_result.go       ✓ Response handler
└── transport/
    └── mqtt/
        └── emitter.go                      ✓ MQTT message publishing
```

### ✅ What Works

If you have **direct Go code access**, you can trigger firmware updates:

```go
import (
    "github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/transport/mqtt"
)

// This works, but requires code-level access
emitter := mqtt.NewEmitter(...)
callMaker := ocpp201.NewCallMaker(emitter)

request := &ocpp201.UpdateFirmwareRequestJson{
    RequestId: 12345,
    Firmware: ocpp201.FirmwareType{
        Location:         "https://example.com/firmware.bin",
        RetrieveDateTime: "2026-01-20T15:00:00Z",
    },
}

err := callMaker.Send(ctx, "cs001", request)
```

This is how the Manager internally sends OCPP messages to charge stations.

## What's Missing

### ❌ REST API Endpoint

There is **no HTTP endpoint** like:

```
POST /api/v0/cs/{csId}/firmware
```

Compare this to existing endpoints that DO exist:

#### Existing Endpoints (for reference)

```yaml
# Register a charge station
POST /api/v0/cs/{csId}
Body: {"securityProfile": 1}

# Reconfigure a charge station  
POST /api/v0/cs/{csId}/reconfigure
Body: {"key1": "value1", "key2": "value2"}

# Install certificates
POST /api/v0/cs/{csId}/certificates
Body: {"certificates": [...]}

# Trigger a message
POST /api/v0/cs/{csId}/trigger
Body: {"trigger": "BootNotification"}
```

#### What's Needed

```yaml
# Update firmware (MISSING - needs to be added)
POST /api/v0/cs/{csId}/firmware
Body: {
  "firmwareUrl": "https://example.com/firmware/v1.2.3.bin",
  "retrieveDateTime": "2026-01-20T15:00:00Z",
  "installDateTime": "2026-01-20T15:10:00Z",
  "retries": 3,
  "retryInterval": 60
}
```

## Why This is a Limitation

### 1. E2E Testing

The test we created (`firmware_update_test.go`) can:
- ✅ Start a firmware HTTP server
- ✅ Monitor for status notifications via MQTT
- ✅ Track download progress
- ❌ **Cannot automatically trigger the UpdateFirmware message**

Currently, it requires **manual intervention**:

```bash
# Terminal 1: Run the test (waits)
go test -tags=e2e -v -run TestFirmwareUpdateOCPP201

# Terminal 2: Manually trigger using Python script
./scripts/test-firmware-update.py --cs-id cs001
```

The test times out after 5 minutes if not manually triggered.

### 2. External Integration

Systems that want to trigger firmware updates must either:

**Option A: Write Go code** (not practical for most users)
```go
// Requires importing manager packages and running Go code
callMaker.Send(ctx, chargeStationId, request)
```

**Option B: Publish directly to MQTT** (complex and error-prone)
```bash
# Requires knowing internal MQTT topic structure
mosquitto_pub -t 'cs/out/ocpp201/cs001' -m '{
  "messageType": 2,
  "messageId": "...",
  "action": "UpdateFirmware",
  "payload": {...}
}'
```

**Option C: Wait for API endpoint** (what we need)
```bash
# Simple HTTP call - not yet available
curl -X POST http://localhost:9410/api/v0/cs/cs001/firmware \
  -H "Content-Type: application/json" \
  -d '{
    "firmwareUrl": "https://example.com/firmware.bin",
    "retrieveDateTime": "2026-01-20T15:00:00Z"
  }'
```

### 3. Operational Use

Without the API endpoint:
- ❌ Can't build a web UI for firmware management
- ❌ Can't create CLI tools for operators
- ❌ Can't schedule automated firmware updates
- ❌ Can't integrate with deployment pipelines

## How to Add the Missing Endpoint

Here's what needs to be implemented:

### Step 1: Update API Spec

Add to `manager/api/api-spec.yaml`:

```yaml
paths:
  /cs/{csId}/firmware:
    post:
      summary: "Update charge station firmware"
      description: |
        Triggers a firmware update on the charge station. The charge station
        will download the firmware from the specified URL and install it.
      operationId: "updateChargeStationFirmware"
      parameters:
        - name: "csId"
          in: "path"
          description: "The charge station identifier"
          schema:
            type: "string"
            maxLength: 28
      requestBody:
        required: true
        content:
          "application/json":
            schema:
              $ref: "#/components/schemas/ChargeStationFirmwareUpdate"
      responses:
        "202":
          description: "Firmware update initiated"
        default:
          description: "Unexpected error"
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Status"

components:
  schemas:
    ChargeStationFirmwareUpdate:
      type: object
      required:
        - firmwareUrl
        - retrieveDateTime
      properties:
        firmwareUrl:
          type: string
          format: uri
          description: URL where the firmware can be downloaded
        retrieveDateTime:
          type: string
          format: date-time
          description: Date and time when firmware should be retrieved
        installDateTime:
          type: string
          format: date-time
          description: Date and time when firmware should be installed
        retries:
          type: integer
          minimum: 0
          description: Number of retry attempts if download fails
        retryInterval:
          type: integer
          minimum: 0
          description: Interval in seconds between retry attempts
        signingCertificate:
          type: string
          description: PEM encoded certificate for signature verification
        signature:
          type: string
          description: Base64 encoded firmware signature
```

### Step 2: Generate API Code

```bash
cd manager/api
oapi-codegen -config cfg.yaml api-spec.yaml
```

This generates the interface in `api.gen.go`.

### Step 3: Implement Handler

Add to `manager/api/server.go`:

```go
func (s *Server) UpdateChargeStationFirmware(w http.ResponseWriter, r *http.Request, csId string) {
    req := new(ChargeStationFirmwareUpdate)
    if err := render.Bind(r, req); err != nil {
        _ = render.Render(w, r, ErrInvalidRequest(err))
        return
    }

    // Get charge station runtime details to determine OCPP version
    details, err := s.store.LookupChargeStationRuntimeDetails(r.Context(), csId)
    if err != nil {
        _ = render.Render(w, r, ErrNotFound(err))
        return
    }

    // Create appropriate emitter
    emitter := mqtt.NewEmitter(/* config */)
    
    // Determine OCPP version and create request
    if details.OcppVersion == "2.0.1" {
        callMaker := ocpp201.NewCallMaker(emitter)
        
        request := &ocpp201.UpdateFirmwareRequestJson{
            RequestId: int(time.Now().Unix()),
            Firmware: ocpp201.FirmwareType{
                Location:         req.FirmwareUrl,
                RetrieveDateTime: req.RetrieveDateTime,
                InstallDateTime:  req.InstallDateTime,
            },
        }
        
        if req.Retries != nil {
            request.Retries = req.Retries
        }
        if req.RetryInterval != nil {
            request.RetryInterval = req.RetryInterval
        }
        
        err = callMaker.Send(r.Context(), csId, request)
    } else {
        // OCPP 1.6 handling
        callMaker := ocpp16.NewCallMaker(emitter)
        // ... similar logic for OCPP 1.6
    }

    if err != nil {
        _ = render.Render(w, r, ErrInternalError(err))
        return
    }

    w.WriteHeader(http.StatusAccepted)
}
```

### Step 4: Wire Up Dependencies

The Server struct needs access to the MQTT emitter:

```go
type Server struct {
    store   store.Engine
    clock   clock.PassiveClock
    swagger *openapi3.T
    ocpi    ocpi.Api
    emitter transport.Emitter  // Add this
}

func NewServer(engine store.Engine, clock clock.PassiveClock, ocpi ocpi.Api, emitter transport.Emitter) (*Server, error) {
    // ...
    return &Server{
        store:   engine,
        clock:   clock,
        ocpi:    ocpi,
        emitter: emitter,  // Store emitter
        swagger: swagger,
    }, nil
}
```

### Step 5: Update main.go

Pass emitter when creating the API server:

```go
// In manager/main.go
apiServer, err := api.NewServer(engine, clock, ocpiApi, emitter)
```

## Workarounds Until Endpoint Exists

### Option 1: Python Test Script

Use the provided test script which handles the MQTT messaging:

```bash
./scripts/test-firmware-update.py --cs-id cs001 --firmware-url http://example.com/firmware.bin
```

### Option 2: Direct MQTT Publishing

If you have MQTT access:

```bash
# Create the OCPP message
cat > update_firmware.json << EOF
{
  "messageType": 2,
  "messageId": "$(uuidgen)",
  "action": "UpdateFirmware",
  "payload": {
    "requestId": 12345,
    "firmware": {
      "location": "http://example.com/firmware.bin",
      "retrieveDateTime": "2026-01-20T15:00:00Z"
    }
  }
}
EOF

# Publish to MQTT
mosquitto_pub -h localhost -p 1883 \
  -t "cs/out/ocpp201/cs001" \
  -f update_firmware.json
```

### Option 3: Custom Go Program

Write a small Go program:

```go
package main

import (
    "context"
    "net/url"
    "github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/transport/mqtt"
)

func main() {
    brokerUrl, _ := url.Parse("mqtt://localhost:1883/")
    emitter := mqtt.NewEmitter(mqtt.WithMqttBrokerUrls([]*url.URL{brokerUrl}))
    callMaker := ocpp201.NewCallMaker(emitter)
    
    request := &ocpp201.UpdateFirmwareRequestJson{...}
    err := callMaker.Send(context.Background(), "cs001", request)
    // handle error
}
```

## Summary

**Current State:**
- ✅ All OCPP message types implemented
- ✅ CallMaker can send UpdateFirmware messages
- ✅ Handlers process responses
- ✅ Status tracking works
- ❌ **No REST API endpoint to trigger updates**

**Impact:**
- E2E tests require manual triggering
- External systems can't easily integrate
- Requires code-level access or MQTT knowledge

**Solution:**
Add `POST /api/v0/cs/{csId}/firmware` endpoint following the pattern of existing endpoints like `/cs/{csId}/certificates` and `/cs/{csId}/trigger`.

**Effort Required:**
- Update OpenAPI spec (15 minutes)
- Implement handler (30 minutes)
- Wire up dependencies (15 minutes)
- Add tests (30 minutes)
- **Total: ~1.5 hours**

This is a straightforward addition that follows established patterns in the codebase. Once implemented, firmware updates will be as easy as any other charge station operation!
