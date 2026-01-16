#!/bin/bash

# SPDX-License-Identifier: Apache-2.0

# Test script for firmware update functionality
# This script tests the UpdateFirmware OCPP message flow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR=$(dirname "$(readlink -f "$0" 2>/dev/null || realpath "$0")")
CSMS_API_URL="${CSMS_API_URL:-http://localhost:9410/api/v0}"
CHARGE_STATION_ID="${CHARGE_STATION_ID:-cs001}"
FIRMWARE_SERVER_PORT="${FIRMWARE_SERVER_PORT:-8888}"
FIRMWARE_FILE="test_firmware_v1.2.3.bin"
OCPP_VERSION="${OCPP_VERSION:-2.0.1}"

# Temporary directory for test artifacts
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create a fake firmware file for testing
create_test_firmware() {
    log_info "Creating test firmware file..."
    dd if=/dev/urandom of="$TEST_DIR/$FIRMWARE_FILE" bs=1024 count=1024 2>/dev/null
    log_info "Test firmware created: $TEST_DIR/$FIRMWARE_FILE (1MB)"
}

# Start a simple HTTP server to serve the firmware file
start_firmware_server() {
    log_info "Starting firmware HTTP server on port $FIRMWARE_SERVER_PORT..."
    cd "$TEST_DIR"
    python3 -m http.server $FIRMWARE_SERVER_PORT > /dev/null 2>&1 &
    FIRMWARE_SERVER_PID=$!
    cd - > /dev/null
    sleep 2
    log_info "Firmware server started with PID $FIRMWARE_SERVER_PID"
}

# Stop the firmware server
stop_firmware_server() {
    if [ ! -z "$FIRMWARE_SERVER_PID" ]; then
        log_info "Stopping firmware server..."
        kill $FIRMWARE_SERVER_PID 2>/dev/null || true
        wait $FIRMWARE_SERVER_PID 2>/dev/null || true
    fi
}

# Register the charge station if not already registered
register_charge_station() {
    log_info "Registering charge station: $CHARGE_STATION_ID"
    
    curl -s -X POST "$CSMS_API_URL/cs/$CHARGE_STATION_ID" \
        -H "Content-Type: application/json" \
        -d '{
            "securityProfile": 1,
            "base64SHA256Password": "test123",
            "invalidUsernameAllowed": true
        }' > /dev/null 2>&1 || log_warn "Charge station may already be registered"
    
    log_info "Charge station registered"
}

# Send firmware update using OCPP 2.0.1
send_firmware_update_ocpp201() {
    local firmware_url="http://host.docker.internal:$FIRMWARE_SERVER_PORT/$FIRMWARE_FILE"
    local retrieve_time=$(date -u -d '+2 minutes' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -v+2M '+%Y-%m-%dT%H:%M:%SZ')
    local install_time=$(date -u -d '+5 minutes' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -v+5M '+%Y-%m-%dT%H:%M:%SZ')
    
    log_info "Sending OCPP 2.0.1 UpdateFirmware request..."
    log_info "Firmware URL: $firmware_url"
    log_info "Retrieve time: $retrieve_time"
    log_info "Install time: $install_time"
    
    # Note: This assumes you have a way to send OCPP messages directly
    # In a real scenario, you'd need to integrate with the CSMS API
    cat > "$TEST_DIR/update_firmware_request.json" << EOF
{
    "requestId": 12345,
    "firmware": {
        "location": "$firmware_url",
        "retrieveDateTime": "$retrieve_time",
        "installDateTime": "$install_time"
    },
    "retries": 3,
    "retryInterval": 60
}
EOF
    
    log_info "Update firmware request saved to: $TEST_DIR/update_firmware_request.json"
    cat "$TEST_DIR/update_firmware_request.json"
}

# Send firmware update using OCPP 1.6
send_firmware_update_ocpp16() {
    local firmware_url="http://host.docker.internal:$FIRMWARE_SERVER_PORT/$FIRMWARE_FILE"
    local retrieve_date=$(date -u -d '+2 minutes' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -v+2M '+%Y-%m-%dT%H:%M:%SZ')
    
    log_info "Sending OCPP 1.6 UpdateFirmware request..."
    log_info "Firmware URL: $firmware_url"
    log_info "Retrieve date: $retrieve_date"
    
    cat > "$TEST_DIR/update_firmware_request.json" << EOF
{
    "location": "$firmware_url",
    "retrieveDate": "$retrieve_date",
    "retries": 3,
    "retryInterval": 60
}
EOF
    
    log_info "Update firmware request saved to: $TEST_DIR/update_firmware_request.json"
    cat "$TEST_DIR/update_firmware_request.json"
}

# Monitor firmware status notifications (requires access to logs or database)
monitor_firmware_status() {
    log_info "Monitoring for firmware status notifications..."
    log_info "Expected status progression:"
    log_info "  1. Idle"
    log_info "  2. Downloading"
    log_info "  3. Downloaded"
    log_info "  4. Installing"
    log_info "  5. Installed"
    log_info ""
    log_info "Or error states: DownloadFailed, InstallationFailed, InvalidSignature, etc."
    log_info ""
    log_warn "You should monitor the CSMS logs for FirmwareStatusNotification messages"
    log_warn "Example: docker logs -f maeve-csms-manager-1 | grep -i firmware"
}

# Main test flow
main() {
    log_info "========================================"
    log_info "Firmware Update Test Script"
    log_info "========================================"
    log_info "CSMS API URL: $CSMS_API_URL"
    log_info "Charge Station ID: $CHARGE_STATION_ID"
    log_info "OCPP Version: $OCPP_VERSION"
    log_info "========================================"
    echo ""
    
    # Create test firmware
    create_test_firmware
    
    # Start firmware server
    start_firmware_server
    trap stop_firmware_server EXIT
    
    # Register charge station
    register_charge_station
    
    # Send firmware update based on OCPP version
    if [ "$OCPP_VERSION" = "2.0.1" ]; then
        send_firmware_update_ocpp201
    elif [ "$OCPP_VERSION" = "1.6" ]; then
        send_firmware_update_ocpp16
    else
        log_error "Unsupported OCPP version: $OCPP_VERSION"
        exit 1
    fi
    
    echo ""
    log_info "========================================"
    log_info "Next Steps:"
    log_info "========================================"
    log_info "1. The firmware server is running at: http://localhost:$FIRMWARE_SERVER_PORT/$FIRMWARE_FILE"
    log_info "2. You need to send the UpdateFirmware OCPP message to the charge station"
    log_info "3. Request payload is in: $TEST_DIR/update_firmware_request.json"
    log_info ""
    log_info "To send the request programmatically, use the CallMaker in your Go code:"
    log_info "   callMaker := ocpp201.NewCallMaker(emitter)"
    log_info "   callMaker.Send(ctx, \"$CHARGE_STATION_ID\", request)"
    log_info ""
    log_info "4. Monitor firmware status with:"
    log_info "   docker logs -f maeve-csms-manager-1 | grep -i firmware"
    log_info ""
    log_warn "Press Ctrl+C to stop the firmware server"
    
    # Keep the server running
    wait $FIRMWARE_SERVER_PID
}

# Handle script interruption
trap stop_firmware_server INT TERM

# Run main
main
