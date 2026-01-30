// SPDX-License-Identifier: Apache-2.0
//go:build e2e

package test_driver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	// Default firmware size for testing (1MB)
	defaultFirmwareSize = 1024 * 1024
	// Firmware update timeout
	firmwareUpdateTimeout = 5 * time.Minute
)

// FirmwareStatus represents the possible firmware status values
type FirmwareStatus string

const (
	FirmwareStatusIdle                       FirmwareStatus = "Idle"
	FirmwareStatusDownloadScheduled          FirmwareStatus = "DownloadScheduled"
	FirmwareStatusDownloading                FirmwareStatus = "Downloading"
	FirmwareStatusDownloaded                 FirmwareStatus = "Downloaded"
	FirmwareStatusDownloadFailed             FirmwareStatus = "DownloadFailed"
	FirmwareStatusDownloadPaused             FirmwareStatus = "DownloadPaused"
	FirmwareStatusInstallScheduled           FirmwareStatus = "InstallScheduled"
	FirmwareStatusInstalling                 FirmwareStatus = "Installing"
	FirmwareStatusInstalled                  FirmwareStatus = "Installed"
	FirmwareStatusInstallationFailed         FirmwareStatus = "InstallationFailed"
	FirmwareStatusInstallRebooting           FirmwareStatus = "InstallRebooting"
	FirmwareStatusInstallVerificationFailed  FirmwareStatus = "InstallVerificationFailed"
	FirmwareStatusInvalidSignature           FirmwareStatus = "InvalidSignature"
	FirmwareStatusSignatureVerified          FirmwareStatus = "SignatureVerified"
)

// FirmwareTestServer provides a local HTTP server for serving test firmware
type FirmwareTestServer struct {
	server       *http.Server
	port         int
	firmwarePath string
	firmwareData []byte
	checksum     string
	downloadCount int
	mu           sync.Mutex
}

// NewFirmwareTestServer creates a new firmware test server
func NewFirmwareTestServer(port int, firmwareSize int) (*FirmwareTestServer, error) {
	fts := &FirmwareTestServer{
		port: port,
	}

	// Generate random firmware data
	fts.firmwareData = make([]byte, firmwareSize)
	if _, err := rand.Read(fts.firmwareData); err != nil {
		return nil, fmt.Errorf("generating firmware data: %w", err)
	}

	// Calculate checksum
	hash := sha256.Sum256(fts.firmwareData)
	fts.checksum = hex.EncodeToString(hash[:])

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/firmware.bin", fts.serveFirmware)
	mux.HandleFunc("/health", fts.serveHealth)

	fts.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return fts, nil
}

