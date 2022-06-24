// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"regexp"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

// Public API.

type (
	RequestGetDeviceSettings struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.DeviceID
	}
	RequestGetUsers struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.GetUsersArg
	}
	RequestGetUserByID struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		UserID            string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	RequestGetUserByUsername struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		Username          string                   `form:"username" example:"jdoe"`
	}
	RequestGetTopCountries struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.GetTopCountriesArg
	}
	RequestGetReferralAcquisitionHistory struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.GetReferralAcquisitionHistoryArg
	}
	RequestGetReferrals struct {
		AuthenticatedUser server.AuthenticatedUser `json:"-" swaggerignore:"true"`
		users.GetReferralsArg
	}
)

// Private API.

const (
	applicationYamlKey = "cmd/eskimo"
	usernameRegex      = `^[\w\-.]{4,20}$`
)

// Values for server.ErrorResponse#Code.
const (
	userNotFoundErrorCode           = "USER_NOT_FOUND"
	invalidUsernameErrorCode        = "INVALID_USERNAME"
	deviceSettingsNotFoundErrorCode = "DEVICE_SETTINGS_NOT_FOUND"
	invalidPropertiesErrorCode      = "INVALID_PROPERTIES"
)

var (
	compiledUsernameRegex = regexp.MustCompile(usernameRegex)
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
)

type (
	// | service implements server.State and is responsible for managing the state and lifecycle of the package.
	service struct {
		usersRepository users.Repository
	}
	config struct {
		Host    string `yaml:"host"`
		Version string `yaml:"version"`
	}
)
