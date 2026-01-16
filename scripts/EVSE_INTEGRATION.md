# How Firmware Update Integrates with Connected EVSEs

This document explains the complete end-to-end flow of how the UpdateFirmware functionality integrates with actual connected EVSEs (charge stations).

## Architecture Overview

MaEVe CSMS uses a distributed architecture with these key components:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        MaEVe CSMS System                                │
│                                                                         │
│  ┌──────────────┐         ┌──────────────┐        ┌─────────────────┐ │
│  │   Manager    │◄───────►│ MQTT Broker  │◄──────►│    Gateway      │ │
│  │  (Backend)   │         │  (Mosquitto) │        │  (WebSocket)    │ │
│  │              │         │              │        │                 │ │
│  │  Stateless   │         │   Message    │        │   Stateful      │ │
│  │   Service    │         │    Broker    │        │   Connection    │ │
│  └──────────────┘         └──────────────┘        └─────────────────┘ │
│        ▲                                                    ▲           │
│        │                                                    │           │
└────────┼────────────────────────────────────────────────────┼───────────┘
         │                                                    │
         │ HTTP API                                           │ WebSocket
         │                                                    │ (OCPP 1.6/2.0.1)
         │                                                    │
    ┌────▼─────┐                                        ┌────▼──────┐
    │   Your   │                                        │   EVSE    │
    │   Code   │                                        │  (Charge  │
    │   or     │                                        │  Station) │
    │   API    │                                        │           │
    └──────────┘                                        └───────────┘
```

## Complete Firmware Update Flow

### Step 1: EVSE Connects to CSMS

```
EVSE                    Gateway                MQTT Broker          Manager
  │                        │                        │                  │
  ├─[WebSocket Connect]──>│                        │                  │
  │  ws://gateway/ws/cs001 │                        │                  │
  │                        │                        │                  │
  │                        ├─[Fetch Auth]──────────┼─────────────────>│
  │                        │<─[Auth Details]────────┼──────────────────┤
  │                        │                        │                  │
  │<──[Connection OK]──────┤                        │                  │
  │                        │                        │                  │
  │                        ├─[Subscribe]──────────>│                  │
  │                        │  cs/out/ocpp201/cs001  │                  │
  │                        │                        │                  │
```

**Details:**
- EVSE establishes WebSocket connection to Gateway at `/ws/<charge-station-id>`
- Gateway authenticates EVSE using credentials from Manager API
- Gateway subscribes to MQTT topic for outgoing messages: `cs/out/<ocpp-version>/<cs-id>`
- Connection is kept alive with heartbeats

### Step 2: Send UpdateFirmware Request

You can trigger an update in several ways:

#### Option A: Direct Go Code (Programmatic)

```go
import (
    "github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/transport/mqtt"
)

// Create MQTT emitter
emitter := mqtt.NewEmitter(
    mqtt.WithMqttBrokerUrls([]*url.URL{brokerUrl}),
    mqtt.WithMqttPrefix("cs"),
)

// Create call maker
callMaker := ocpp201.NewCallMaker(emitter)

// Send update firmware request
request := &ocpp201.UpdateFirmwareRequestJson{
    RequestId: 12345,
    Firmware: ocpp201.FirmwareType{
        Location:         "https://firmware.example.com/v1.2.3.bin",
        RetrieveDateTime: "2026-01-16T15:00:00Z",
        InstallDateTime:  &installTime,
    },
}

err := callMaker.Send(ctx, "cs001", request)
```

#### Option B: Test Scripts

```bash
# Run the Python test script
./scripts/test-firmware-update.py --cs-id cs001 --ocpp-version 2.0.1

