// SPDX-License-Identifier: Apache-2.0

package ocpp201

type UpdateFirmwareStatusEnumType string

const UpdateFirmwareStatusEnumTypeAccepted UpdateFirmwareStatusEnumType = "Accepted"
const UpdateFirmwareStatusEnumTypeRejected UpdateFirmwareStatusEnumType = "Rejected"
const UpdateFirmwareStatusEnumTypeAcceptedCanceled UpdateFirmwareStatusEnumType = "AcceptedCanceled"
const UpdateFirmwareStatusEnumTypeInvalidCertificate UpdateFirmwareStatusEnumType = "InvalidCertificate"
const UpdateFirmwareStatusEnumTypeRevokedCertificate UpdateFirmwareStatusEnumType = "RevokedCertificate"

type UpdateFirmwareResponseJson struct {
	// CustomData corresponds to the JSON schema field "customData".
	CustomData *CustomDataType `json:"customData,omitempty" yaml:"customData,omitempty" mapstructure:"customData,omitempty"`

	// This field indicates whether the Charging Station was able to accept the request.
	//
	Status UpdateFirmwareStatusEnumType `json:"status" yaml:"status" mapstructure:"status"`

	// StatusInfo corresponds to the JSON schema field "statusInfo".
	StatusInfo *StatusInfoType `json:"statusInfo,omitempty" yaml:"statusInfo,omitempty" mapstructure:"statusInfo,omitempty"`
}

func (*UpdateFirmwareResponseJson) IsResponse() {}
