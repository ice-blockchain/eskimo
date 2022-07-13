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
	UsernameRegex     = `^[\w\-.]{4,20}$`
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
	NotExpired                   bool
	UserID                       = string
	Username                     = string
	DeviceID                     = device.ID
	DeviceMetadataSnapshot       = devicemetadata.DeviceMetadataSnapshot
	DeviceMetadata               = devicemetadata.DeviceMetadata
	ReplaceDeviceMetadataArg     = devicemetadata.ReplaceDeviceMetadataArg
	GetDeviceMetadataLocationArg = devicemetadata.GetDeviceMetadataLocationArg
	DeviceLocation               = devicemetadata.DeviceLocation
	DeviceSettings               = devicesettings.DeviceSettings
	DeviceSettingsSnapshot       = devicesettings.DeviceSettingsSnapshot
	MinimalUserProfile           struct {
		Active      *NotExpired `json:"active,omitempty" example:"true"`
		PingAllowed *NotExpired `json:"pingAllowed,omitempty" example:"false"`
		PublicUserInformation
	}
	PublicUserInformation struct {
		ID                UserID   `uri:"userId" json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Username          Username `json:"username,omitempty" example:"jdoe"`
		FirstName         string   `json:"firstName,omitempty" example:"John"`
		LastName          string   `json:"lastName,omitempty" example:"Doe"`
		PhoneNumber       string   `json:"phoneNumber,omitempty" example:"+12099216581"`
		ProfilePictureURL string   `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
		DeviceLocation
	}
	User struct {
		_msgpack            struct{}   `msgpack:",asArray"` // nolint:unused // To insert we need asArray
		CreatedAt           *time.Time `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt           *time.Time `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		LastMiningStartedAt *time.Time `json:"lastMiningStartedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		LastPingAt          *time.Time `json:"lastPingAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		PublicUserInformation
		Email                   string `form:"email" json:"email,omitempty" example:"jdoe@gmail.com"`
		ReferredBy              UserID `form:"referredBy" json:"referredBy,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumberHash         string `form:"phoneNumberHash" json:"phoneNumberHash,omitempty" example:"Ef86A6021afCDe5673511376B2"`
		AgendaPhoneNumberHashes string `form:"agendaPhoneNumberHashes" json:"agendaPhoneNumberHashes,omitempty" example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2"` //nolint:lll // .
		HashCode                uint64 `json:"-" swaggerignore:"true"`
	}
	RelatableUserProfile struct {
		MinimalUserProfile
		ReferralType string `json:"referralType,omitempty" example:"T1" enums:"T1,T2"`
	}
	UserProfile struct {
		PublicUserInformation
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
		_msgpack struct{} `msgpack:",asArray"` // nolint:unused // To insert we need asArray
		// ISO 3166 country code.
		Country   devicemetadata.Country `json:"country" example:"US"`
		UserCount uint64                 `json:"userCount" example:"12121212"`
	}
	PhoneNumberValidation struct {
		_msgpack struct{} `msgpack:",asArray"` // nolint:unused // To insert we need asArray
		// `Read Only`.
		CreatedAt       *time.Time `json:"createdAt" example:"2022-01-03T16:20:52.156534Z"`
		UserID          UserID     `uri:"userId" json:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber     string     `json:"phoneNumber" example:"+12345678"`
		PhoneNumberHash string     `json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		ValidationCode  string     `json:"validationCode" example:"1234"`
	} // @name ValidatePhoneNumberRequestBody //nolint:godot // It's handled by swaggo.
	// Repository main API exposed that handles all the features of this package.
	Repository interface {
		io.Closer
		devicemetadata.DeviceMetadataRepository
		devicesettings.DeviceSettingsRepository

		GetUsers(context.Context, *GetUsersArg) ([]*RelatableUserProfile, error)
		GetUserByUsername(context.Context, Username) (*UserProfile, error)
		GetUserByID(context.Context, UserID) (*UserProfile, error)

		CreateUser(context.Context, *CreateUserArg) error
		DeleteUser(context.Context, UserID) error
		ModifyUser(context.Context, *ModifyUserArg) error

		ValidatePhoneNumber(context.Context, *PhoneNumberValidation) error

		GetTopCountries(context.Context, *GetTopCountriesArg) ([]*CountryStatistics, error)

		GetReferrals(context.Context, *GetReferralsArg) (*Referrals, error)
		GetReferralAcquisitionHistory(context.Context, *GetReferralAcquisitionHistoryArg) ([]*ReferralAcquisition, error)
	}
	Processor interface {
		Repository
		CheckHealth(context.Context) error
	}
)

