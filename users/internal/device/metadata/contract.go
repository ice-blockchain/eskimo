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
		// Read Only.
		UpdatedAt        *time.Time `json:"updatedAt,omitempty" swaggertype:"string" db:"updated_at"`
		FirstInstallTime *time.Time `json:"firstInstallTime,omitempty" swaggertype:"integer" db:"first_install_time"`
		LastUpdateTime   *time.Time `json:"lastUpdateTime,omitempty" swaggertype:"integer" db:"last_update_time"`
		device.ID
		ReadableVersion       string `json:"readableVersion,omitempty" db:"readable_version"`
		Fingerprint           string `json:"fingerprint,omitempty" db:"fingerprint"`
		InstanceID            string `json:"instanceId,omitempty" db:"instance_id"`
		Hardware              string `json:"hardware,omitempty" db:"hardware"`
		Product               string `json:"product,omitempty" db:"product"`
		Device                string `json:"device,omitempty" db:"device"`
		Type                  string `json:"type,omitempty" db:"type"`
		Tags                  string `json:"tags,omitempty" db:"tags"`
		DeviceID              string `json:"deviceId,omitempty" db:"device_id"`
		DeviceType            string `json:"deviceType,omitempty" db:"device_type"`
		DeviceName            string `json:"deviceName,omitempty" db:"device_name"`
		Brand                 string `json:"brand,omitempty" db:"brand"`
		Carrier               string `json:"carrier,omitempty" db:"carrier"`
		Manufacturer          string `json:"manufacturer,omitempty" db:"manufacturer"`
		UserAgent             string `json:"userAgent,omitempty" db:"user_agent"`
		SystemName            string `json:"systemName,omitempty" db:"system_name"`
		SystemVersion         string `json:"systemVersion,omitempty" db:"system_version"`
		BaseOS                string `json:"baseOs,omitempty" db:"base_os"`
		BuildID               string `json:"buildId,omitempty" db:"build_id"`
		Bootloader            string `json:"bootloader,omitempty" db:"bootloader"`
		Codename              string `json:"codename,omitempty" db:"codename"`
		InstallerPackageName  string `json:"installerPackageName,omitempty" db:"installer_package_name"`
		PushNotificationToken string `json:"pushNotificationToken,omitempty" db:"push_notification_token"`
		TZ                    string `json:"tz,omitempty" db:"device_timezone"`
		ip2LocationRecord
		APILevel            uint64 `json:"apiLevel,omitempty" db:"api_level"`
		Tablet              bool   `json:"tablet,omitempty" db:"tablet"`
		PinOrFingerprintSet bool   `json:"pinOrFingerprintSet,omitempty" db:"pin_or_fingerprint_set"`
		Emulator            bool   `json:"emulator,omitempty" db:"emulator"`
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
		CountryShort       string  `json:"-" swaggerignore:"true" db:"country_short"`
		CountryLong        string  `json:"-" swaggerignore:"true" db:"country_long"`
		Region             string  `json:"-" swaggerignore:"true" db:"region"`
		City               string  `json:"-" swaggerignore:"true" db:"city"`
		Isp                string  `json:"-" swaggerignore:"true" db:"isp"`
		Domain             string  `json:"-" swaggerignore:"true" db:"domain"`
		Zipcode            string  `json:"-" swaggerignore:"true" db:"zipcode"`
		Timezone           string  `json:"-" swaggerignore:"true" db:"timezone"`
		Netspeed           string  `json:"-" swaggerignore:"true" db:"net_speed"`
		Iddcode            string  `json:"-" swaggerignore:"true" db:"idd_code"`
		Areacode           string  `json:"-" swaggerignore:"true" db:"area_code"`
		Weatherstationcode string  `json:"-" swaggerignore:"true" db:"weather_station_code"`
		Weatherstationname string  `json:"-" swaggerignore:"true" db:"weather_station_name"`
		Mcc                string  `json:"-" swaggerignore:"true" db:"mcc"`
		Mnc                string  `json:"-" swaggerignore:"true" db:"mnc"`
		Mobilebrand        string  `json:"-" swaggerignore:"true" db:"mobile_brand"`
		Usagetype          string  `json:"-" swaggerignore:"true" db:"usage_type"`
		Latitude           float64 `json:"-" swaggerignore:"true" db:"latitude"`
		Longitude          float64 `json:"-" swaggerignore:"true" db:"longitude"`
		Elevation          float64 `json:"-" swaggerignore:"true" db:"elevation"`
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
