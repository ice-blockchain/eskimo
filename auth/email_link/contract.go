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
		ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error
	}
	Client interface {
		IceUserIDClient
		SendSignInLinkToEmail(ctx context.Context, emailValue, deviceUniqueID, language string) (loginSession string, err error)
		SignIn(ctx context.Context, emailLinkPayload, confirmationCode string) error
		RegenerateTokens(ctx context.Context, prevToken string) (tokens *Tokens, err error)
		Status(ctx context.Context, loginSession string) (tokens *Tokens, emailConfirmed bool, err error)
		UpdateMetadata(ctx context.Context, userID string, metadata *users.JSON) (*users.JSON, error)
	}
	IceUserIDClient interface {
		io.Closer
		IceUserID(ctx context.Context, mail string) (iceID string, err error)
		Metadata(ctx context.Context, userID, emailAddress string) (metadata string, metadataFields *users.JSON, err error)
	}
	Tokens struct {
		RefreshToken string `json:"refreshToken,omitempty" example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
		AccessToken  string `json:"accessToken,omitempty"  example:"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODQzMjQ0NTYsImV4cCI6MTcxNTg2MDQ1NiwiYXVkIjoiIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIm90cCI6IjUxMzRhMzdkLWIyMWEtNGVhNi1hNzk2LTAxOGIwMjMwMmFhMCJ9.q3xa8Gwg2FVCRHLZqkSedH3aK8XBqykaIy85rRU40nM"` //nolint:lll // .
	}
	Metadata struct {
		UserID   string `json:"userId" example:"1c0b9801-cfb2-4c4e-b48a-db18ce0894f9"`
		Metadata string `json:"metadata"`
	}
)

var (
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("expired token")
	ErrNoConfirmationRequired = errors.New("no pending confirmation")

	ErrUserDataMismatch = errors.New("parameters were not equal to user data in db")
	ErrUserNotFound     = storage.ErrNotFound
	ErrUserDuplicate    = errors.New("such user already exists")

	ErrConfirmationCodeWrong            = errors.New("wrong confirmation code provided")
	ErrConfirmationCodeAttemptsExceeded = errors.New("confirmation code attempts exceeded")
	ErrStatusNotVerified                = errors.New("not verified")
	ErrNoPendingLoginSession            = errors.New("no pending login session")
	ErrUserBlocked                      = errors.New("user is blocked")
	ErrConfirmationInProgress           = errors.New("confirmation in progress")
)

// Private API.

const (
	applicationYamlKey = "auth/email-link"
	jwtIssuer          = "ice.io"
	defaultLanguage    = "en"

	signInEmailType        string = "signin"
	notifyEmailChangedType string = "notify_changed"
	modifyEmailType        string = "modify_email"

	iceIDPrefix = "ice_"

	textExtension = "txt"
	htmlExtension = "html"
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
		LoginSession     struct {
			JwtSecret string `yaml:"jwtSecret"`
		} `yaml:"loginSession"`
		EmailValidation struct {
			AuthLink       string              `yaml:"authLink"`
			JwtSecret      string              `yaml:"jwtSecret"`
			ExpirationTime stdlibtime.Duration `yaml:"expirationTime" mapstructure:"expirationTime"`
			BlockDuration  stdlibtime.Duration `yaml:"blockDuration"`
		} `yaml:"emailValidation"`
		ConfirmationCode struct {
			MaxWrongAttemptsCount int64 `yaml:"maxWrongAttemptsCount"`
		} `yaml:"confirmationCode"`
	}
	loginID struct {
		Email          string `json:"email,omitempty" example:"someone1@example.com"`
		DeviceUniqueID string `json:"deviceUniqueId,omitempty" example:"6FB988F3-36F4-433D-9C7C-555887E57EB2" db:"device_unique_id"`
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
		DeviceUniqueID   string `json:"deviceUniqueId,omitempty"`
		ConfirmationCode string `json:"confirmationCode,omitempty"`
	}
	emailLinkSignIn struct {
		CreatedAt                          *time.Time
		TokenIssuedAt                      *time.Time
		BlockedUntil                       *time.Time
		EmailConfirmedAt                   *time.Time
		Metadata                           *users.JSON `json:"metadata,omitempty"`
		UserID                             *string     `json:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Email                              string      `json:"email,omitempty" example:"someone1@example.com"`
		OTP                                string      `json:"otp,omitempty" example:"207d0262-2554-4df9-b954-08cb42718b25"`
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
	metadata struct {
		Metadata *users.JSON
		Email    *string
		UserID   *string
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
		signInEmailType,
		modifyEmailType,
		notifyEmailChangedType,
	}
)