# Or the bash script
./scripts/test-firmware-update.sh
```

### Step 3: Message Flow Through the System

```
Your Code              Manager               MQTT Broker           Gateway              EVSE
   │                      │                       │                    │                   │
   ├─[callMaker.Send]────>│                       │                    │                   │
   │                      │                       │                    │                   │
   │                      ├─[Marshal JSON]        │                    │                   │
   │                      │                       │                    │                   │
   │                      ├─[Publish MQTT]───────>│                    │                   │
   │                      │  cs/out/ocpp201/cs001 │                    │                   │
   │                      │                       │                    │                   │
   │                      │                       ├─[Message]─────────>│                   │
   │                      │                       │                    │                   │
   │                      │                       │                    ├─[Convert]         │
   │                      │                       │                    │  to OCPP JSON     │
   │                      │                       │                    │                   │
   │                      │                       │                    ├─[WebSocket Send]─>│
   │                      │                       │                    │  OCPP Message     │
   │                      │                       │                    │                   │
```

**Details:**
1. **Manager (Your Code)**:
   - Creates UpdateFirmwareRequest with firmware URL and timing
   - CallMaker marshals request to JSON
   - Emitter publishes to MQTT topic: `cs/out/ocpp201/cs001`

2. **MQTT Broker**:
   - Receives message on `out` topic
   - Routes to Gateway that subscribed to this charge station's topic

3. **Gateway**:
   - Receives message from MQTT
   - Converts to OCPP protocol format (JSON over WebSocket)
   - Uses Pipe to manage message state machine
   - Sends via WebSocket to connected EVSE

4. **EVSE**:
   - Receives UpdateFirmware OCPP message
   - Processes the firmware update request

### Step 4: EVSE Responds

```
EVSE               Gateway              MQTT Broker          Manager
  │                   │                      │                  │
  ├─[Response]───────>│                      │                  │
  │  {"status":       │                      │                  │
  │   "Accepted"}     │                      │                  │
  │                   │                      │                  │
  │                   ├─[Convert & Publish]─>│                  │
  │                   │  cs/in/ocpp201/cs001 │                  │
  │                   │                      │                  │
  │                   │                      ├─[Deliver]───────>│
  │                   │                      │                  │
  │                   │                      │                  ├─[Process]
  │                   │                      │                  │  Update-
  │                   │                      │                  │  Firmware
  │                   │                      │                  │  Result
  │                   │                      │                  │  Handler
```

**Details:**
- EVSE sends UpdateFirmwareResponse back via WebSocket
- Gateway publishes to MQTT topic: `cs/in/ocpp201/cs001`
- Manager receives and processes with `UpdateFirmwareResultHandler`
- Handler logs firmware update initiation with telemetry

### Step 5: EVSE Downloads and Installs Firmware

```
EVSE                Firmware Server         Gateway          MQTT Broker      Manager
  │                       │                     │                  │              │
  ├─[Status: Idle]───────┼────────────────────>│                  │              │
  │                       │                     ├─[Publish]───────>│              │
  │                       │                     │                  ├─[Notify]────>│
  │                       │                     │                  │              │
  ├─[HTTP GET]───────────>│                     │                  │              │
  │  firmware.bin         │                     │                  │              │
  │                       │                     │                  │              │
  ├─[Status: Downloading]┼────────────────────>│                  │              │
  │                       │                     ├─[Publish]───────>│              │
  │                       │                     │                  ├─[Notify]────>│
  │                       │                     │                  │              │
  │<─[Firmware Data]──────┤                     │                  │              │
  │                       │                     │                  │              │
  ├─[Status: Downloaded]─┼────────────────────>│                  │              │
  │                       │                     ├─[Publish]───────>│              │
  │                       │                     │                  ├─[Notify]────>│
  │                       │                     │                  │              │
  ├─[Install Firmware]    │                     │                  │              │
  │                       │                     │                  │              │
  ├─[Status: Installing]─┼────────────────────>│                  │              │
  │                       │                     ├─[Publish]───────>│              │
  │                       │                     │                  ├─[Notify]────>│
  │                       │                     │                  │              │
  ├─[Reboot]              │                     │                  │              │
  │                       │                     │                  │              │
  ├─[Reconnect]──────────┼────────────────────>│                  │              │
  │                       │                     │                  │              │
  ├─[Status: Installed]──┼────────────────────>│                  │              │
  │                       │                     ├─[Publish]───────>│              │
  │                       │                     │                  ├─[Notify]────>│
