// SPDX-License-Identifier: ice License 1.0

package main

import (
	"regexp"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
)

// Public API.

type (
	GetUsersArg struct {
		Keyword string `form:"keyword" required:"true" example:"john"`
		Limit   uint64 `form:"limit" maximum:"1000" example:"10"` // 10 by default.
		Offset  uint64 `form:"offset" example:"5"`
	}
	GetUserByIDArg struct {
		UserID string `uri:"userId" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	GetUserByUsernameArg struct {
		Username string `form:"username" required:"true" example:"jdoe"`
	}
	GetTopCountriesArg struct {
		Keyword string `form:"keyword" example:"united states"`
		Limit   uint64 `form:"limit" maximum:"1000" example:"10"` // 10 by default.
		Offset  uint64 `form:"offset" example:"5"`
	}
	GetUserGrowthArg struct {
		TZ   string `form:"tz" example:"+4:30"`
		Days uint64 `form:"days" example:"7"`
	}
	GetReferralAcquisitionHistoryArg struct {
		UserID string `uri:"userId" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Days   uint64 `form:"days" maximum:"30" example:"5"`
	}
	GetReferralsArg struct {
		UserID string `uri:"userId" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Type   string `form:"type" required:"true" example:"T1" enums:"T1,T2,CONTACTS"`
		Limit  uint64 `form:"limit" maximum:"1000" example:"10"` // 10 by default.
		Offset uint64 `form:"offset" example:"5"`
	}
	User struct {
		*users.UserProfile
		Checksum string `json:"checksum,omitempty" example:"1232412415326543647657"`
	}
)

// Private API.

const (
	applicationYamlKey                  = "cmd/eskimo"
	swaggerRoot                         = "/users/r"
	everythingNotAllowedInUsernameRegex = `[^.a-zA-Z0-9]+`
)

// Values for server.ErrorResponse#Code.
const (
	userNotFoundErrorCode      = "USER_NOT_FOUND"
	invalidUsernameErrorCode   = "INVALID_USERNAME"
	invalidKeywordErrorCode    = "INVALID_KEYWORD"
	invalidPropertiesErrorCode = "INVALID_PROPERTIES"

	requestingUserIDCtxValueKey = "requestingUserIDCtxValueKey"

	adminRole = "admin"
)

var (
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg                                   config
	everythingNotAllowedInUsernamePattern = regexp.MustCompile(everythingNotAllowedInUsernameRegex)
)

type (
	// | service implements server.State and is responsible for managing the state and lifecycle of the package.
	service struct {
		usersRepository users.Repository
		iceClient       emaillink.IceUserIDClient
	}
	config struct {
		Host    string `yaml:"host"`
		Version string `yaml:"version"`
	}
)
