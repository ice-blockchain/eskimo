// SPDX-License-Identifier: BUSL-1.1

package main

import (
	_ "embed"
	"mime/multipart"
	"net"

	"github.com/ice-blockchain/eskimo/countries"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

// Public API.

type (
	RequestCreateUser struct {
		// `email` is optional.
		Email string `json:"email" example:"jdoe@gmail.com"`
		// `fullName` is optional.
		FullName string `json:"fullName" example:"John Doe"`
		// `phoneNumber` is optional.
		PhoneNumber string `json:"phoneNumber" example:"+12099216581"`
		// `phoneNumberHash` is optional (because of phoneNumber is optional too).
		PhoneNumberHash string `form:"phoneNumberHash" json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		Username        string `json:"username" example:"jdoe"`
		// User's ID, so client app requests user by user name and provides ID here.
		ReferredBy        string                   `json:"referredBy" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ClientIP          net.IP                   `json:"clientIP" swaggerignore:"true"`
	}
	RequestModifyUser struct {
		Email                   string                   `form:"email" json:"email" example:"jdoe@gmail.com"`
		FullName                string                   `form:"fullName" json:"fullName" example:"John Doe"`
		PhoneNumber             string                   `form:"phoneNumber" json:"phoneNumber" example:"+12099216581"`
		PhoneNumberHash         string                   `form:"phoneNumberHash" json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		AgendaPhoneNumberHashes string                   `form:"agendaPhoneNumberHashes" json:"agendaPhoneNumberHashes" example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2"` //nolint:lll // hash
		Username                string                   `form:"username" json:"username" example:"jdoe"`
		ProfilePicture          multipart.FileHeader     `form:"profilePicture"`
		AuthenticatedUser       server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                      string                   `form:"-" json:"-" uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Country                 string                   `form:"country" json:"country" example:"us"`
	}
	RequestDeleteUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	RequestValidatePhoneNumber struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ValidationCode    string                   `json:"validationCode" example:"232323232"`
		PhoneNumber       string                   `json:"phoneNumber" example:"+12099216581"`
	}
)

// Private API.

const (
	applicationYamlKey = "cmd/eskimo-hut"
	userNotFoundCode   = "USER_NOT_FOUND"
	userDuplicateCode  = "USER_DUPLICATE"
	userBadRequest     = "USER_BAD_REQUEST"
	notAllowed         = "NOT_ALLOWED"
	userInvalidCode    = "INVALID_VALIDATION_CODE"
	userExpiredCode    = "EXPIRED_VALIDATION_CODE"
)

//nolint:gochecknoglobals // Because its loaded once, at runtime.
var cfg config

type (
	// | service implements server.State and is responsible for managing the state and lifecycle of the package.
	service struct {
		usersProcessor      users.Processor
		countriesRepository countries.Repository
	}
	config struct {
		Host    string `yaml:"host"`
		Version string `yaml:"version"`
	}
)
