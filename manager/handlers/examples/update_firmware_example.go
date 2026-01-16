// SPDX-License-Identifier: Apache-2.0

// Package examples demonstrates how to send firmware updates to charge stations
package examples

import (
	"context"
	"time"

	"github.com/thoughtworks/maeve-csms/manager/handlers/ocpp16"
	"github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
	ocpp16types "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp16"
	ocpp201types "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
	"github.com/thoughtworks/maeve-csms/manager/transport"
)

// SendOCPP201FirmwareUpdate demonstrates how to send a firmware update to an OCPP 2.0.1 charge station
func SendOCPP201FirmwareUpdate(emitter transport.Emitter, chargeStationId string) error {
	// Create the call maker
	callMaker := ocpp201.NewCallMaker(emitter)

	// Prepare the firmware update request
	retrieveDateTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	installDateTime := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	
	request := &ocpp201types.UpdateFirmwareRequestJson{
		RequestId: 12345,
		Firmware: ocpp201types.FirmwareType{
			Location:         "https://example.com/firmware/v1.2.3.bin",
			RetrieveDateTime: retrieveDateTime,
			InstallDateTime:  &installDateTime,
		},
	}

	// Send the request to the charge station
	return callMaker.Send(context.Background(), chargeStationId, request)
}

// SendOCPP16FirmwareUpdate demonstrates how to send a firmware update to an OCPP 1.6 charge station
func SendOCPP16FirmwareUpdate(emitter transport.Emitter, chargeStationId string) error {
	// Create the call maker
	callMaker := ocpp16.NewCallMaker(emitter)

	// Prepare the firmware update request
	retrieveDate := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	retries := 3
	retryInterval := 60
	
	request := &ocpp16types.UpdateFirmwareJson{
		Location:      "https://example.com/firmware/v1.2.3.bin",
		RetrieveDate:  retrieveDate,
		Retries:       &retries,
		RetryInterval: &retryInterval,
	}

	// Send the request to the charge station
	return callMaker.Send(context.Background(), chargeStationId, request)
}
