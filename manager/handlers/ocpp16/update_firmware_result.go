// SPDX-License-Identifier: Apache-2.0

package ocpp16

import (
	"context"
	"github.com/thoughtworks/maeve-csms/manager/ocpp"
	types "github.com/thoughtworks/maeve-csms/manager/ocpp/ocpp16"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type UpdateFirmwareResultHandler struct{}

func (h UpdateFirmwareResultHandler) HandleCallResult(ctx context.Context, chargeStationId string, request ocpp.Request, response ocpp.Response, state any) error {
	req := request.(*types.UpdateFirmwareJson)

	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.String("update_firmware.location", req.Location),
		attribute.String("update_firmware.retrieve_date", req.RetrieveDate))

	if req.Retries != nil {
		span.SetAttributes(attribute.Int("update_firmware.retries", *req.Retries))
	}

	if req.RetryInterval != nil {
		span.SetAttributes(attribute.Int("update_firmware.retry_interval", *req.RetryInterval))
	}

	return nil
}
