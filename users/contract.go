// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"
	"time"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
)

// Public API.

var (
	ErrNotFound                   = storage.ErrNotFound
	ErrDuplicate                  = storage.ErrDuplicate
	ErrInvalidPhoneValidationCode = errors.New("invalid phone validation code")
	ErrExpiredPhoneValidationCode = errors.New("expired phone validation code")
)

type (
	UserID   = string
	Username = string
	Offset   = uint64
	Limit    = uint64
	User     struct {
		CreatedAt               time.Time            `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt               time.Time            `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		DeletedAt               *time.Time           `json:"deletedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		ID                      UserID               `json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Email                   string               `form:"email,omitempty" json:"email" example:"jdoe@gmail.com"`
		FullName                string               `form:"fullName,omitempty" json:"fullName" example:"John Doe"`
		PhoneNumber             string               `form:"phoneNumber,omitempty" json:"phoneNumber" example:"+12099216581"`
		PhoneNumberHash         string               `form:"phoneNumberHash,omitempty" json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		AgendaPhoneNumberHashes string               `form:"agendaPhoneNumberHashes,omitempty" json:"agendaPhoneNumberHashes" example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2"` //nolint:lll // hash
		confirmedPhoneNumber    string               `example:"+12099216581"`
		Username                string               `form:"username,omitempty" json:"username" example:"jdoe"`
		ReferredBy              UserID               `form:"referredBy,omitempty" json:"referredBy" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		ProfilePictureURL       string               `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
		ProfilePicture          multipart.FileHeader `json:"-"`
		// ISO 3166 country code.
		Country  string `json:"country" example:"us"`
		HashCode uint64 `json:"hashCode"`
	}

	// Referral is a user acquired by other user (limited fields comparing to the user struct)
	// because of sql fetches only required fields too.
	Referral struct {
		ID                UserID `json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber       string `form:"phoneNumber,omitempty" json:"phoneNumber" example:"+12099216581"`
		Username          string `form:"username,omitempty" json:"username" example:"jdoe"`
		ProfilePictureURL string `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
		ExistsInAgenda    bool   `json:"existsInAgenda" example:"true"`
	}

	UserSnapshot struct {
		*User
		Before *User
	}
	ReferralAcquisition struct {
		Date time.Time `json:"date" example:"2022-01-03"`
		T1   uint64    `json:"t1" example:"22"`
		T2   uint64    `json:"t2" example:"13"`
	}
	CountryStatistics struct {
		// ISO 3166 country code.
		Country   string `json:"country" example:"us"`
		UserCount uint64 `json:"userCount" example:"12121212"`
	}
	PhoneNumberConfirmation struct {
		UserID         UserID `json:"id" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber    string `json:"phoneNumber" example:"+12345678"`
		ValidationCode string `json:"code" example:"1234"`
	}

	// Repository main API exposed that handles all the features(including internal/system ones) of this package.
	Repository interface {
		io.Closer
		ReadRepository
	}

	Processor interface {
		io.Closer
		ReadRepository
		WriteRepository
		CheckHealth(context.Context) error
	}

	WriteRepository interface {
		AddUser(context.Context, *User) error
		RemoveUser(context.Context, UserID) error
		ModifyUser(context.Context, *User) error
		ConfirmPhoneNumber(context.Context, *PhoneNumberConfirmation) error
	}

	ReadRepository interface {
		GetUserByUsername(context.Context, Username) (*User, error)
		GetUserByID(context.Context, UserID) (*User, error)
		GetTopCountries(context.Context, Limit, Offset) ([]*CountryStatistics, error)
		GetTier1Referrals(ctx context.Context, id UserID, limit Limit, offset Offset) ([]*Referral, error)
	}
)

// Private API.

const (
	applicationYamlKey                     = "users"
	defaultUserImage                       = "default-user-image.jpg"
	tableCodes                             = "PHONE_NUMBER_VALIDATION_CODES"
	Add                arithmeticOperation = "+"
	Substract          arithmeticOperation = "-"
)

var (
	//go:embed DDL.lua
	ddl string
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
)

type (
	arithmeticOperation string
	// | users implements the UserRepository and only handles everything related to `users`.
	users struct {
		mb messagebroker.Client
		db tarantool.Connector
	}

	usersSource struct {
		db tarantool.Connector
	}

	// | repository implements the public API that this package exposes.
	repository struct {
		close func() error
		ReadRepository
	}

	processor struct {
		close func() error
		ReadRepository
		WriteRepository
	}

	// | user is the internal (User) structure for deserialization from the DB
	// because it cannot deserialize time.Time or map/json structures properly.
	// !! Order of fields is crucial, so do not change it !!
	user struct {
		//nolint:unused // Because it is used by the msgpack library for marshalling/unmarshalling.
		_msgpack                struct{} `msgpack:",asArray"`
		ID                      UserID
		ReferredBy              UserID
		Username                Username
		Email                   string
		FullName                string
		PhoneNumber             string
		PhoneNumberHash         string
		AgendaPhoneNumberHashes string
		ProfilePictureName      string
		Country                 string
		HashCode                uint64
		CreatedAt               uint64
		UpdatedAt               uint64
	}

	phoneNumberValidationCode struct {
		//nolint:unused // Because it is used by the msgpack library for marshalling/unmarshalling.
		_msgpack        struct{} `msgpack:",asArray"`
		ID              UserID
		PhoneNumber     string
		PhoneNumberHash string
		ValidationCode  string
		CreatedAt       uint64
	}

	usersPerCountry struct {
		_msgpack  struct{} `msgpack:",asArray"` // nolint:unused // To insert we need asArray
		Country   string
		UserCount uint64
	}

	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		PictureStorage struct {
			URLUpload   string `yaml:"urlUpload"`
			URLDownload string `yaml:"urlDownload"`
			AccessKey   string `yaml:"accessKey"`
		} `yaml:"pictureStorage"`
		MessageBroker struct {
			Topics []struct {
				Name string `yaml:"name" json:"name"`
			} `yaml:"topics"`
		} `yaml:"messageBroker"`
		PhoneNumberValidation struct {
			TwilioCredentials struct {
				User     string `yaml:"user"`
				Password string `yaml:"password"`
			} `yaml:"twilioCredentials"`
			FromPhoneNumber string        `yaml:"fromPhoneNumber"`
			SmsTemplate     string        `yaml:"smsTemplate"`
			ExpirationTime  time.Duration `yaml:"expirationTime"`
		} `yaml:"phoneNumberValidation"`
	}
)
