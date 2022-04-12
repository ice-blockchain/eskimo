// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	_ "embed"
	messagebroker "github.com/ICE-Blockchain/wintr/connectors/message_broker"
	"github.com/ICE-Blockchain/wintr/connectors/storage"
	"github.com/framey-io/go-tarantool"
	"io"
	"time"
)

// Public API.

var (
	ErrNotFound = storage.ErrNotFound
)

type (
	UserID string
	User   struct {
		CreatedAt         time.Time `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt         time.Time `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		DeletedAt         *time.Time `json:"deletedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		ID                string    `json:"id,omitempty" example:"226fcb86-fcce-458e-95f0-867e09c8c274"`
		Email             string    `form:"email,omitempty" json:"email" example:"jdoe@gmail.com"`
		FullName          string    `form:"fullName,omitempty" json:"fullName" example:"John Doe"`
		PhoneNumber       string    `form:"phoneNumber,omitempty" json:"phoneNumber" example:"+12099216581"`
		Username          string    `form:"username,omitempty" json:"username" example:"jdoe"`
		ReferredBy        string    `form:"referredBy,omitempty" json:"referredBy" example:"billy112"`
		ProfilePictureURL string    `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
		// ISO 3166 country code.
		Country string `json:"country" example:"us"`
	}
	ReferralAcquisition struct {
		Date time.Time `json:"date" example:"2022-01-03"`
		T1   uint64    `json:"t1" example:"22"`
		T2   uint64    `json:"t2" example:"13"`
	}
	CountryStatistics struct {
		UserCount uint64 `json:"userCount" example:"12121212"`
		// ISO 3166 country code.
		Country string `json:"country" example:"us"`
	}

	// Repository main API exposed that handles all the features(including internal/system ones) of this package.
	Repository interface {
		io.Closer
		UserRepository
	}

	// UserRepository manages the database operations related to `users`.
	UserRepository interface {
		AddUser(context.Context, *User) error
		GetUser(context.Context, UserID) (*User, error)
		RemoveUser(context.Context, UserID) error
		ModifyUser(context.Context, *User) error
	}
)

// Private API.
const (
	applicationYamlKey                 = "users"
	messageBrokerProduceRecordDeadline = 25 * time.Second
)

var (
	//go:embed DDL.lua
	DDL string
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
)

type (
	// | messageBrokerProduceMessageResponseChanKey is the context.Context value key
	// that returns `chan error` that holds the outcome of a sendMessage operation.
	messageBrokerProduceMessageResponseChanKey struct{}

	// | users implements the UserRepository and only handles everything related to `users`.
	users struct {
		mb messagebroker.Client
		db tarantool.Connector
	}

	// | repository implements the public API that this package exposes.
	repository struct {
		close func() error
		UserRepository
	}

	// | user is the internal (User) structure for deserialization from the DB
	// because it cannot deserialize time.Time or map/json structures properly.
	// !! Order of fields is crucial, so do not change it !!
	user struct {
		_msgpack       struct{} `msgpack:",asArray"`
		ID             UserID
		ReferredBy     UserID
		Username       string
		Email          string
		FullName       string
		PhoneNumber    string
		ProfilePicture string
		CreatedAt      uint64
		UpdatedAt      uint64
		DeletedAt      uint64
	}

	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		MessageBroker struct {
			Topics []struct {
				Name string `yaml:"name" json:"name"`
			} `yaml:"topics"`
		} `yaml:"messageBroker"`
	}
)
