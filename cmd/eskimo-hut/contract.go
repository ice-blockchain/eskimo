// SPDX-License-Identifier: ice License 1.0

package main

import (
	_ "embed"
	"mime/multipart"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
)

// Public API.

type (
	CreateUserRequestBody struct {
		// Optional. Example: `{"key1":{"something":"somethingElse"},"key2":"value"}`.
		ClientData *users.JSON `json:"clientData"`
		// Optional.
		PhoneNumber string `json:"phoneNumber" example:"+12099216581"`
		// Optional. Required only if `phoneNumber` is set.
		PhoneNumberHash string `json:"phoneNumberHash" example:"Ef86A6021afCDe5673511376B2"`
		// Optional.
		Email string `json:"email" example:"jdoe@gmail.com"`
		// Optional.
		FirstName string `json:"firstName" example:"John"`
		// Optional.
		LastName string `json:"lastName" example:"Doe"`
		// Optional. Defaults to `en`.
		Language string `json:"language" example:"en"`
	}
	ModifyUserRequestBody struct {
		UserID string `uri:"userId" swaggerignore:"true" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		// Optional. Example:`did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2`.
		ReferredBy string `form:"referredBy" formMultipart:"referredBy"`
		// Optional. Example: Array of [`globalRank`,`referralCount`,`level`,`role`,`badges`].
		HiddenProfileElements      *users.Enum[users.HiddenProfileElement] `form:"hiddenProfileElements" formMultipart:"hiddenProfileElements" swaggertype:"array,string" enums:"globalRank,referralCount,level,role,badges"` //nolint:lll // .
		ClearHiddenProfileElements *bool                                   `form:"clearHiddenProfileElements" formMultipart:"clearHiddenProfileElements"`
		// Optional. Example: `{"key1":{"something":"somethingElse"},"key2":"value"}`.
		ClientData *string     `form:"clientData" formMultipart:"clientData"`
		clientData *users.JSON //nolint:revive // It's meant for internal use only.
		// Optional. Example:`true`.
		ResetProfilePicture *bool `form:"resetProfilePicture" formMultipart:"resetProfilePicture"`
		// Optional.
		ProfilePicture *multipart.FileHeader `form:"profilePicture" formMultipart:"profilePicture" swaggerignore:"true"`
		// Optional. Example:`US`.
		Country string `form:"country" formMultipart:"country"`
		// Optional. Example:`New York`.
		City string `form:"city" formMultipart:"city"`
		// Optional. Example:`jdoe`.
		Username string `form:"username" formMultipart:"username"`
		// Optional. Example:`John`.
		FirstName string `form:"firstName" formMultipart:"firstName"`
		// Optional. Example:`Doe`.
		LastName string `form:"lastName" formMultipart:"lastName"`
		// Optional. Example:`+12099216581`.
		PhoneNumber string `form:"phoneNumber" formMultipart:"phoneNumber"`
		// Optional. Required only if `phoneNumber` is set. Example:`Ef86A6021afCDe5673511376B2`.
		PhoneNumberHash string `form:"phoneNumberHash" formMultipart:"phoneNumberHash"`
		// Optional. Example:`jdoe@gmail.com`.
		Email string `form:"email" formMultipart:"email"`
		// Optional. Example:`Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2`.
		AgendaPhoneNumberHashes string `form:"agendaPhoneNumberHashes" formMultipart:"agendaPhoneNumberHashes"`
		// Optional. Example:`some hash`.
		BlockchainAccountAddress string `form:"blockchainAccountAddress" formMultipart:"blockchainAccountAddress"`
		// Optional. Example:`en`.
		Language string `form:"language" formMultipart:"language"`
		// Optional. Example:`1232412415326543647657`.
		Checksum string `form:"checksum" formMultipart:"checksum"`
	}
	DeleteUserArg struct {
		UserID string `uri:"userId" required:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	GetDeviceLocationArg struct {
		// Optional. Set it to `-` if unknown.
		UserID string `uri:"userId" required:"true" allowUnauthorized:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		// Optional. Set it to `-` if unknown.
		DeviceUniqueID string `uri:"deviceUniqueId" required:"true" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
	}
	ReplaceDeviceMetadataRequestBody struct {
		UserID         string `uri:"userId" allowUnauthorized:"true" required:"true" swaggerignore:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"` //nolint:lll // .
		DeviceUniqueID string `uri:"deviceUniqueId" required:"true" swaggerignore:"true" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
		Bogus          string `json:"bogus" swaggerignore:"true"` // It's just for the router to register the JSON body binder.
		users.DeviceMetadata
	}
	SendSignInLinkToEmailRequestArg struct {
		Email          string `json:"email" allowUnauthorized:"true" required:"true" example:"jdoe@gmail.com"`
		DeviceUniqueID string `json:"deviceUniqueId" required:"true" example:"70063ABB-E69F-4FD2-8B83-90DD372802DA"`
		Language       string `json:"language" required:"true" example:"en"`
	}
	StatusArg struct {
		LoginSession string `json:"loginSession" allowUnauthorized:"true" required:"true" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
	}
	ModifyUserResponse struct {
		*User
		LoginSession string `json:"loginSession,omitempty" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
	}
	User struct {
		*users.User
		Checksum string `json:"checksum,omitempty" example:"1232412415326543647657"`
	}
	Auth struct {
		LoginSession string `json:"loginSession" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
	}
	RefreshedToken struct {
		*emaillink.Tokens
	}
	MagicLinkPayload struct {
		EmailToken       string `form:"token" required:"true" allowUnauthorized:"true" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
		ConfirmationCode string `form:"confirmationCode" required:"true" example:"999"`
	}
	RefreshToken struct {
		// Optional. In null - current claims are used, if any value - it would be overwritten. Example {"role":"new_role"}.
		CustomClaims  *users.JSON `json:"customClaims"`
		Authorization string      `header:"Authorization" swaggerignore:"true" required:"true" allowForbiddenWriteOperation:"true" allowUnauthorized:"true"`
	}
)

// Private API.

const (
	applicationYamlKey = "cmd/eskimo-hut"
	swaggerRoot        = "/users/w"
)

// Values for server.ErrorResponse#Code.
const (
	deviceMetadataAppUpdateRequireErrorCode = "UPDATE_REQUIRED"
	invalidUsernameErrorCode                = "INVALID_USERNAME"
	userNotFoundErrorCode                   = "USER_NOT_FOUND"
	duplicateUserErrorCode                  = "CONFLICT_WITH_ANOTHER_USER"
	referralNotFoundErrorCode               = "REFERRAL_NOT_FOUND"
	raceConditionErrorCode                  = "RACE_CONDITION"
	invalidPropertiesErrorCode              = "INVALID_PROPERTIES"

	linkExpiredErrorCode    = "EXPIRED_LINK"
	invalidOTPCodeErrorCode = "INVALID_OTP"
	dataMismatchErrorCode   = "DATA_MISMATCH"

	confirmationCodeNotFoundErrorCode         = "CONFIRMATION_CODE_NOT_FOUND"
	confirmationCodeAttemptsExceededErrorCode = "CONFIRMATION_CODE_ATTEMPTS_EXCEEDED"
	confirmationCodeWrongErrorCode            = "CONFIRMATION_CODE_WRONG"

	noPendingLoginSessionErrorCode = "NO_PENDING_LOGIN_SESSION"
	statusNotVerifiedErrorCode     = "NOT_VERIFIED"
)

// .
var (
	//nolint:gochecknoglobals // Because its loaded once, at runtime.
	cfg config
)

type (
	// | service implements server.State and is responsible for managing the state and lifecycle of the package.
	service struct {
		usersProcessor      users.Processor
		authEmailLinkClient emaillink.Client
	}
	config struct {
		Host    string `yaml:"host"`
		Version string `yaml:"version"`
	}
)
