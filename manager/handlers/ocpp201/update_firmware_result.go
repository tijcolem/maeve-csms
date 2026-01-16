// SPDX-License-Identifier: Apache-2.0

package ocpp201

import (
	"context"
	"github.com/thoughtworks/maeve-csms/manager/ocpp"
	types "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp201"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type UpdateFirmwareResultHandler struct{}

func (h UpdateFirmwareResultHandler) HandleCallResult(ctx context.Context, chargeStationId string, request ocpp.Request, response ocpp.Response, state any) error {
	req := request.(*types.UpdateFirmwareRequestJson)
	resp := response.(*types.UpdateFirmwareResponseJson)

	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.Int("update_firmware.request_id", req.RequestId),
		attribute.String("update_firmware.location", req.Firmware.Location),
		attribute.String("update_firmware.status", string(resp.Status)))

	if req.Retries != nil {
		span.SetAttributes(attribute.Int("update_firmware.retries", *req.Retries))
	}

	if req.RetryInterval != nil {
		span.SetAttributes(attribute.Int("update_firmware.retry_interval", *req.RetryInterval))
	}

	return nil
}
