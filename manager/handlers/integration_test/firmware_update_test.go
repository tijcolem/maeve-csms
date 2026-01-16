// SPDX-License-Identifier: Apache-2.0
//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thoughtworks/maeve-csms/manager/handlers/ocpp16"
	"github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
	ocpp16types "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp16"
	ocpp201types "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
	"github.com/thoughtworks/maeve-csms/manager/transport"
)

const (
	testFirmwareSize = 1024 * 1024 // 1MB test firmware
)

// MockEmitter captures emitted messages for testing
type MockEmitter struct {
	LastMessage        *transport.Message
	LastOcppVersion    transport.OcppVersion
	LastChargeStation  string
	EmittedMessages    []*transport.Message
	EmitError          error
}

func (m *MockEmitter) Emit(ctx context.Context, ocppVersion transport.OcppVersion, chargeStationId string, message *transport.Message) error {
	m.LastMessage = message
	m.LastOcppVersion = ocppVersion
	m.LastChargeStation = chargeStationId
	m.EmittedMessages = append(m.EmittedMessages, message)
	return m.EmitError
}

func TestOCPP201UpdateFirmwareRequest(t *testing.T) {
	// Setup
	emitter := &MockEmitter{}
	callMaker := ocpp201.NewCallMaker(emitter)
	
	chargeStationId := "cs001"
	firmwareUrl := "https://example.com/firmware/v1.2.3.bin"
	retrieveTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	installTime := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	requestId := 12345
	
	request := &ocpp201types.UpdateFirmwareRequestJson{
		RequestId: requestId,
		Firmware: ocpp201types.FirmwareType{
			Location:         firmwareUrl,
			RetrieveDateTime: retrieveTime,
			InstallDateTime:  &installTime,
		},
	}
	
	// Execute
	err := callMaker.Send(context.Background(), chargeStationId, request)
	
	// Assert
	require.NoError(t, err)
	assert.NotNil(t, emitter.LastMessage)
	assert.Equal(t, transport.OcppVersion201, emitter.LastOcppVersion)
	assert.Equal(t, chargeStationId, emitter.LastChargeStation)
	assert.Equal(t, "UpdateFirmware", emitter.LastMessage.Action)
	assert.Equal(t, transport.MessageTypeCall, emitter.LastMessage.MessageType)
	
	t.Logf("Successfully sent UpdateFirmware request")
	t.Logf("Message ID: %s", emitter.LastMessage.MessageId)
	t.Logf("Action: %s", emitter.LastMessage.Action)
}

func TestOCPP201UpdateFirmwareWithRetries(t *testing.T) {
	// Setup
	emitter := &MockEmitter{}
	callMaker := ocpp201.NewCallMaker(emitter)
	
	chargeStationId := "cs002"
	firmwareUrl := "https://example.com/firmware/v2.0.0.bin"
	retrieveTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	requestId := 54321
	retries := 5
	retryInterval := 120
	
	request := &ocpp201types.UpdateFirmwareRequestJson{
		RequestId: requestId,
		Firmware: ocpp201types.FirmwareType{
			Location:         firmwareUrl,
			RetrieveDateTime: retrieveTime,
		},
		Retries:       &retries,
		RetryInterval: &retryInterval,
	}
	
	// Execute
	err := callMaker.Send(context.Background(), chargeStationId, request)
	
	// Assert
	require.NoError(t, err)
	assert.NotNil(t, emitter.LastMessage)
	assert.Equal(t, "UpdateFirmware", emitter.LastMessage.Action)
	
	t.Logf("Successfully sent UpdateFirmware request with retries")
	t.Logf("Retries: %d, Retry Interval: %ds", retries, retryInterval)
}

func TestOCPP16UpdateFirmwareRequest(t *testing.T) {
	// Setup
	emitter := &MockEmitter{}
	callMaker := ocpp16.NewCallMaker(emitter)
	
	chargeStationId := "cs003"
	firmwareUrl := "https://example.com/firmware/v1.6.0.bin"
	retrieveDate := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	retries := 3
	retryInterval := 60
	
	request := &ocpp16types.UpdateFirmwareJson{
		Location:      firmwareUrl,
		RetrieveDate:  retrieveDate,
		Retries:       &retries,
		RetryInterval: &retryInterval,
	}
	
	// Execute
	err := callMaker.Send(context.Background(), chargeStationId, request)
	
	// Assert
	require.NoError(t, err)
	assert.NotNil(t, emitter.LastMessage)
	assert.Equal(t, transport.OcppVersion16, emitter.LastOcppVersion)
	assert.Equal(t, chargeStationId, emitter.LastChargeStation)
	assert.Equal(t, "UpdateFirmware", emitter.LastMessage.Action)
	assert.Equal(t, transport.MessageTypeCall, emitter.LastMessage.MessageType)
	
	t.Logf("Successfully sent OCPP 1.6 UpdateFirmware request")
	t.Logf("Message ID: %s", emitter.LastMessage.MessageId)
}

