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
	User     struct {
		CreatedAt            time.Time            `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt            time.Time            `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		DeletedAt            *time.Time           `json:"deletedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		ID                   UserID               `json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Email                string               `form:"email,omitempty" json:"email" example:"jdoe@gmail.com"`
		FullName             string               `form:"fullName,omitempty" json:"fullName" example:"John Doe"`
		PhoneNumber          string               `form:"phoneNumber,omitempty" json:"phoneNumber" example:"+12099216581"`
		confirmedPhoneNumber string               `example:"+12099216581"`
		Username             string               `form:"username,omitempty" json:"username" example:"jdoe"`
		ReferredBy           UserID               `form:"referredBy,omitempty" json:"referredBy" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		ProfilePictureURL    string               `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
		ProfilePicture       multipart.FileHeader `json:"-"`
		// ISO 3166 country code.
		Country  string `json:"country" example:"us"`
		HashCode uint64 `json:"hashCode"`
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
		GetUser(context.Context, UserID) (*User, error)
		UsernameExists(context.Context, Username) (bool, error)
	}
)

// Private API.

const (
	applicationYamlKey = "users"
	defaultUserImage   = "default-user-image.jpg"
	tableCodes         = "PHONE_NUMBER_VALIDATION_CODES"
)

var (
	//go:embed DDL.lua
	ddl string
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
)

type (
	// | users implements the UserRepository and only handles everything related to `users`.
	users struct {
		mb messagebroker.Client
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
		_msgpack           struct{} `msgpack:",asArray"`
		ID                 UserID
		HashCode           uint64
		ReferredBy         UserID
		Username           Username
		Email              string
		FullName           string
		PhoneNumber        string
		ProfilePictureName string
		Country            string
		CreatedAt          uint64
		UpdatedAt          uint64
	}

	phoneNumberValidationCode struct {
		_msgpack       struct{} `msgpack:",asArray"`
		ID             UserID
		PhoneNumber    string
		ValidationCode string
		CreatedAt      uint64
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
