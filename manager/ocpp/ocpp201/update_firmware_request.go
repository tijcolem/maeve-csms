// SPDX-License-Identifier: Apache-2.0

package ocpp201

// FirmwareType represents a copy of the firmware that can be loaded/updated on the Charging Station.
type FirmwareType struct {
	// CustomData corresponds to the JSON schema field "customData".
	CustomData *CustomDataType `json:"customData,omitempty" yaml:"customData,omitempty" mapstructure:"customData,omitempty"`

	// Firmware. Location. URI
	// urn:x-enexis:ecdm:uid:1:569460
	// URI defining the origin of the firmware.
	//
	Location string `json:"location" yaml:"location" mapstructure:"location"`

	// Firmware. Retrieve. Date_ Time
	// urn:x-enexis:ecdm:uid:1:569461
	// Date and time at which the firmware shall be retrieved.
	//
	RetrieveDateTime string `json:"retrieveDateTime" yaml:"retrieveDateTime" mapstructure:"retrieveDateTime"`

	// Firmware. Install. Date_ Time
	// urn:x-enexis:ecdm:uid:1:569462
	// Date and time at which the firmware shall be installed.
	//
	InstallDateTime *string `json:"installDateTime,omitempty" yaml:"installDateTime,omitempty" mapstructure:"installDateTime,omitempty"`

	// Certificate with which the firmware was signed.
	// PEM encoded X.509 certificate.
	//
	SigningCertificate *string `json:"signingCertificate,omitempty" yaml:"signingCertificate,omitempty" mapstructure:"signingCertificate,omitempty"`

	// Firmware. Signature. Signature
	// urn:x-enexis:ecdm:uid:1:569464
	// Base64 encoded firmware signature.
	//
	Signature *string `json:"signature,omitempty" yaml:"signature,omitempty" mapstructure:"signature,omitempty"`
}

type UpdateFirmwareRequestJson struct {
	// CustomData corresponds to the JSON schema field "customData".
	CustomData *CustomDataType `json:"customData,omitempty" yaml:"customData,omitempty" mapstructure:"customData,omitempty"`

	// This specifies how many times Charging Station must try to download the firmware before giving up.
	// If this field is not present, it is left to Charging Station to decide how many times it wants to retry.
	//
	Retries *int `json:"retries,omitempty" yaml:"retries,omitempty" mapstructure:"retries,omitempty"`

	// The interval in seconds after which a retry may be attempted.
	// If this field is not present, it is left to Charging Station to decide how long to wait between attempts.
	//
	RetryInterval *int `json:"retryInterval,omitempty" yaml:"retryInterval,omitempty" mapstructure:"retryInterval,omitempty"`

	// The Id of this request
	//
	RequestId int `json:"requestId" yaml:"requestId" mapstructure:"requestId"`

	// Firmware corresponds to the JSON schema field "firmware".
	Firmware FirmwareType `json:"firmware" yaml:"firmware" mapstructure:"firmware"`
}

func (*UpdateFirmwareRequestJson) IsRequest() {}
