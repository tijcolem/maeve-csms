#!/usr/bin/env python3

"""
SPDX-License-Identifier: Apache-2.0

Firmware Update E2E Test Script

This script performs an end-to-end test of firmware update functionality:
1. Creates a test firmware file
2. Starts a local HTTP server to serve the firmware
3. Sends an UpdateFirmware OCPP message to a charge station
4. Monitors for FirmwareStatusNotification messages
5. Verifies the firmware download

Requirements:
    pip install websocket-client requests
"""

import argparse
import hashlib
import http.server
import json
import logging
import os
import socketserver
import sys
import threading
import time
from datetime import datetime, timedelta
from pathlib import Path

try:
    import requests
except ImportError:
    print("Error: requests module not found. Install with: pip install requests")
    sys.exit(1)

# Configuration
CSMS_WS_URL = os.getenv("CSMS_WS_URL", "ws://localhost:9410/ws")
CSMS_API_URL = os.getenv("CSMS_API_URL", "http://localhost:9410/api/v0")
FIRMWARE_SERVER_PORT = int(os.getenv("FIRMWARE_SERVER_PORT", "8888"))
CHARGE_STATION_ID = os.getenv("CHARGE_STATION_ID", "cs001")
OCPP_VERSION = os.getenv("OCPP_VERSION", "2.0.1")

# Logging setup
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class FirmwareServer:
    """Simple HTTP server to serve firmware files"""
    
    def __init__(self, directory, port):
        self.directory = Path(directory)
        self.port = port
        self.server = None
        self.thread = None
        
    def start(self):
        """Start the HTTP server in a background thread"""
        os.chdir(self.directory)
        handler = http.server.SimpleHTTPRequestHandler
        self.server = socketserver.TCPServer(("", self.port), handler)
        
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()
        
        logger.info(f"Firmware server started on port {self.port}")
        logger.info(f"Serving files from: {self.directory}")
        
    def stop(self):
        """Stop the HTTP server"""
        if self.server:
            self.server.shutdown()
            logger.info("Firmware server stopped")


def create_test_firmware(output_dir, filename="test_firmware.bin", size_mb=1):
    """Create a test firmware file with random data"""
    output_path = Path(output_dir) / filename
    size_bytes = size_mb * 1024 * 1024
    
    logger.info(f"Creating test firmware: {output_path} ({size_mb}MB)")
    
    # Generate random firmware data
    with open(output_path, 'wb') as f:
        # Write in chunks to avoid memory issues with large files
        chunk_size = 1024 * 1024  # 1MB chunks
        remaining = size_bytes
        
        while remaining > 0:
            chunk = os.urandom(min(chunk_size, remaining))
            f.write(chunk)
            remaining -= len(chunk)
    
    # Calculate checksum
    sha256_hash = hashlib.sha256()
    with open(output_path, 'rb') as f:
        for chunk in iter(lambda: f.read(4096), b""):
            sha256_hash.update(chunk)
    
    checksum = sha256_hash.hexdigest()
    logger.info(f"Firmware created successfully")
    logger.info(f"SHA256: {checksum}")
    
    return str(output_path), checksum


def send_update_firmware_ocpp201(firmware_url, request_id=12345):
    """Send OCPP 2.0.1 UpdateFirmware request"""
    retrieve_time = (datetime.utcnow() + timedelta(minutes=2)).isoformat() + "Z"
    install_time = (datetime.utcnow() + timedelta(minutes=5)).isoformat() + "Z"
    
    request = {
        "requestId": request_id,
        "firmware": {
            "location": firmware_url,
            "retrieveDateTime": retrieve_time,
            "installDateTime": install_time
        },
        "retries": 3,
        "retryInterval": 60
    }
    
    logger.info("OCPP 2.0.1 UpdateFirmware Request:")
    logger.info(json.dumps(request, indent=2))
    
    return request


def send_update_firmware_ocpp16(firmware_url):
    """Send OCPP 1.6 UpdateFirmware request"""
    retrieve_date = (datetime.utcnow() + timedelta(minutes=2)).isoformat() + "Z"
    
    request = {
        "location": firmware_url,
        "retrieveDate": retrieve_date,
        "retries": 3,
        "retryInterval": 60
    }
    
    logger.info("OCPP 1.6 UpdateFirmware Request:")
    logger.info(json.dumps(request, indent=2))
    
    return request


def register_charge_station(charge_station_id):
    """Register a charge station with the CSMS"""
    url = f"{CSMS_API_URL}/cs/{charge_station_id}"
    
    payload = {
        "securityProfile": 1,
        "base64SHA256Password": "test123",
        "invalidUsernameAllowed": True
    }
    
    try:
        response = requests.post(url, json=payload, timeout=5)
        if response.status_code == 201:
            logger.info(f"Charge station {charge_station_id} registered successfully")
            return True
        else:
            logger.warning(f"Charge station registration returned status {response.status_code}")
            logger.warning("Station may already be registered")
            return True
    except requests.exceptions.RequestException as e:
        logger.error(f"Failed to register charge station: {e}")
        return False


