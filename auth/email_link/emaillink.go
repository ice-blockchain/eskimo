// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/auth"
	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
)

func NewClient(ctx context.Context, userModifier UserModifier) Client {
	cfg := loadConfiguration()
	cfg.validate()
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &client{
		cfg:          cfg,
		shutdown:     db.Close,
		db:           db,
		emailClient:  email.New(applicationYamlKey),
		authClient:   auth.New(ctx, applicationYamlKey),
		userModifier: userModifier,
	}
}

func (c *client) Close() error {
	return errors.Wrap(c.shutdown(), "closing auth/emaillink repository failed")
}

func loadConfiguration() *config {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	if cfg.EmailJWTSecret == "" {
		module := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
		cfg.EmailJWTSecret = os.Getenv(fmt.Sprintf("%s_EMAIL_JWT_SECRET", module))
		if cfg.EmailJWTSecret == "" {
			cfg.EmailJWTSecret = os.Getenv("EMAIL_JWT_SECRET")
		}
		// If specific one for emails for found - let's use the same one as wintr/ice/auth uses for token generation.
		if cfg.EmailJWTSecret == "" {
			module = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
			cfg.EmailJWTSecret = os.Getenv(fmt.Sprintf("%s_JWT_SECRET", module))
			if cfg.EmailJWTSecret == "" {
				cfg.EmailJWTSecret = os.Getenv("JWT_SECRET")
			}
		}
	}

	return &cfg
}

func (cfg *config) validate() {
	if cfg.EmailJWTSecret == "" {
		log.Panic(errors.New("no email jwt secret provided"))
	}
	if cfg.EmailValidation.AuthLink == "" {
		log.Panic("no auth link provided")
	}
	if cfg.EmailValidation.SignIn.EmailBodyHTMLTemplate == "" {
		log.Panic("no email body html template provided")
	}
	if cfg.EmailValidation.SignIn.EmailSubject == "" {
		log.Panic("no email subject provided")
	}
	if cfg.EmailValidation.NotifyChanged.EmailBodyHTMLTemplate == "" {
		log.Panic("no email body html template provided")
	}
	if cfg.EmailValidation.NotifyChanged.EmailSubject == "" {
		log.Panic("no email subject provided")
	}
	if cfg.EmailValidation.FromEmailAddress == "" {
		log.Panic("no from email address provided")
	}
	if cfg.EmailValidation.FromEmailName == "" {
		log.Panic("no from email name provided")
	}
}
