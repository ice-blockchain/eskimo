// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"
	"net"
	"regexp"
	stdlibtime "time"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"
	"github.com/twilio/twilio-go"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	devicesettings "github.com/ice-blockchain/eskimo/users/internal/device/settings"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

const (
	ContactsReferrals = "CONTACTS"
	Tier1Referrals    = "T1"
	Tier2Referrals    = "T2"
	UsernameRegex     = `^[-\w.]{4,20}$`
)

var (
	ErrNotFound                   = storage.ErrNotFound
	ErrRelationNotFound           = storage.ErrRelationNotFound
	ErrDuplicate                  = storage.ErrDuplicate
	ErrInvalidPhoneValidationCode = errors.New("invalid phone validation code")
	ErrExpiredPhoneValidationCode = errors.New("expired phone validation code")
	ErrInvalidPhoneNumber         = errors.New("phone number invalid")
	ErrInvalidPhoneNumberFormat   = errors.New("phone number has invalid format")
	ErrInvalidCountry             = errors.New("country invalid")
	//nolint:gochecknoglobals // It's just for more descriptive validation messages.
	ReferralTypes         = []string{ContactsReferrals, Tier1Referrals, Tier2Referrals}
	CompiledUsernameRegex = regexp.MustCompile(UsernameRegex)
)

type (
	NotExpired         bool
	UserID             = string
	MinimalUserProfile struct {
		Active *NotExpired `json:"active,omitempty" example:"true"`
		Pinged *NotExpired `json:"pinged,omitempty" example:"false"`
		PublicUserInformation
	}
	PublicUserInformation struct {
		ID                UserID `json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Username          string `json:"username,omitempty" example:"jdoe"`
		FirstName         string `json:"firstName,omitempty" example:"John"`
		LastName          string `json:"lastName,omitempty" example:"Doe"`
		PhoneNumber       string `json:"phoneNumber,omitempty" example:"+12099216581"`
		ProfilePictureURL string `json:"profilePictureUrl,omitempty" example:"https://somecdn.com/p1.jpg"`
		DeviceLocation
	}
	User struct {
		_msgpack            struct{}   `msgpack:",asArray"` // nolint:unused,tagliatelle,revive,nosnakecase // To insert we need asArray
		CreatedAt           *time.Time `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt           *time.Time `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		LastMiningStartedAt *time.Time `json:"lastMiningStartedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		LastPingAt          *time.Time `json:"lastPingAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		PublicUserInformation
		Email                   string `json:"email,omitempty" example:"jdoe@gmail.com"`
		ReferredBy              UserID `json:"referredBy,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumberHash         string `json:"phoneNumberHash,omitempty" example:"Ef86A6021afCDe5673511376B2"`
		AgendaPhoneNumberHashes string `json:"agendaPhoneNumberHashes,omitempty" example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2"` //nolint:lll // .
		HashCode                uint64 `json:"-"`
	}
	RelatableUserProfile struct {
		MinimalUserProfile
		ReferralType string `json:"referralType,omitempty" example:"T1" enums:"T1,T2"`
	}
	UserProfile struct {
		User
		ReferralCount uint64 `json:"referralCount,omitempty" example:"100"`
	}
	Referrals struct {
		Referrals []*Referral `json:"referrals"`
		Active    uint64      `json:"active" example:"11"`
		Total     uint64      `json:"total" example:"11"`
	}
	Referral struct {
		MinimalUserProfile
	}
	UserSnapshot struct {
		*User
		Before *User `json:"before"`
	}
	ReferralAcquisition struct {
		Date *time.Time `json:"date" example:"2022-01-03"`
		T1   uint64     `json:"t1" example:"22"`
		T2   uint64     `json:"t2" example:"13"`
	}
	CountryStatistics struct {
		_msgpack struct{} `msgpack:",asArray"` // nolint:unused,revive,tagliatelle,nosnakecase // To insert we need asArray
		// ISO 3166 country code.
		Country   devicemetadata.Country `json:"country" example:"US"`
		UserCount uint64                 `json:"userCount" example:"12121212"`
	}
	PhoneNumberValidation struct {
		_msgpack struct{} `msgpack:",asArray"` // nolint:unused,revive,tagliatelle,nosnakecase // To insert we need asArray
		// `Read Only`.
		CreatedAt       *time.Time `json:"createdAt" example:"2022-01-03T16:20:52.156534Z"`
		UserID          UserID     `json:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber     string     `json:"phoneNumber" example:"+12345678"`
		PhoneNumberHash string     `json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		ValidationCode  string     `json:"validationCode" example:"1234"`
	}
	// Repository main API exposed that handles all the features of this package.
	Repository interface {
		io.Closer
		devicemetadata.DeviceMetadataRepository
		devicesettings.DeviceSettingsRepository

		GetUsers(ctx context.Context, keyword string, limit, offset uint64) ([]*RelatableUserProfile, error)
		GetUserByUsername(ctx context.Context, username string) (*UserProfile, error)
		GetUserByID(ctx context.Context, userID string) (*UserProfile, error)

		CreateUser(ctx context.Context, usr *User, clientIP net.IP) error
		DeleteUser(ctx context.Context, userID UserID) error
		ModifyUser(ctx context.Context, usr *User, profilePicture *multipart.FileHeader) error

		ValidatePhoneNumber(context.Context, *PhoneNumberValidation) error

		GetTopCountries(ctx context.Context, keyword string, limit, offset uint64) ([]*CountryStatistics, error)

		GetReferrals(ctx context.Context, userID, referralType string, limit, offset uint64) (*Referrals, error)
		GetReferralAcquisitionHistory(ctx context.Context, userID string, days uint64) ([]*ReferralAcquisition, error)
	}
	Processor interface {
		Repository
		CheckHealth(context.Context) error
	}
)

