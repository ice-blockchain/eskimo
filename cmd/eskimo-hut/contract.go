// SPDX-License-Identifier: BUSL-1.1

package main

import (
	_ "embed"
	"mime/multipart"

	"github.com/ice-blockchain/eskimo/users"
)

// Public API.

type (
	CreateUserArg struct {
		// Optional.
		ReferredBy string `json:"referredBy" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Username   string `json:"username" required:"true" example:"jdoe"`
		// Optional.
		PhoneNumber string `json:"phoneNumber" example:"+12099216581"`
		// Optional. Required only if `phoneNumber` is set.
		PhoneNumberHash string `json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		// Optional.
		Email string `json:"email" example:"jdoe@gmail.com"`
	} // @name CreateUserRequestBody  //nolint:godot // It's handled by swaggo.
	ModifyUserArg struct {
		UserID string `uri:"userId" swaggerignore:"true" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		// Optional.
		ProfilePicture *multipart.FileHeader `form:"profilePicture" formMultipart:"profilePicture" swaggerignore:"true"`
		// Optional. Example:`US`.
		Country string `form:"country" formMultipart:"country"`
		// Optional. Example:`New York`.
		City string `form:"city" formMultipart:"city"`
		// Optional. Example:`jdoe`.
		Username string `form:"username" formMultipart:"username"`
		// Optional. Required only if `lastName` is set. Example:`John`.
		FirstName string `form:"firstName" formMultipart:"firstName"`
		// Optional. Required only if `firstName` is set. Example:`Doe`.
		LastName string `form:"lastName" formMultipart:"lastName"`
		// Optional. Example:`+12099216581`.
		PhoneNumber string `form:"phoneNumber" formMultipart:"phoneNumber"`
		// Optional. Required only if `phoneNumber` is set. Example:`Ef86A6021afCDe5673511376B2`.
		PhoneNumberHash string `form:"phoneNumberHash" formMultipart:"phoneNumberHash"`
		// Optional. Example:`jdoe@gmail.com`.
		Email string `form:"email" formMultipart:"email"`
		// Optional. Example:`Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2`.
		AgendaPhoneNumberHashes string `form:"agendaPhoneNumberHashes" formMultipart:"agendaPhoneNumberHashes"`
	} // @name ModifyUserRequestBody  //nolint:godot // It's handled by swaggo.
	DeleteUserArg struct {
		UserID string `uri:"userId" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	ValidatePhoneNumberArg struct {
		UserID          string `uri:"userId" swaggerignore:"true" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber     string `json:"phoneNumber" required:"true" example:"+12345678"`
		PhoneNumberHash string `json:"phoneNumberHash" required:"true" example:"Ef86A6021afCDe5673511376B2"`
		ValidationCode  string `json:"validationCode" required:"true" example:"1234"`
	} // @name ValidatePhoneNumberRequestBody  //nolint:godot // It's handled by swaggo.
	GetDeviceLocationArg struct {
		// Optional. Set it to `-` if unknown.
		UserID string `uri:"userId" required:"true" allowUnauthorized:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		// Optional. Set it to `-` if unknown.
		DeviceUniqueID string `uri:"deviceUniqueId" required:"true" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
	}
	ModifyDeviceSettingsArg struct {
		// Optional.
		NotificationSettings *users.NotificationSettings `json:"notificationSettings"`
		// Optional.
		Language *string `json:"language" example:"en"`
		// Optional.
		DisableAllNotifications *bool  `json:"disableAllNotifications" example:"true"`
		UserID                  string `uri:"userId" required:"true" swaggerignore:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		DeviceUniqueID          string `uri:"deviceUniqueId" required:"true" swaggerignore:"true" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
	} // @name ModifyDeviceSettingsRequestBody  //nolint:godot // It's handled by swaggo.
	CreateDeviceSettingsArg struct {
		// Optional.
		NotificationSettings *users.NotificationSettings `json:"notificationSettings"`
		// Optional.
		Language *string `json:"language" example:"en"`
		// Optional.
		DisableAllNotifications *bool  `json:"disableAllNotifications" example:"true"`
		UserID                  string `uri:"userId" required:"true" swaggerignore:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		DeviceUniqueID          string `uri:"deviceUniqueId" required:"true" swaggerignore:"true" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
	} // @name CreateDeviceSettingsRequestBody  //nolint:godot // It's handled by swaggo.
	ReplaceDeviceMetadataArg struct {
		UserID         string `uri:"userId" required:"true" swaggerignore:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		DeviceUniqueID string `uri:"deviceUniqueId" required:"true" swaggerignore:"true" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
		users.DeviceMetadata
	} // @name ReplaceDeviceMetadataRequestBody  //nolint:godot // It's handled by swaggo.
)

// Private API.

const (
	applicationYamlKey = "cmd/eskimo-hut"
	swaggerRoot        = "/users/w"
)

// Values for server.ErrorResponse#Code.
const (
	invalidUsernameErrorCode             = "INVALID_USERNAME"
	userNotFoundErrorCode                = "USER_NOT_FOUND"
	duplicateUserErrorCode               = "CONFLICT_WITH_ANOTHER_USER"
	referralNotFoundErrorCode            = "REFERRAL_NOT_FOUND"
	invalidValidationCodeErrorCode       = "INVALID_VALIDATION_CODE"
	phoneValidationCodeExpiredErrorCode  = "PHONE_VALIDATION_EXPIRED"
	phoneValidationNotFoundErrorCode     = "PHONE_VALIDATION_NOT_FOUND"
	phoneNumberFormatInvalidErrorCode    = "INVALID_PHONE_NUMBER_FORMAT"
	phoneNumberInvalidErrorCode          = "INVALID_PHONE_NUMBER"
	invalidPropertiesErrorCode           = "INVALID_PROPERTIES"
	deviceSettingsNotFoundErrorCode      = "DEVICE_SETTINGS_NOT_FOUND"
	deviceSettingsAlreadyExistsErrorCode = "DEVICE_SETTINGS_ALREADY_EXISTS"
)

//
var (
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
)

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
