// SPDX-License-Identifier: Apache-2.0

package ocpp16

type UpdateFirmwareJson struct {
	// Location corresponds to the JSON schema field "location".
	Location string `json:"location" yaml:"location" mapstructure:"location"`

	// Retries corresponds to the JSON schema field "retries".
	Retries *int `json:"retries,omitempty" yaml:"retries,omitempty" mapstructure:"retries,omitempty"`

	// RetrieveDate corresponds to the JSON schema field "retrieveDate".
	RetrieveDate string `json:"retrieveDate" yaml:"retrieveDate" mapstructure:"retrieveDate"`

	// RetryInterval corresponds to the JSON schema field "retryInterval".
	RetryInterval *int `json:"retryInterval,omitempty" yaml:"retryInterval,omitempty" mapstructure:"retryInterval,omitempty"`
}

func (*UpdateFirmwareJson) IsRequest() {}
