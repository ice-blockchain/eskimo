// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"mime/multipart"
	"net"

	"github.com/ICE-Blockchain/eskimo/users"
	"github.com/ICE-Blockchain/wintr/server"
)

// Public API.
const (
	userNotFoundCode  = "USER_NOT_FOUND"
	userDuplicateCode = "USER_DUPLICATE"
	userBadRequest    = "USER_BAD_REQUEST"
)

type (
	RequestCreateUser struct {
		// `email` is optional.
		Email string `json:"email" example:"jdoe@gmail.com"`
		// `fullName` is optional.
		FullName string `json:"fullName" example:"John Doe"`
		// `phoneNumber` is optional.
		PhoneNumber       string                   `json:"phoneNumber" example:"+12099216581"`
		Username          string                   `json:"username" example:"jdoe"`
		ReferredBy        string                   `json:"referredBy" example:"billy112"`
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ClientIP          net.IP                   `json:"clientIP" swaggerignore:"true"`
	}
	RequestModifyUser struct {
		Email             string                   `form:"email" json:"email" example:"jdoe@gmail.com"`
		FullName          string                   `form:"fullName" json:"fullName" example:"John Doe"`
		PhoneNumber       string                   `form:"phoneNumber" json:"phoneNumber" example:"+12099216581"`
		Username          string                   `form:"username" json:"username" example:"jdoe"`
		ProfilePicture    multipart.FileHeader     `form:"profilePicture"`
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `form:"-" json:"-" uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	RequestDeleteUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	RequestValidateUsername struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		Username          string                   `json:"username" example:"jdoe"`
	}
	RequestValidatePhoneNumber struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ValidationCode    string                   `json:"validationCode" example:"232323232"`
		PhoneNumber       string                   `json:"phoneNumber" example:"+12099216581"`
	}
)

// Private API.

const applicationYamlKey = "cmd/eskimo-hut"

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
