// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"
	"time"

	"github.com/framey-io/go-tarantool"

	messagebroker "github.com/ICE-Blockchain/wintr/connectors/message_broker"
	"github.com/ICE-Blockchain/wintr/connectors/storage"
)

// Public API.

var (
	ErrNotFound  = storage.ErrNotFound
	ErrDuplicate = storage.ErrDuplicate
)

type (
	UserID   = string
	Username = string
	User     struct {
		CreatedAt         time.Time            `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt         time.Time            `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		DeletedAt         *time.Time           `json:"deletedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		ID                string               `json:"id,omitempty" example:"226fcb86-fcce-458e-95f0-867e09c8c274"`
		Email             string               `form:"email,omitempty" json:"email" example:"jdoe@gmail.com"`
		FullName          string               `form:"fullName,omitempty" json:"fullName" example:"John Doe"`
		PhoneNumber       string               `form:"phoneNumber,omitempty" json:"phoneNumber" example:"+12099216581"`
		Username          string               `form:"username,omitempty" json:"username" example:"jdoe"`
		ReferredBy        string               `form:"referredBy,omitempty" json:"referredBy" example:"billy112"`
		ProfilePictureURL string               `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
		ProfilePicture    multipart.FileHeader `json:"-"`
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

	// Repository main API exposed that handles all the features(including internal/system ones) of this package.
	Repository interface {
		ReadRepository
		WriteRepository
	}

	Processor interface {
		Repository
		CheckHealth(context.Context) error
	}

	WriteRepository interface {
		AddUser(context.Context, *User) error
		RemoveUser(context.Context, UserID) error
		ModifyUser(context.Context, *User) error
	}

	ReadRepository interface {
		io.Closer
		GetUser(context.Context, UserID) (*User, error)
		UsernameExists(context.Context, Username) (bool, error)
	}
)

const (
	applicationYamlKey = "users"
	defaultUserImage   = "default-user-image.jpg"
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
	user struct { //nolint:govet // This is about DB
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
	}
)

func (u *users) Close() error {
	return nil
}
