package emaillink

import (
	_ "embed"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/pkg/errors"
	"io"
	stdlibtime "time"
)

// Public API.
type (
	Processor interface {
		Repository
		VerifyMagicLink(jwt string) error
	}
	Repository interface {
		io.Closer
		//RefreshToken(jwt string) (string, error)
		//IssueRefreshToken(userId users.UserID) (string, error)
	}
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
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
	}
	emailClaims struct {
		jwt.RegisteredClaims
		OTP string `json:"otp" example:"c8f64979-9cea-4649-a89a-35607e734e68"`
	}
)

var (
	//go:embed DDL.sql
	ddl string
	cfg config
)
