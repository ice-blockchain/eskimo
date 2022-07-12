// SPDX-License-Identifier: BUSL-1.1

package devicesettings

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

//nolint:gochecknoglobals // It's a non-primitive constant.
var AllNotificationDomains = [4]string{"NEWS", "ACHIEVEMENTS", "TEAM", "REMINDERS"}

type (
	NotificationDomain   = string
	NotificationSettings map[NotificationDomain]*NotificationChannels
	NotificationChannels struct {
		Push  *bool `json:"push,omitempty" example:"true"`
		Email *bool `json:"email,omitempty" example:"false"`
		SMS   *bool `json:"sms,omitempty" example:"false"`
		InApp *bool `json:"inApp,omitempty" example:"false"`
	}
	//nolint:revive // We don't have a choice if we want to embed it, cuz it will clash with others named "snapshot".
	DeviceSettingsSnapshot struct {
		*DeviceSettings
		Before *DeviceSettings `json:"before"`
	}
	DeviceSettings struct {
		//nolint:unused // Because it is used by the msgpack library for marshalling/unmarshalling.
		_msgpack struct{} `msgpack:",asArray"`
		// `Read Only`.
		UpdatedAt *time.Time `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		// Optional.
		NotificationSettings *NotificationSettings `json:"notificationSettings,omitempty"`
		// Optional.
		Language *string `json:"language,omitempty" example:"en"`
		// Optional. Default is `false`.
		DisableAllNotifications *bool `json:"disableAllNotifications" example:"true"`
		device.ID
	}
	//nolint:revive // We don't have a choice if we want to embed it, cuz it will clash with others named "Repository".
	DeviceSettingsRepository interface {
		GetDeviceSettings(context.Context, device.ID) (*DeviceSettings, error)
		CreateDeviceSettings(context.Context, *DeviceSettings) error
		ModifyDeviceSettings(context.Context, *DeviceSettings) error
	}
)

// Private API.

const (
	applicationYamlKey = "users"
	defaultLanguage    = "en"
)

var (
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
	_   msgpack.CustomEncoder = (*NotificationSettings)(nil)
	_   msgpack.CustomDecoder = (*NotificationSettings)(nil)
)

type (
	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		MessageBroker struct {
			Topics []struct {
				Name string `yaml:"name" json:"name"`
			} `yaml:"topics"`
		} `yaml:"messageBroker"`
	}
	repository struct {
		db tarantool.Connector
		mb messagebroker.Client
	}
)