func TestFirmwareServerAvailability(t *testing.T) {
	// This test checks if a firmware server is available
	// Skip if FIRMWARE_SERVER_URL is not set
	firmwareServerUrl := os.Getenv("FIRMWARE_SERVER_URL")
	if firmwareServerUrl == "" {
		t.Skip("FIRMWARE_SERVER_URL not set, skipping firmware server test")
	}
	
	t.Logf("Testing firmware server at: %s", firmwareServerUrl)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "HEAD", firmwareServerUrl, nil)
	require.NoError(t, err)
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Logf("Firmware server is available (HTTP %d)", resp.StatusCode)
}

func TestDownloadFirmware(t *testing.T) {
	// This test actually downloads firmware from a test server
	firmwareServerUrl := os.Getenv("FIRMWARE_SERVER_URL")
	if firmwareServerUrl == "" {
		t.Skip("FIRMWARE_SERVER_URL not set, skipping firmware download test")
	}
	
	t.Logf("Downloading test firmware from: %s", firmwareServerUrl)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", firmwareServerUrl, nil)
	require.NoError(t, err)
	
	startTime := time.Now()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Read the firmware data
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	downloadTime := time.Since(startTime)
	downloadedSize := len(data)
	
	t.Logf("Downloaded %d bytes in %v", downloadedSize, downloadTime)
	t.Logf("Download speed: %.2f KB/s", float64(downloadedSize)/1024/downloadTime.Seconds())
	
	// Verify minimum size
	assert.Greater(t, downloadedSize, 1024, "Firmware should be larger than 1KB")
}

func TestMultipleUpdateFirmwareRequests(t *testing.T) {
	// Test sending multiple firmware update requests
	emitter := &MockEmitter{}
	callMaker := ocpp201.NewCallMaker(emitter)
	
	chargeStations := []string{"cs001", "cs002", "cs003"}
	
	for i, csId := range chargeStations {
		retrieveTime := time.Now().Add(time.Duration(5+i) * time.Minute).Format(time.RFC3339)
		
		request := &ocpp201types.UpdateFirmwareRequestJson{
			RequestId: 10000 + i,
			Firmware: ocpp201types.FirmwareType{
				Location:         fmt.Sprintf("https://example.com/firmware/cs%d_v1.0.bin", i+1),
				RetrieveDateTime: retrieveTime,
			},
		}
		
		err := callMaker.Send(context.Background(), csId, request)
		require.NoError(t, err)
		
		t.Logf("Sent UpdateFirmware to %s (request ID: %d)", csId, 10000+i)
	}
	
	assert.Len(t, emitter.EmittedMessages, len(chargeStations))
	
	// Verify each message
	for i, msg := range emitter.EmittedMessages {
		assert.Equal(t, "UpdateFirmware", msg.Action)
		assert.Equal(t, transport.MessageTypeCall, msg.MessageType)
		t.Logf("Message %d: ID=%s, Action=%s", i+1, msg.MessageId, msg.Action)
	}
}

func TestUpdateFirmwareWithSignature(t *testing.T) {
	// Test firmware update with signature verification
	emitter := &MockEmitter{}
	callMaker := ocpp201.NewCallMaker(emitter)
	
	chargeStationId := "cs004"
	firmwareUrl := "https://example.com/firmware/signed_v1.0.bin"
	retrieveTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	requestId := 99999
	
	// Mock certificate and signature (in real scenario, these would be actual values)
	signingCert := "-----BEGIN CERTIFICATE-----\nMIIC...sample...cert...\n-----END CERTIFICATE-----"
	signature := "c29tZV9iYXNlNjRfZW5jb2RlZF9zaWduYXR1cmU="
	
	request := &ocpp201types.UpdateFirmwareRequestJson{
		RequestId: requestId,
		Firmware: ocpp201types.FirmwareType{
			Location:           firmwareUrl,
			RetrieveDateTime:   retrieveTime,
			SigningCertificate: &signingCert,
			Signature:          &signature,
		},
	}
	
	// Execute
	err := callMaker.Send(context.Background(), chargeStationId, request)
	
	// Assert
	require.NoError(t, err)
	assert.NotNil(t, emitter.LastMessage)
	assert.Equal(t, "UpdateFirmware", emitter.LastMessage.Action)
	
	t.Logf("Successfully sent signed firmware update request")
	t.Logf("Request includes signature and signing certificate")
}
