// SPDX-License-Identifier: BUSL-1.1

package devicemetadata

import (
	"context"
	_ "embed"
	"io"
	"net"

	"github.com/framey-io/go-tarantool"
	"github.com/ip2location/ip2location-go"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

type (
	Keyword = string
	Country = string
	City    = string
	//nolint:revive // We don't have a choice if we want to embed it, cuz it will clash with others named "Repository".
	DeviceMetadataRepository interface {
		io.Closer
		IsValid(Country) bool
		LookupCountries(Keyword) []Country
		GetDeviceMetadataLocation(context.Context, *GetDeviceMetadataLocationArg) *DeviceLocation
		GetDeviceMetadata(context.Context, device.ID) (*DeviceMetadata, error)
		ReplaceDeviceMetadata(context.Context, *ReplaceDeviceMetadataArg) error
	}
	DeviceLocation struct {
		Country Country `json:"country,omitempty" example:"US"`
		City    City    `json:"city,omitempty" example:"New York"`
	}
	//nolint:revive // We don't have a choice if we want to embed it, cuz it will clash with others named "snapshot".
	DeviceMetadataSnapshot struct {
		*DeviceMetadata
		Before *DeviceMetadata `json:"before"`
	}
	DeviceMetadata struct {
		FirstInstallTime *time.Time `json:"firstInstallTime" swaggertype:"integer"`
		LastUpdateTime   *time.Time `json:"lastUpdateTime" swaggertype:"integer"`
		device.ID
		ReadableVersion       string `json:"readableVersion"`
		Fingerprint           string `json:"fingerprint"`
		InstanceID            string `json:"instanceId"`
		Hardware              string `json:"hardware"`
		Product               string `json:"product"`
		Device                string `json:"device"`
		Type                  string `json:"type"`
		Tags                  string `json:"tags"`
		DeviceID              string `json:"deviceId"`
		DeviceType            string `json:"deviceType"`
		DeviceName            string `json:"deviceName"`
		Brand                 string `json:"brand"`
		Carrier               string `json:"carrier"`
		Manufacturer          string `json:"manufacturer"`
		UserAgent             string `json:"userAgent"`
		SystemName            string `json:"systemName"`
		SystemVersion         string `json:"systemVersion"`
		BaseOS                string `json:"baseOs"`
		BuildID               string `json:"buildId"`
		Bootloader            string `json:"bootloader"`
		Codename              string `json:"codename"`
		InstallerPackageName  string `json:"installerPackageName"`
		PushNotificationToken string `json:"pushNotificationToken"`
		APILevel              uint64 `json:"apiLevel"`
		Tablet                bool   `json:"tablet"`
		PinOrFingerprintSet   bool   `json:"pinOrFingerprintSet"`
		Emulator              bool   `json:"emulator"`
	}
)

// API Arguments.
type (
	ReplaceDeviceMetadataArg struct {
		ClientIP net.IP `json:"clientIp" swaggerignore:"true"`
		DeviceMetadata
	}
	GetDeviceMetadataLocationArg struct {
		device.ID
		ClientIP net.IP `json:"clientIp" swaggerignore:"true"`
	}
)

// Private API.

const (
	applicationYamlKey = "users"
)

var (
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	countries map[Country]*country
	//go:embed countries.json
	countriesJSON string
)

type (
	deviceMetadata struct {
		//nolint:unused,revive,tagliatelle // Wrong. It's a marker for marshalling/unmarshalling to/from db.
		_msgpack struct{} `msgpack:",asArray"`
		ip2location.IP2Locationrecord
		UpdatedAt *time.Time `json:"updatedAt"`
		DeviceMetadata
	}
	country struct {
		Name    string `json:"name"`
		Flag    string `json:"flag"`
		IsoCode string `json:"isoCode"`
		IddCode string `json:"iddCode"`
	}
	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		IP2LocationBinaryPath string `yaml:"ip2LocationBinaryPath"`
		MessageBroker         struct {
			Topics []struct {
				Name string `yaml:"name" json:"name"`
			} `yaml:"topics"`
		} `yaml:"messageBroker"`
	}
	repository struct {
		db            tarantool.Connector
		mb            messagebroker.Client
		ip2LocationDB *ip2location.DB
	}
)