// API Arguments.
type (
	CreateUserArg struct {
		// Optional.
		ReferredBy UserID   `json:"referredBy,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Username   Username `json:"username,omitempty" example:"jdoe"`
		// Optional.
		PhoneNumber string `json:"phoneNumber,omitempty" example:"+12099216581"`
		// Optional. Required only if `phoneNumber` is set. Example:"Ef86A6021afCDe5673511376B2".
		PhoneNumberHash string `json:"phoneNumberHash,omitempty" example:"Ef86A6021afCDe5673511376B2"`
		// Optional.
		Email    string `json:"email,omitempty" example:"jdoe@gmail.com"`
		User     User   `json:"-"`
		ClientIP net.IP `json:"-" swaggerignore:"true"`
	} // @name CreateUserRequestBody  //nolint:godot // It's handled by swaggo.
	ModifyUserArg struct {
		// Optional.
		ProfilePicture *multipart.FileHeader `form:"profilePicture" json:"-"`
		// Optional. Example:"US".
		Country string `form:"country" json:"country,omitempty"`
		// Optional. Example:"New York".
		City string `form:"city" json:"city,omitempty"`
		// Example:"jdoe".
		Username Username `form:"username" json:"username,omitempty"`
		// Optional. Required only if `lastName` is set. Example:"John".
		FirstName string `form:"firstName" json:"firstName,omitempty"`
		// Optional. Required only if `firstName` is set.  Example:"Doe".
		LastName string `form:"lastName" json:"lastName,omitempty"`
		// Optional. Example:"+12099216581".
		PhoneNumber string `form:"phoneNumber" json:"phoneNumber,omitempty" `
		// Optional. Required only if `phoneNumber` is set. Example:"Ef86A6021afCDe5673511376B2".
		PhoneNumberHash      string `form:"phoneNumberHash" json:"phoneNumberHash,omitempty"`
		confirmedPhoneNumber string `example:"+12099216581"`
		// Optional. Example:"jdoe@gmail.com".
		Email string `form:"email" json:"email,omitempty"`
		// Optional. Example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2".
		AgendaPhoneNumberHashes string `form:"agendaPhoneNumberHashes" json:"agendaPhoneNumberHashes,omitempty"`
		User                    User   `json:"-"`
	} // @name ModifyUserRequestBody  //nolint:godot // It's handled by swaggo.
	GetUsersArg struct {
		UserID  UserID `json:"userId" swaggerignore:"true"`
		Keyword string `form:"keyword" json:"keyword" example:"john"`
		Limit   uint64 `form:"limit" json:"limit" maximum:"1000" example:"10"`
		Offset  uint64 `form:"offset" json:"offset" example:"5"`
	}
	GetTopCountriesArg struct {
		Keyword string `form:"keyword" json:"keyword" example:"united states"`
		Limit   uint64 `form:"limit" json:"limit" maximum:"1000" example:"20"`
		Offset  uint64 `form:"offset" json:"offset" example:"5"`
	}
	GetReferralAcquisitionHistoryArg struct {
		UserID UserID `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Days   uint64 `form:"days" maximum:"30" example:"5"`
	}
	GetReferralsArg struct {
		UserID UserID `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Type   string `form:"type" example:"T1" enums:"T1,T2,CONTACTS"`
		Limit  uint64 `form:"limit" maximum:"1000" example:"10"` // 10 by default.
		Offset uint64 `form:"offset" example:"5"`
	}
)

// Private API.

const (
	applicationYamlKey                     = "users"
	defaultUserImage                       = "default-user-image.jpg"
	add                arithmeticOperation = "+"
	subtract           arithmeticOperation = "-"
	expirationDeadline                     = 24 * stdlibtime.Hour
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
		close        func() error
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
