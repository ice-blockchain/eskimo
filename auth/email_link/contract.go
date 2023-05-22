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
)

// Public API.
type (
	Processor interface {
		Repository
		IssueRefreshToken(ctx context.Context, emailLinkPayload string) (string, error)
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
	}
)

var (
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("expired token")
	ErrNoConfirmationRequired = errors.New("no pending confirmation")
)

// Private API.
const (
	applicationYamlKey = "auth/email-link"
	jwtIssuer          = "ice.io"
)

type (
	repository struct {
		db       *storage.DB
		cfg      *config
		shutdown func() error
	}
	processor struct {
		*repository
	}
	config struct {
		JWTSecret      string              `yaml:"jwtSecret" mapstructure:"jwtSecret"`
		ExpirationTime stdlibtime.Duration `yaml:"expirationTime" mapstructure:"expirationTime"`
		//TODO: move to wintr?
		RefreshExpirationTime stdlibtime.Duration `yaml:"refreshExpirationTime" mapstructure:"refreshExpirationTime"`
		AccessExpirationTime  stdlibtime.Duration `yaml:"accessExpirationTime" mapstructure:"refreshExpirationTime"`
	}
	emailClaims struct {
		*jwt.RegisteredClaims
		OTP string `json:"otp" example:"c8f64979-9cea-4649-a89a-35607e734e68"`
	}
)

var (
	//go:embed DDL.sql
	ddl string
)
