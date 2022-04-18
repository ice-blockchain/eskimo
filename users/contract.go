// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"io"
	"time"
)

// Public API.

type (
	User struct {
		CreatedAt         time.Time  `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt         time.Time  `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		DeletedAt         *time.Time `json:"deletedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		ID                string     `json:"id,omitempty" example:"226fcb86-fcce-458e-95f0-867e09c8c274"`
		Email             string     `form:"email,omitempty" json:"email" example:"jdoe@gmail.com"`
		FullName          string     `form:"fullName,omitempty" json:"fullName" example:"John Doe"`
		PhoneNumber       string     `form:"phoneNumber,omitempty" json:"phoneNumber" example:"+12099216581"`
		Username          string     `form:"username,omitempty" json:"username" example:"jdoe"`
		ReferredBy        string     `form:"referredBy,omitempty" json:"referredBy" example:"billy112"`
		ProfilePictureURL string     `json:"profilePictureURL,omitempty" example:"https://somecdn.com/p1.jpg"`
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
	Repository interface {
		io.Closer
	}
	Processor interface {
		Repository
		CheckHealth(context.Context) error
	}
)

// Private API.

type (
	repository struct {
	}
	processor struct {
	}
)