```

**Details:**
- EVSE sends `FirmwareStatusNotification` messages throughout the process
- Each status change flows: EVSE → Gateway → MQTT → Manager
- Manager's `FirmwareStatusNotificationHandler` processes each status
- Statuses tracked: Idle, Downloading, Downloaded, Installing, Installed

## MQTT Topics Used

The system uses specific MQTT topics for routing messages:

### Outgoing Messages (CSMS → EVSE)
```
cs/out/ocpp201/cs001    # OCPP 2.0.1 messages for cs001
cs/out/ocpp16/cs002     # OCPP 1.6 messages for cs002
```

### Incoming Messages (EVSE → CSMS)
```
cs/in/ocpp201/cs001     # Messages from cs001 using OCPP 2.0.1
cs/in/ocpp16/cs002      # Messages from cs002 using OCPP 1.6
```

### Topic Structure
```
<prefix>/<direction>/<ocpp-version>/<charge-station-id>

Where:
- prefix: Configurable (default: "cs")
- direction: "in" or "out"
- ocpp-version: "ocpp16" or "ocpp201"
- charge-station-id: Unique identifier for the charge station
```

## Code Components Involved

### Manager Side (Backend)

1. **CallMaker** ([manager/handlers/call_maker.go](../manager/handlers/call_maker.go))
   - `OcppCallMaker.Send()` - Sends OCPP requests to charge stations

2. **Emitter** ([manager/transport/mqtt/emitter.go](../manager/transport/mqtt/emitter.go))
   - `Emitter.Emit()` - Publishes messages to MQTT broker

3. **Request Types** ([manager/ocpp/ocpp201/update_firmware_request.go](../manager/ocpp/ocpp201/update_firmware_request.go))
   - `UpdateFirmwareRequestJson` - Request structure

4. **Result Handler** ([manager/handlers/ocpp201/update_firmware_result.go](../manager/handlers/ocpp201/update_firmware_result.go))
   - `UpdateFirmwareResultHandler` - Processes EVSE responses

5. **Status Handler** ([manager/handlers/ocpp201/firmware_status_notification.go](../manager/handlers/ocpp201/firmware_status_notification.go))
   - `FirmwareStatusNotificationHandler` - Processes status updates

### Gateway Side (Connection Manager)

1. **WebSocket Handler** ([gateway/server/ws.go](../gateway/server/ws.go))
   - Manages WebSocket connections with EVSEs
   - Converts between OCPP and internal message format

2. **Pipe** ([gateway/pipe/pipe.go](../gateway/pipe/pipe.go))
   - State machine for managing message flow
   - Buffers messages during ongoing calls
   - Handles timeouts

3. **MQTT Integration** 
   - Subscribes to outgoing topics
   - Publishes incoming messages

## Practical Integration Example

Here's a complete example showing how to integrate firmware updates into your application:

### 1. Setup (One-time)

```go
package main

import (
    "context"
    "log"
    "net/url"
    
    "github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
    "github.com/thoughtworks/maeve-csms/manager/transport/mqtt"
)

