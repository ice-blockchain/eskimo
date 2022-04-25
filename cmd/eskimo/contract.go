// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"github.com/ICE-Blockchain/eskimo/users"
	"github.com/ICE-Blockchain/wintr/server"
)

// Public API.

type (
	RequestGetUser struct {
		AuthenticatedUser server.AuthenticatedUser `json:"authenticatedUser" swaggerignore:"true"`
		ID                string                   `uri:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
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
const userNotFoundCode = "USER_NOT_FOUND"

//nolint:gochecknoglobals // Because its loaded once, at runtime.
var cfg config

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
