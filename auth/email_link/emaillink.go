// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
)

func StartProcessor(ctx context.Context, cancel context.CancelFunc, userModifier UserModifier) Processor {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	if cfg.JWTSecret == "" {
		module := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
		cfg.JWTSecret = os.Getenv(fmt.Sprintf("%s_JWT_SECRET", module))
		if cfg.JWTSecret == "" {
			cfg.JWTSecret = os.Getenv("JWT_SECRET")
		}
	}
	if cfg.JWTSecret == "" {
		log.Panic(errors.New("no jwt secret provided"))
	}
	if cfg.JWTSecret == "" {
		log.Panic(errors.New("no jwt secret provided"))
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
	if cfg.EmailValidation.ServiceName == "" {
		log.Panic("no service name provided")
	}
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	repo := &repository{
		cfg:          &cfg,
		shutdown:     db.Close,
		db:           db,
		emailClient:  email.New(applicationYamlKey),
		userModifier: userModifier,
	}

	return &processor{repo}
}

func (r *repository) Close() error {
	return errors.Wrap(r.shutdown(), "closing auth/emaillink repository failed")
}
