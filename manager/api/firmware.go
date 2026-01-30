// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"time"

	"github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp16"
	"github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
)

func (s *Server) sendOCPP201FirmwareUpdate(ctx context.Context, csId string, req *ChargeStationFirmwareUpdate) error {
	request := &ocpp201.UpdateFirmwareRequestJson{
		Firmware: ocpp201.FirmwareType{
			Location:         req.Location,
			RetrieveDateTime: req.RetrieveDateTime.Format(time.RFC3339),
		},
	}

	if req.InstallDateTime != nil {
		installTime := req.InstallDateTime.Format(time.RFC3339)
		request.Firmware.InstallDateTime = &installTime
	}

	if req.RequestId != nil {
		request.RequestId = *req.RequestId
	}

	if req.Retries != nil {
		request.Retries = req.Retries
	}

	if req.RetryInterval != nil {
		request.RetryInterval = req.RetryInterval
	}

	if req.SigningCertificate != nil {
		request.Firmware.SigningCertificate = req.SigningCertificate
	}

	if req.Signature != nil {
		request.Firmware.Signature = req.Signature
	}

	return s.ocpp201CallMaker.Send(ctx, csId, request)
}

func (s *Server) sendOCPP16FirmwareUpdate(ctx context.Context, csId string, req *ChargeStationFirmwareUpdate) error {
	request := &ocpp16.UpdateFirmwareJson{
		Location:     req.Location,
		RetrieveDate: req.RetrieveDateTime.Format(time.RFC3339),
	}

	if req.Retries != nil {
		request.Retries = req.Retries
	}

	if req.RetryInterval != nil {
		request.RetryInterval = req.RetryInterval
	}

	return s.ocpp16CallMaker.Send(ctx, csId, request)
}
