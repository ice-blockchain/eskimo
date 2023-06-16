// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"embed"
	"io"
	"mime/multipart"
	"text/template"
	stdlibtime "time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

type (
	UserModifier interface {
		ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) (emailValidation *users.EmailValidation, err error)
	}
	Client interface {
		io.Closer
		SendSignInLinkToEmail(ctx context.Context, emailValue, deviceUniqueID, language string) (loginSession, confirmationCode string, err error)
		SignIn(ctx context.Context, emailLinkPayload, confirmationCode string) error
		RegenerateTokens(ctx context.Context, prevToken string, customClaims *users.JSON) (tokens *Tokens, err error)
		Status(ctx context.Context, emailValue, deviceUniqueID string) (tokens *Tokens, err error)
	}
	ID struct {
		Email          string `json:"email,omitempty" example:"someone1@example.com"`
		DeviceUniqueID string `json:"deviceUniqueId,omitempty" example:"6FB988F3-36F4-433D-9C7C-555887E57EB2" db:"device_unique_id"`
	}
	Tokens struct {
		RefreshToken string `json:"refreshToken" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
		AccessToken  string `json:"accessToken" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"`  //nolint:lll // .
	}
)

var (
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("expired token")
	ErrNoConfirmationRequired = errors.New("no pending confirmation")

	ErrUserDataMismatch = errors.New("parameters were not equal to user data in db")
	ErrUserNotFound     = storage.ErrNotFound

	ErrConfirmationCodeWrong            = errors.New("wrong confirmation code provided")
	ErrConfirmationCodeAttemptsExceeded = errors.New("confirmation code attempts exceeded")
	ErrConfirmationCodeTimeout          = errors.New("confirmation code timeout")

	ErrStatusNotVerified     = errors.New("not verified")
	ErrNoPendingConfirmation = errors.New("no pending confirmation code")
	ErrNoPendingLoginSession = errors.New("no pending login session")
)

// Private API.

const (
	applicationYamlKey = "auth/email-link"
	jwtIssuer          = "ice.io"
	defaultLanguage    = "en"

	loginSessionCtxValueKey = "loginSessionCtxValueKey"

	ValidationEmailType    string = "validation"
	NotifyEmailChangedType string = "notify_changed"
)

type (
	languageCode = string
	client       struct {
		db           *storage.DB
		cfg          *config
		shutdown     func() error
		emailClient  email.Client
		authClient   auth.Client
		userModifier UserModifier
	}
	config struct {
		FromEmailName    string `yaml:"fromEmailName"`
		FromEmailAddress string `yaml:"fromEmailAddress"`
		EmailValidation  struct {
			AuthLink       string              `yaml:"authLink"`
			JwtSecret      string              `yaml:"jwtSecret"`
			ExpirationTime stdlibtime.Duration `yaml:"expirationTime" mapstructure:"expirationTime"`
		} `yaml:"emailValidation"`
		LoginSession struct {
			JwtSecret      string              `yaml:"jwtSecret"`
			ExpirationTime stdlibtime.Duration `yaml:"expirationTime" mapstructure:"expirationTime"`
		} `yaml:"loginSession"`
		ConfirmationCode struct {
			ExpirationTime        stdlibtime.Duration `yaml:"expirationTime"`
			MaxWrongAttemptsCount int64               `yaml:"maxWrongAttemptsCount"`
		} `yaml:"confirmationCode"`
	}
	magicLinkToken struct {
		*jwt.RegisteredClaims
		OTP            string `json:"otp" example:"c8f64979-9cea-4649-a89a-35607e734e68"`
		OldEmail       string `json:"oldEmail,omitempty"`
		NotifyEmail    string `json:"notifyEmail,omitempty"`
		DeviceUniqueID string `json:"deviceUniqueId,omitempty"`
	}
	loginFlowToken struct {
		*jwt.RegisteredClaims
		DeviceUniqueID string `json:"deviceUniqueId,omitempty"`
	}
	issuedTokenSeq struct {
		IssuedTokenSeq int64 `db:"issued_token_seq"`
	}
	minimalUser struct {
		CreatedAt                          *time.Time
		TokenIssuedAt                      *time.Time
		ConfirmationCodeCreatedAt          *time.Time
		CustomClaims                       *users.JSON `json:"customClaims,omitempty"`
		UserID                             string      `json:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Email                              string      `json:"email,omitempty" example:"someone1@example.com"`
		OTP                                string      `json:"otp,omitempty" example:"207d0262-2554-4df9-b954-08cb42718b25"`
		LoginSession                       string      `json:"loginSession,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJpY2UuaW8iLCJzdWIiOiJzdXV2b3JAZ21haWwuY29tIiwiZXhwIjoxNjg2ODU1MTY2LCJuYmYiOjE2ODY4NTM5NjYsImlhdCI6MTY4Njg1Mzk2NiwiZGV2aWNlVW5pcXVlSWQiOiI3MDA2M0FCQi1FNjlGLTRGRDItOEI4My05MEREMzcyODAyREEifQ.SD9MFnKkJGIVh6kkzQ9TGVpAkcApthxTFeOQkV9aJgs"` //nolint:lll // .
		Language                           string      `json:"language,omitempty" example:"en"`
		DeviceUniqueID                     string      `json:"deviceUniqueId,omitempty" example:"6FB988F3-36F4-433D-9C7C-555887E57EB2" db:"device_unique_id"`
		ConfirmationCode                   string      `json:"confirmationCode,omitempty" example:"123"`
		IssuedTokenSeq                     int64       `json:"issuedTokenSeq,omitempty" example:"1"`
		ConfirmationCodeWrongAttemptsCount int64       `json:"confirmationCodeWrongAttemptsCount,omitempty" example:"3" db:"confirmation_code_wrong_attempts_count"`
		HashCode                           int64       `json:"hashCode,omitempty" example:"43453546464576547"`
	}
	emailTemplate struct {
		subject, body *template.Template
		Subject       string `json:"subject"` //nolint:revive // That's intended.
		Body          string `json:"body"`    //nolint:revive // That's intended.
	}
)

// .
var (
	//go:embed DDL.sql
	ddl string
	//go:embed translations
	translations embed.FS
	//nolint:gochecknoglobals // Its loaded once at startup.
	allEmailLinkTemplates map[string]map[languageCode]*emailTemplate

	//nolint:gochecknoglobals // It's just for more descriptive validation messages.
	allEmailTypes = users.Enum[string]{
		"validation",
		"notify_changed",
	}
)