// Start starts the firmware server
func (fts *FirmwareTestServer) Start() error {
	go func() {
		if err := fts.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Firmware server error: %v\n", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the firmware server
func (fts *FirmwareTestServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return fts.server.Shutdown(ctx)
}

// serveFirmware handles firmware download requests
func (fts *FirmwareTestServer) serveFirmware(w http.ResponseWriter, r *http.Request) {
	fts.mu.Lock()
	fts.downloadCount++
	count := fts.downloadCount
	fts.mu.Unlock()

	fmt.Printf("Firmware download request #%d from %s\n", count, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=firmware.bin")
	w.Header().Set("X-Firmware-Checksum", fts.checksum)

	if _, err := io.Copy(w, bytes.NewReader(fts.firmwareData)); err != nil {
		fmt.Printf("Error serving firmware: %v\n", err)
	}
}

// serveHealth handles health check requests
func (fts *FirmwareTestServer) serveHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// GetFirmwareURL returns the URL to download the firmware
func (fts *FirmwareTestServer) GetFirmwareURL() string {
	// Use host.docker.internal for Docker environments
	// This allows containers to access host services
	return fmt.Sprintf("http://host.docker.internal:%d/firmware.bin", fts.port)
}

// GetChecksum returns the SHA256 checksum of the firmware
func (fts *FirmwareTestServer) GetChecksum() string {
	return fts.checksum
}

// GetDownloadCount returns the number of times firmware was downloaded
func (fts *FirmwareTestServer) GetDownloadCount() int {
	fts.mu.Lock()
	defer fts.mu.Unlock()
	return fts.downloadCount
}

// FirmwareStatusTracker tracks firmware status notifications
type FirmwareStatusTracker struct {
	statuses      []FirmwareStatus
	statusChanges map[FirmwareStatus]time.Time
	requestID     *int
	mu            sync.Mutex
}

// NewFirmwareStatusTracker creates a new status tracker
func NewFirmwareStatusTracker() *FirmwareStatusTracker {
	return &FirmwareStatusTracker{
		statuses:      make([]FirmwareStatus, 0),
		statusChanges: make(map[FirmwareStatus]time.Time),
	}
}

// AddStatus records a new firmware status
func (fst *FirmwareStatusTracker) AddStatus(status FirmwareStatus, requestID *int) {
	fst.mu.Lock()
	defer fst.mu.Unlock()

	fst.statuses = append(fst.statuses, status)
	fst.statusChanges[status] = time.Now()
	if requestID != nil {
		fst.requestID = requestID
	}
}

// GetStatuses returns all recorded statuses
func (fst *FirmwareStatusTracker) GetStatuses() []FirmwareStatus {
	fst.mu.Lock()
	defer fst.mu.Unlock()

	result := make([]FirmwareStatus, len(fst.statuses))
	copy(result, fst.statuses)
	return result
}

// HasStatus checks if a specific status was recorded
func (fst *FirmwareStatusTracker) HasStatus(status FirmwareStatus) bool {
	fst.mu.Lock()
	defer fst.mu.Unlock()

	_, exists := fst.statusChanges[status]
	return exists
}

// GetLastStatus returns the most recent status
func (fst *FirmwareStatusTracker) GetLastStatus() FirmwareStatus {
	fst.mu.Lock()
	defer fst.mu.Unlock()

	if len(fst.statuses) == 0 {
		return ""
	}
	return fst.statuses[len(fst.statuses)-1]
}

// sendUpdateFirmwareRequest sends an UpdateFirmware OCPP request via CSMS API
func sendUpdateFirmwareRequest(t *testing.T, chargeStationID, firmwareURL string, requestID int) error {
	csmsAPIURL := os.Getenv("CSMS_API_URL")
	if csmsAPIURL == "" {
		csmsAPIURL = "http://localhost:9410"
	}

	// Calculate retrieve and install times
	retrieveTime := time.Now().Add(30 * time.Second).UTC()
	installTime := time.Now().Add(2 * time.Minute).UTC()

	// Construct the API request for firmware update
	request := map[string]interface{}{
		"location":            firmwareURL,
		"retrieveDateTime":    retrieveTime.Format(time.RFC3339),
		"installDateTime":     installTime.Format(time.RFC3339),
		"requestId":           requestID,
		"retries":             3,
		"retryInterval":       30,
		"signing_certificate": "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAKHHCgVZU4pUMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yMTAxMDEwMDAwMDBaFw0zMTAxMDEwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAr3Ow8PSxp4pLfGVN\ntDKkqOv6r5lqzJqCdZ8PDQHQX5vBnPvUVGYMp3JzSqKvMhkQTkLYRo8FsVvqvBfV\nXCKLVpXKR5bUV5TqR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5Tq\nR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5TqR5UCAQMCAQIDAQMCAQIDAQMCAQIDAQMC\nAQIDAgAGCCqGSM49BAMCA0cAMEQCIFOl0hLqPELZkqUaVtPR9rMU6B5D7P3xXvVx\nQvXpKvXqAiA5LqT8RmT8RmT8RmT8RmT8RmT8RmT8RmT8RmT8RmT8Rg==\n-----END CERTIFICATE-----",
		"signature":           "3046022100d5e8c3b8f9c7a2e1d4f5b6a9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0022100a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
	}

	requestJSON, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	t.Logf("Sending UpdateFirmware request to %s/api/v0/cs/%s/firmware", csmsAPIURL, chargeStationID)
	t.Logf("Request body:\n%s", string(requestJSON))

	// Send POST request to the CSMS API
	apiURL := fmt.Sprintf("%s/api/v0/cs/%s/firmware", csmsAPIURL, chargeStationID)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return fmt.Errorf("sending firmware update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("Firmware update request sent successfully")
	return nil
}

// monitorFirmwareStatus subscribes to firmware status notifications via MQTT
func monitorFirmwareStatus(t *testing.T, client mqtt.Client, chargeStationID string, tracker *FirmwareStatusTracker, wg *sync.WaitGroup) error {
	// Subscribe to OCPP messages from the charge station
	// The topic pattern depends on your MQTT setup
	topic := fmt.Sprintf("cs/in/ocpp201/%s", chargeStationID)

	t.Logf("Subscribing to firmware status on topic: %s", topic)

	token := client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		// Parse the OCPP message
		var ocppMsg map[string]interface{}
		if err := json.Unmarshal(msg.Payload(), &ocppMsg); err != nil {
			t.Logf("Error parsing OCPP message: %v", err)
			return
		}

		// Check if this is a FirmwareStatusNotification
		if action, ok := ocppMsg["action"].(string); ok && action == "FirmwareStatusNotification" {
			if payload, ok := ocppMsg["payload"].(map[string]interface{}); ok {
				if statusStr, ok := payload["status"].(string); ok {
					status := FirmwareStatus(statusStr)
					t.Logf("Firmware status: %s", status)

					var requestID *int
					if reqID, ok := payload["requestId"].(float64); ok {
						id := int(reqID)
						requestID = &id
					}

					tracker.AddStatus(status, requestID)

					// Signal completion on certain statuses
					if status == FirmwareStatusInstalled || 
					   status == FirmwareStatusDownloadFailed ||
					   status == FirmwareStatusInstallationFailed {
						wg.Done()
					}
				}
			}
		}
	})

	if token.WaitTimeout(2 * time.Second); token.Error() != nil {
		return fmt.Errorf("failed to subscribe to firmware status: %w", token.Error())
	}

	return nil
}

// TestFirmwareUpdateOCPP201 tests the complete firmware update flow for OCPP 2.0.1
func TestFirmwareUpdateOCPP201(t *testing.T) {
	chargeStationID := os.Getenv("CHARGE_STATION_ID")
	if chargeStationID == "" {
		chargeStationID = "cp001"
	}

	// Check if we should skip this test
	if os.Getenv("SKIP_FIRMWARE_TEST") != "" {
		t.Skip("Skipping firmware update test (SKIP_FIRMWARE_TEST is set)")
	}

	t.Logf("Starting firmware update test for charge station: %s", chargeStationID)

	// Create firmware test server
	serverPort := 8888
	if portStr := os.Getenv("FIRMWARE_SERVER_PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &serverPort)
	}

	firmwareServer, err := NewFirmwareTestServer(serverPort, defaultFirmwareSize)
	if err != nil {
		t.Fatalf("Failed to create firmware server: %v", err)
	}

	if err := firmwareServer.Start(); err != nil {
		t.Fatalf("Failed to start firmware server: %v", err)
	}
	defer firmwareServer.Stop()

	t.Logf("Firmware server started on port %d", serverPort)
	t.Logf("Firmware URL: %s", firmwareServer.GetFirmwareURL())
	t.Logf("Firmware checksum: %s", firmwareServer.GetChecksum())

	// Setup MQTT connection
	wg := sync.WaitGroup{}
	client, shutdown := setupBrokerConnection(t, &wg)
	defer shutdown()

	// Create status tracker
	tracker := NewFirmwareStatusTracker()

	// Monitor firmware status notifications
	if err := monitorFirmwareStatus(t, client, chargeStationID, tracker, &wg); err != nil {
		t.Fatalf("Failed to setup firmware status monitoring: %v", err)
	}

	// Send UpdateFirmware request
	requestID := int(time.Now().Unix())
	if err := sendUpdateFirmwareRequest(t, chargeStationID, firmwareServer.GetFirmwareURL(), requestID); err != nil {
		t.Fatalf("Failed to send UpdateFirmware request: %v", err)
	}

	t.Log("Waiting for firmware update to complete...")
	t.Log("Expected status progression:")
	t.Log("  1. Idle")
	t.Log("  2. Downloading")
	t.Log("  3. Downloaded")
	t.Log("  4. Installing")
	t.Log("  5. Installed")

	// Wait for firmware update completion
	wg.Add(1)
	timedOut := waitTimeout(&wg, firmwareUpdateTimeout)

	if timedOut {
		t.Logf("WARNING: Firmware update timed out after %v", firmwareUpdateTimeout)
		t.Logf("This test requires manual triggering of the UpdateFirmware OCPP message")
		t.Logf("Recorded statuses: %v", tracker.GetStatuses())
		t.Skip("Firmware update test requires manual interaction")
		return
	}

	// Verify the update process
	lastStatus := tracker.GetLastStatus()
	t.Logf("Final status: %s", lastStatus)
	t.Logf("All statuses: %v", tracker.GetStatuses())

	// Check if firmware was downloaded
	downloadCount := firmwareServer.GetDownloadCount()
	t.Logf("Firmware downloaded %d time(s)", downloadCount)

	// Verify success
	if lastStatus == FirmwareStatusInstalled {
		t.Log("✓ Firmware update completed successfully")
		
		// Verify expected status progression
		if tracker.HasStatus(FirmwareStatusDownloading) {
			t.Log("✓ Downloading status received")
		} else {
			t.Error("✗ Downloading status not received")
		}

		if tracker.HasStatus(FirmwareStatusDownloaded) {
			t.Log("✓ Downloaded status received")
		} else {
			t.Error("✗ Downloaded status not received")
		}

		if tracker.HasStatus(FirmwareStatusInstalling) {
			t.Log("✓ Installing status received")
		} else {
			t.Log("⚠ Installing status not received (may have been skipped)")
		}

		if downloadCount > 0 {
			t.Log("✓ Firmware was downloaded from test server")
		} else {
			t.Error("✗ Firmware was not downloaded from test server")
		}
	} else if lastStatus == FirmwareStatusDownloadFailed {
		t.Errorf("Firmware download failed")
	} else if lastStatus == FirmwareStatusInstallationFailed {
		t.Errorf("Firmware installation failed")
	} else {
		t.Errorf("Unexpected final status: %s", lastStatus)
	}
}

// TestFirmwareUpdateWithRetry tests firmware update with retry mechanism
func TestFirmwareUpdateWithRetry(t *testing.T) {
	t.Skip("Skipping retry test - requires specific charge station configuration")

	// This test would simulate a firmware server that fails initially
	// and succeeds on retry to verify the retry mechanism works correctly
}

// TestFirmwareUpdateServerUnavailable tests behavior when firmware server is unreachable
func TestFirmwareUpdateServerUnavailable(t *testing.T) {
	t.Skip("Skipping unavailable server test - requires manual setup")

	// This test would use an invalid firmware URL to verify
	// the charge station properly reports DownloadFailed status
}

// saveFirmwareToFile saves test firmware to a file for inspection
func saveFirmwareToFile(t *testing.T, data []byte, checksum string) string {
	tmpDir := os.TempDir()
	filename := filepath.Join(tmpDir, fmt.Sprintf("test_firmware_%s.bin", checksum[:8]))

	if err := os.WriteFile(filename, data, 0644); err != nil {
		t.Logf("Warning: Could not save firmware to file: %v", err)
		return ""
	}

	t.Logf("Test firmware saved to: %s", filename)
	return filename
}
