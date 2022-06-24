// SPDX-License-Identifier: BUSL-1.1

package main

import (
	_ "embed"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

// Public API.

type (
	RequestCreateUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.CreateUserArg
	}
	RequestModifyUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		UserID            users.UserID             `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		users.ModifyUserArg
	}
	RequestDeleteUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		UserID            string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	RequestValidatePhoneNumber struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.PhoneNumberValidation
	}
	RequestGetDeviceLocation struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.GetDeviceMetadataLocationArg
	}
	RequestModifyDeviceSettings struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.DeviceSettings
	}
	RequestReplaceDeviceMetadata struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.ReplaceDeviceMetadataArg
	}
)

// Private API.

const (
	applicationYamlKey = "cmd/eskimo-hut"
)

// Values for server.ErrorResponse#Code.
const (
	userNotFoundErrorCode               = "USER_NOT_FOUND"
	duplicateUserErrorCode              = "CONFLICT_WITH_ANOTHER_USER"
	invalidValidationCodeErrorCode      = "INVALID_VALIDATION_CODE"
	phoneValidationCodeExpiredErrorCode = "PHONE_VALIDATION_EXPIRED"
	phoneValidationNotFoundErrorCode    = "PHONE_VALIDATION_NOT_FOUND"
	phoneNumberFormatInvalidErrorCode   = "INVALID_PHONE_NUMBER_FORMAT"
	phoneNumberInvalidErrorCode         = "INVALID_PHONE_NUMBER"
	invalidPropertiesErrorCode          = "INVALID_PROPERTIES"
)

//nolint:gochecknoglobals // Because its loaded once, at runtime.
var cfg config

type (
	// | service implements server.State and is responsible for managing the state and lifecycle of the package.
	service struct {
		usersProcessor users.Processor
	}
	config struct {
		Host    string `yaml:"host"`
		Version string `yaml:"version"`
	}
)
