// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"github.com/ICE-Blockchain/eskimo/users"
	"github.com/ICE-Blockchain/wintr/server"
	"mime/multipart"
	"net"
)

// Public API.

const userNotFoundCode = "USER_NOT_FOUND"
const userDuplicateCode = "USER_DUPLICATE"

const defaultUserImage = "https://ice-staging.b-cdn.net/profile/default-user-image.jpg"

type (
	RequestCreateUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ClientIP          net.IP                   `json:"clientIP" swaggerignore:"true"`
		// `email` is optional.
		Email string `json:"email" example:"jdoe@gmail.com"`
		// `fullName` is optional.
		FullName string `json:"fullName" example:"John Doe"`
		// `phoneNumber` is optional.
		PhoneNumber string `json:"phoneNumber" example:"+12099216581"`
		Username    string `json:"username" example:"jdoe"`
		ReferredBy  string `json:"referredBy" example:"billy112"`
	}
	RequestModifyUser struct {
		Email             string                   `form:"email" json:"email" example:"jdoe@gmail.com"`
		FullName          string                   `form:"fullName" json:"fullName" example:"John Doe"`
		PhoneNumber       string                   `form:"phoneNumber" json:"phoneNumber" example:"+12099216581"`
		Username          string                   `form:"username" json:"username" example:"jdoe"`
		ProfilePicture    multipart.FileHeader     `form:"profilePicture"`
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `form:"-" json:"-" uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"` //nolint:lll
	}
	RequestDeleteUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"` //nolint:lll
	}
	RequestGetUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"` //nolint:lll
	}
	RequestValidateUsername struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		Username          string                   `form:"username" example:"jdoe"`
	}
	RequestValidatePhoneNumber struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ValidationCode    string                   `form:"validationCode" example:"232323232"`
		PhoneNumber       string                   `form:"phoneNumber" example:"+12099216581"`
	}
	RequestGetTopCountries struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		Limit             uint64                   `form:"limit" example:"20"`
		Offset            uint64                   `form:"offset" example:"5"`
	}
	RequestGetReferralAcquisitionHistory struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Days              uint64                   `form:"days" example:"5"`
	}
	RequestGetReferrals struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Type              string                   `form:"type" example:"T1"`
	}
)

// Private API.

const applicationYamlKey = "cmd/eskimo"

//nolint:gochecknoglobals // Because its loaded once, at runtime.
var cfg config

type (
	// | service implements server.State and is responsible for managing the state and lifecycle of the package.
	service struct {
		usersRepository users.Repository
	}
	config struct { //nolint:govet
		Host              string `yaml:"host"`
		Version           string `yaml:"version"`
		DefaultPagination struct {
			Limit    uint64 `yaml:"limit"`
			MaxLimit uint64 `yaml:"maxLimit"`
		} `yaml:"defaultPagination"`
	}
)