// Proxy Internal Types.
type (
	DeviceID               = device.ID
	DeviceMetadataSnapshot = devicemetadata.DeviceMetadataSnapshot
	DeviceMetadata         = devicemetadata.DeviceMetadata
	DeviceLocation         = devicemetadata.DeviceLocation
	DeviceSettings         = devicesettings.DeviceSettings
	NotificationSettings   = devicesettings.NotificationSettings
	DeviceSettingsSnapshot = devicesettings.DeviceSettingsSnapshot
)

// Private API.

const (
	applicationYamlKey                                    = "users"
	isPhoneNumberConfirmedCtxValueKey                     = "isPhoneNumberConfirmedCtxValueKey"
	requestingUserIDCtxValueKey                           = "requestingUserIDCtxValueKey"
	defaultUserImage                                      = "default-user-image.jpg"
	hashCodeDBColumnName                                  = "hash_code"
	add                               arithmeticOperation = "+"
	subtract                          arithmeticOperation = "-"
	expirationDeadline                                    = 24 * stdlibtime.Hour
)

var (
	//go:embed DDL.lua
	ddl string
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
	_   msgpack.CustomDecoder = (*NotExpired)(nil)
)

type (
	arithmeticOperation string

	userSnapshotSource struct {
		*processor
	}

	miningStartedSource struct {
		*processor
	}
	miningStarted struct {
		TS *time.Time `json:"ts"`
	}

	// | repository implements the public API that this package exposes.
	repository struct {
		db tarantool.Connector
		mb messagebroker.Client
		devicemetadata.DeviceMetadataRepository
		devicesettings.DeviceSettingsRepository
		twilioClient *twilio.RestClient
		shutdown     func() error
	}

	processor struct {
		*repository
	}
	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		PictureStorage struct {
			URLUpload   string `yaml:"urlUpload"`
			URLDownload string `yaml:"urlDownload"`
			AccessKey   string `yaml:"accessKey"`
		} `yaml:"pictureStorage"`
		MessageBroker struct {
			ConsumingTopics []string `yaml:"consumingTopics"`
			Topics          []struct {
				Name string `yaml:"name" json:"name"`
			} `yaml:"topics"`
		} `yaml:"messageBroker"`
		PhoneNumberValidation struct {
			TwilioCredentials struct {
				User     string `yaml:"user"`
				Password string `yaml:"password"`
			} `yaml:"twilioCredentials"`
			FromPhoneNumber string              `yaml:"fromPhoneNumber"`
			SmsTemplate     string              `yaml:"smsTemplate"`
			ExpirationTime  stdlibtime.Duration `yaml:"expirationTime"`
		} `yaml:"phoneNumberValidation"`
	}
)
