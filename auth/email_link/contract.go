// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"
	stdlibtime "time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
)

// Public API.
type (
	UserModifier interface {
		ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error
	}
	Processor interface {
		Repository
		StartEmailLinkAuth(ctx context.Context, userEmail string) error
		FinishLoginUsingMagicLink(ctx context.Context, emailLinkPayload string) (refresh string, access string, err error)
		RenewTokens(ctx context.Context, prevToken string, customClaims *users.JSON) (refresh string, access string, err error)
	}
	Repository interface {
		io.Closer
	}
	// TODO: move to wintr.
	Token struct {
		*jwt.RegisteredClaims
		Custom   *map[string]any `json:"custom,omitempty"`
		Role     string          `json:"role" example:"1"`
		Email    string          `json:"email" example:"jdoe@example.com"`
		HashCode int64           `json:"hashCode,omitempty" example:"12356789"`
		Seq      int64           `json:"seq"`
	}
)

var (
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("expired token")
	ErrNoConfirmationRequired = errors.New("no pending confirmation")

	ErrUserDataMismatch = errors.New("user data was updated in db")
	ErrUserNotFound     = storage.ErrNotFound
)

// Private API.
const (
	applicationYamlKey = "auth/email-link"
	jwtIssuer          = "ice.io"

	defaultRole = "app"
)

type (
	repository struct {
		db           *storage.DB
		cfg          *config
		shutdown     func() error
		emailClient  email.Client
		userModifier UserModifier
	}
	processor struct {
		*repository
	}
	config struct {
		EmailValidation struct {
			AuthLink         string `yaml:"authLink"`
			FromEmailName    string `yaml:"fromEmailName"`
			FromEmailAddress string `yaml:"fromEmailAddress"`
			ServiceName      string `yaml:"serviceName"`
			SignIn           struct {
				EmailBodyHTMLTemplate string `mapstructure:"emailBodyHTMLTemplate" yaml:"emailBodyHTMLTemplate"` //nolint:tagliatelle // Nope.
				EmailSubject          string `yaml:"emailSubject"`
			} `yaml:"signIn"`
			NotifyChanged struct {
				EmailBodyHTMLTemplate string `mapstructure:"emailBodyHTMLTemplate" yaml:"emailBodyHTMLTemplate"` //nolint:tagliatelle // Nope.
				EmailSubject          string `yaml:"emailSubject"`
			} `yaml:"notifyChanged"`
		} `yaml:"emailValidation"`
		JWTSecret           string              `yaml:"jwtSecret" mapstructure:"jwtSecret"`
		EmailExpirationTime stdlibtime.Duration `yaml:"emailExpirationTime" mapstructure:"emailExpirationTime"`
		// TODO: move to wintr?
		RefreshExpirationTime stdlibtime.Duration `yaml:"refreshExpirationTime" mapstructure:"refreshExpirationTime"`
		AccessExpirationTime  stdlibtime.Duration `yaml:"accessExpirationTime" mapstructure:"accessExpirationTime"`
	}
	emailClaims struct {
		*jwt.RegisteredClaims
		OTP         string `json:"otp" example:"c8f64979-9cea-4649-a89a-35607e734e68"`
		OldEmail    string `json:"oldEmail,omitempty"`
		NotifyEmail string `json:"notifyEmail,omitempty"`
	}

	issuedTokenSeq struct {
		IssuedTokenSeq int64 `db:"issued_token_seq"`
	}
	minimalUser struct {
		CustomClaims *users.JSON
		ID           string
		Email        string
		HashCode     int64
	}
)

//go:embed DDL.sql
var ddl string
