// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"
	_ "embed"
	"io"
	stdlibtime "time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
)

// Public API.
type (
	Auth struct {
		Email string `json:"email" example:"jdoe@gmail.com"`
	}
	Processor interface {
		Repository
		StartEmailLinkAuth(ctx context.Context, a *Auth) error
		IssueRefreshTokenForMagicLink(ctx context.Context, emailLinkPayload string) (string, error)
		RenewRefreshToken(ctx context.Context, prevToken string) (string, error)
		IssueAccessToken(ctx context.Context, refreshToken string) (string, error)
	}
	Repository interface {
		io.Closer
	}
	// TODO: move to wintr.
	Token struct {
		*jwt.RegisteredClaims
		Role     string `json:"role" example:"1"`
		EMail    string `json:"email" example:"jdoe@example.com"`
		HashCode int64  `json:"hashCode,omitempty" example:"12356789"`
		Seq      int64  `json:"seq"`
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
)

type (
	repository struct {
		db          *storage.DB
		cfg         *config
		shutdown    func() error
		emailClient email.Client
	}
	processor struct {
		*repository
	}
	config struct {
		EmailValidation struct {
			AuthLink              string `yaml:"authLink"`
			FromEmailName         string `yaml:"fromEmailName"`
			FromEmailAddress      string `yaml:"fromEmailAddress"`
			EmailBodyHTMLTemplate string `mapstructure:"emailBodyHTMLTemplate" yaml:"emailBodyHTMLTemplate"` //nolint:tagliatelle // Nope.
			EmailSubject          string `yaml:"emailSubject"`
			ServiceName           string `yaml:"serviceName"`
		} `yaml:"emailValidation"`
		JWTSecret      string              `yaml:"jwtSecret" mapstructure:"jwtSecret"`
		ExpirationTime stdlibtime.Duration `yaml:"expirationTime" mapstructure:"expirationTime"`
		//TODO: move to wintr?
		RefreshExpirationTime stdlibtime.Duration `yaml:"refreshExpirationTime" mapstructure:"refreshExpirationTime"`
		AccessExpirationTime  stdlibtime.Duration `yaml:"accessExpirationTime" mapstructure:"accessExpirationTime"`
	}
	emailClaims struct {
		*jwt.RegisteredClaims
		OTP string `json:"otp" example:"c8f64979-9cea-4649-a89a-35607e734e68"`
	}

	issuedTokenSeq struct {
		IssuedTokenSeq int64 `db:"issued_token_seq"`
	}
)

var (
	//go:embed DDL.sql
	ddl string
)
