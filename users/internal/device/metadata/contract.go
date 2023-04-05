// SPDX-License-Identifier: ice License 1.0

package devicemetadata

import (
	"context"
	_ "embed"
	"io"
	"net"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

var (
	ErrInvalidAppVersion  = errors.New("invalid mobile app version")
	ErrOutdatedAppVersion = errors.New("outdated mobile app version")
)

type (
	Keyword = string
	Country = string
	City    = string
	//nolint:revive // We don't have a choice if we want to embed it, cuz it will clash with others named "Repository".
	DeviceMetadataRepository interface {
		io.Closer
		IsValid(Country) bool
		LookupCountries(Keyword) []Country
		GetDeviceMetadataLocation(ctx context.Context, deviceID *device.ID, clientIP net.IP) *DeviceLocation
		GetDeviceMetadata(context.Context, *device.ID) (*DeviceMetadata, error)
		ReplaceDeviceMetadata(ctx context.Context, deviceMetadata *DeviceMetadata, clientIP net.IP) error
		DeleteAllDeviceMetadata(ctx context.Context, userID string) error
	}
	DeviceLocation struct {
		Country Country `json:"country,omitempty" example:"US"`
		City    City    `json:"city,omitempty" example:"New York"`
	}
	//nolint:revive // We don't have a choice if we want to embed it, cuz it will clash with others named "snapshot".
	DeviceMetadataSnapshot struct {
		*DeviceMetadata
		Before *DeviceMetadata `json:"before,omitempty"`
	}
	DeviceMetadata struct {
		//nolint:unused,revive,tagliatelle,nosnakecase // Wrong. It's a marker for marshalling/unmarshalling to/from db.
		_msgpack struct{} `msgpack:",asArray"`
		// Read Only.
		UpdatedAt        *time.Time `json:"updatedAt,omitempty" swaggertype:"string"`
		FirstInstallTime *time.Time `json:"firstInstallTime,omitempty" swaggertype:"integer"`
		LastUpdateTime   *time.Time `json:"lastUpdateTime,omitempty" swaggertype:"integer"`
		device.ID
		ReadableVersion       string `json:"readableVersion,omitempty"`
		Fingerprint           string `json:"fingerprint,omitempty"`
		InstanceID            string `json:"instanceId,omitempty"`
		Hardware              string `json:"hardware,omitempty"`
		Product               string `json:"product,omitempty"`
		Device                string `json:"device,omitempty"`
		Type                  string `json:"type,omitempty"`
		Tags                  string `json:"tags,omitempty"`
		DeviceID              string `json:"deviceId,omitempty"`
		DeviceType            string `json:"deviceType,omitempty"`
		DeviceName            string `json:"deviceName,omitempty"`
		Brand                 string `json:"brand,omitempty"`
		Carrier               string `json:"carrier,omitempty"`
		Manufacturer          string `json:"manufacturer,omitempty"`
		UserAgent             string `json:"userAgent,omitempty"`
		SystemName            string `json:"systemName,omitempty"`
		SystemVersion         string `json:"systemVersion,omitempty"`
		BaseOS                string `json:"baseOs,omitempty"`
		BuildID               string `json:"buildId,omitempty"`
		Bootloader            string `json:"bootloader,omitempty"`
		Codename              string `json:"codename,omitempty"`
		InstallerPackageName  string `json:"installerPackageName,omitempty"`
		PushNotificationToken string `json:"pushNotificationToken,omitempty"`
		TZ                    string `json:"tz,omitempty"`
		ip2LocationRecord
		APILevel            uint64 `json:"apiLevel,omitempty"`
		Tablet              bool   `json:"tablet,omitempty"`
		PinOrFingerprintSet bool   `json:"pinOrFingerprintSet,omitempty"`
		Emulator            bool   `json:"emulator,omitempty"`
	}
)

// Private API.

const (
	applicationYamlKey = "users"
)

var (
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	countries map[Country]*country
	//go:embed countries.json
	countriesJSON string
)

type (
	ip2LocationRecord struct {
		CountryShort       string  `json:"-" swaggerignore:"true"`
		CountryLong        string  `json:"-" swaggerignore:"true"`
		Region             string  `json:"-" swaggerignore:"true"`
		City               string  `json:"-" swaggerignore:"true"`
		Isp                string  `json:"-" swaggerignore:"true"`
		Domain             string  `json:"-" swaggerignore:"true"`
		Zipcode            string  `json:"-" swaggerignore:"true"`
		Timezone           string  `json:"-" swaggerignore:"true"`
		Netspeed           string  `json:"-" swaggerignore:"true"`
		Iddcode            string  `json:"-" swaggerignore:"true"`
		Areacode           string  `json:"-" swaggerignore:"true"`
		Weatherstationcode string  `json:"-" swaggerignore:"true"`
		Weatherstationname string  `json:"-" swaggerignore:"true"`
		Mcc                string  `json:"-" swaggerignore:"true"`
		Mnc                string  `json:"-" swaggerignore:"true"`
		Mobilebrand        string  `json:"-" swaggerignore:"true"`
		Usagetype          string  `json:"-" swaggerignore:"true"`
		Latitude           float64 `json:"-" swaggerignore:"true"`
		Longitude          float64 `json:"-" swaggerignore:"true"`
		Elevation          float64 `json:"-" swaggerignore:"true"`
	}
	country struct {
		Name    string `json:"name"`
		Flag    string `json:"flag"`
		IsoCode string `json:"isoCode"`
		IddCode string `json:"iddCode"`
	}
	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		RequiredAppVersion    string                   `yaml:"requiredAppVersion"`
		IP2LocationBinaryPath string                   `yaml:"ip2LocationBinaryPath"`
		messagebroker.Config  `mapstructure:",squash"` //nolint:tagliatelle // Nope.
	}
	repository struct {
		cfg           *config
		db            *storage.DB
		mb            messagebroker.Client
		ip2LocationDB *ip2location.DB
	}
)