func setupFirmwareUpdateService() (*ocpp201.OcppCallMaker, error) {
    // Connect to MQTT broker
    brokerUrl, _ := url.Parse("mqtt://localhost:1883/")
    
    emitter := mqtt.NewEmitter(
        mqtt.WithMqttBrokerUrls([]*url.URL{brokerUrl}),
        mqtt.WithMqttPrefix("cs"),
    )
    
    // Create call maker
    callMaker := ocpp201.NewCallMaker(emitter)
    
    return callMaker, nil
}
```

### 2. Trigger Firmware Update

```go
func sendFirmwareUpdate(callMaker *ocpp201.OcppCallMaker, csId, firmwareUrl string) error {
    ctx := context.Background()
    
    retrieveTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
    installTime := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
    
    request := &ocpp201types.UpdateFirmwareRequestJson{
        RequestId: int(time.Now().Unix()),
        Firmware: ocpp201types.FirmwareType{
            Location:         firmwareUrl,
            RetrieveDateTime: retrieveTime,
            InstallDateTime:  &installTime,
        },
    }
    
    log.Printf("Sending firmware update to %s", csId)
    return callMaker.Send(ctx, csId, request)
}
```

### 3. Monitor Status (via logs or database)

```go
// The FirmwareStatusNotification handler automatically processes status updates
// You can monitor these in the CSMS logs:
// docker logs -f maeve-csms-manager-1 | grep firmware
```

## Testing with Real EVSEs

### Prerequisites

1. **EVSE must be connected**:
   ```bash
   # Check if EVSE is connected (look in Gateway logs)
   docker logs maeve-csms-gateway-1 | grep "charge station connected"
   ```

2. **EVSE must be registered**:
   ```bash
   curl -X POST http://localhost:9410/api/v0/cs/cs001 \
     -H "Content-Type: application/json" \
     -d '{"securityProfile": 1}'
   ```

3. **Firmware server must be accessible from EVSE network**:
   - If EVSE is on same Docker network: `http://host.docker.internal:8888/firmware.bin`
   - If EVSE is external: Use public URL or VPN

### Running the Test

```bash
# Start the test firmware server
./scripts/test-firmware-update.py --cs-id cs001

# In another terminal, monitor the logs
docker logs -f maeve-csms-manager-1 | grep -i firmware

# Watch for status progression:
# - firmware_status.status=Downloading
# - firmware_status.status=Downloaded
# - firmware_status.status=Installing
# - firmware_status.status=Installed
```

## Troubleshooting

### Message Not Reaching EVSE

**Check Manager → MQTT:**
```bash
# Subscribe to outgoing topic
docker exec -it maeve-csms-mqtt-1 mosquitto_sub -t 'cs/out/#' -v
```

**Check MQTT → Gateway:**
```bash
# View Gateway logs
docker logs -f maeve-csms-gateway-1
```

**Check Gateway → EVSE:**
```bash
# Look for WebSocket sends in Gateway logs
docker logs maeve-csms-gateway-1 | grep "sending message"
```

### EVSE Response Not Received

**Check EVSE → Gateway:**
```bash
# Look for WebSocket receives
docker logs maeve-csms-gateway-1 | grep "received message"
```

**Check Gateway → MQTT:**
```bash
# Subscribe to incoming topic
docker exec -it maeve-csms-mqtt-1 mosquitto_sub -t 'cs/in/#' -v
```

**Check MQTT → Manager:**
```bash
# View Manager logs for processing
docker logs -f maeve-csms-manager-1 | grep UpdateFirmware
```

## Summary

The firmware update integration works through this flow:

1. **Your Code** calls `CallMaker.Send()` with UpdateFirmware request
2. **Manager** publishes OCPP message to MQTT broker
3. **MQTT Broker** routes message to appropriate Gateway
4. **Gateway** forwards message via WebSocket to connected EVSE
5. **EVSE** downloads firmware and sends status notifications back
6. Status flows reverse: EVSE → Gateway → MQTT → Manager
7. **Manager** logs all status changes with telemetry

This architecture allows:
- **Horizontal scaling** - Multiple Managers can process messages
- **High availability** - Gateways can be load balanced
- **Resilience** - MQTT provides reliable message delivery
- **Stateless Manager** - All state in database, not memory
- **Multiple OCPP versions** - Supports 1.6 and 2.0.1 simultaneously

The key insight is that the Manager doesn't communicate directly with EVSEs - it uses MQTT as an intermediary, and the Gateway maintains the actual WebSocket connections. This separation allows the system to scale efficiently while maintaining connection stability.