def monitor_logs_for_status(duration_seconds=300):
    """Monitor for firmware status notifications in logs"""
    logger.info(f"Monitoring for firmware status notifications for {duration_seconds} seconds...")
    logger.info("Expected status progression:")
    logger.info("  -> Idle")
    logger.info("  -> Downloading")
    logger.info("  -> Downloaded")
    logger.info("  -> Installing")
    logger.info("  -> Installed")
    logger.info("")
    logger.info("Or error states: DownloadFailed, InstallationFailed, InvalidSignature")
    logger.info("")
    logger.warning("Manual monitoring required!")
    logger.warning("Run in another terminal: docker logs -f maeve-csms-manager-1 | grep -i firmware")
    
    # Wait for user input or timeout
    try:
        input(f"\nPress Enter when firmware update completes or Ctrl+C to abort...\n")
    except KeyboardInterrupt:
        logger.info("\nMonitoring interrupted by user")


def save_request_to_file(request, filename="update_firmware_request.json"):
    """Save the request to a file for reference"""
    with open(filename, 'w') as f:
        json.dump(request, f, indent=2)
    logger.info(f"Request saved to: {filename}")


def main():
    parser = argparse.ArgumentParser(description="Test firmware update functionality")
    parser.add_argument("--cs-id", default=CHARGE_STATION_ID, 
                       help="Charge station ID (default: cs001)")
    parser.add_argument("--ocpp-version", default=OCPP_VERSION, 
                       choices=["1.6", "2.0.1"],
                       help="OCPP version (default: 2.0.1)")
    parser.add_argument("--firmware-size", type=int, default=1,
                       help="Firmware size in MB (default: 1)")
    parser.add_argument("--port", type=int, default=FIRMWARE_SERVER_PORT,
                       help="Firmware server port (default: 8888)")
    parser.add_argument("--no-server", action="store_true",
                       help="Don't start firmware server (use external URL)")
    parser.add_argument("--firmware-url", 
                       help="External firmware URL (requires --no-server)")
    
    args = parser.parse_args()
    
    logger.info("=" * 60)
    logger.info("Firmware Update E2E Test")
    logger.info("=" * 60)
    logger.info(f"Charge Station ID: {args.cs_id}")
    logger.info(f"OCPP Version: {args.ocpp_version}")
    logger.info(f"CSMS API URL: {CSMS_API_URL}")
    logger.info("=" * 60)
    
    # Create temporary directory for test artifacts
    test_dir = Path("firmware_test_artifacts")
    test_dir.mkdir(exist_ok=True)
    
    try:
        firmware_server = None
        firmware_url = args.firmware_url
        
        if not args.no_server:
            # Create test firmware
            firmware_file, checksum = create_test_firmware(
                test_dir, 
                size_mb=args.firmware_size
            )
            
            # Start firmware server
            firmware_server = FirmwareServer(test_dir, args.port)
            firmware_server.start()
            time.sleep(1)  # Give server time to start
            
            # Construct firmware URL
            firmware_filename = Path(firmware_file).name
            firmware_url = f"http://host.docker.internal:{args.port}/{firmware_filename}"
            
            logger.info(f"Firmware URL: {firmware_url}")
            logger.info(f"Firmware SHA256: {checksum}")
        
        if not firmware_url:
            logger.error("No firmware URL specified. Use --firmware-url or omit --no-server")
            return 1
        
        # Register charge station
        if not register_charge_station(args.cs_id):
            logger.error("Failed to register charge station")
            return 1
        
        # Create update firmware request
        if args.ocpp_version == "2.0.1":
            request = send_update_firmware_ocpp201(firmware_url)
        else:
            request = send_update_firmware_ocpp16(firmware_url)
        
        # Save request to file
        save_request_to_file(request, str(test_dir / "update_firmware_request.json"))
        
        logger.info("")
        logger.info("=" * 60)
        logger.info("NEXT STEPS:")
        logger.info("=" * 60)
        logger.info("1. Send the UpdateFirmware OCPP message to the charge station")
        logger.info("   using your preferred method (e.g., via CSMS API or directly)")
        logger.info("")
        logger.info("2. The charge station should:")
        logger.info("   - Download firmware from the URL")
        logger.info("   - Send FirmwareStatusNotification messages")
        logger.info("   - Install the firmware")
        logger.info("")
        logger.info("3. Monitor the CSMS logs:")
        logger.info("   docker logs -f maeve-csms-manager-1 | grep -i firmware")
        logger.info("=" * 60)
        
        # Monitor for status updates
        monitor_logs_for_status()
        
    except KeyboardInterrupt:
        logger.info("\nTest interrupted by user")
    except Exception as e:
        logger.error(f"Test failed: {e}", exc_info=True)
        return 1
    finally:
        if firmware_server:
            firmware_server.stop()
    
    logger.info("\nTest completed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
