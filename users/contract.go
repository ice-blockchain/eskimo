// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"io"
	"time"
)

// Public API.

type (
	User struct {
		CreatedAt         time.Time  `json:"createdAt" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt         time.Time  `json:"updatedAt" example:"2022-01-03T16:20:52.156534Z"`
		DeletedAt         *time.Time `json:"deletedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		ID                string     `json:"id" example:"226fcb86-fcce-458e-95f0-867e09c8c274"`
		Email             string     `form:"email" json:"email" example:"jdoe@gmail.com"`
		FullName          string     `form:"fullName" json:"fullName" example:"John Doe"`
		PhoneNumber       string     `form:"phoneNumber" json:"phoneNumber" example:"+12099216581"`
		Username          string     `form:"username" json:"username" example:"jdoe"`
		ReferredBy        string     `form:"referredBy" json:"referredBy" example:"billy112"`
		ProfilePictureURL string     `json:"profilePictureURL" example:"https://somecdn.com/p1.jpg"`
	}
	Repository interface {
		io.Closer
	}
)

// Private API.

type (
	repository struct {
	}
)
