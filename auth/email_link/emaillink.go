// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ice-blockchain/wintr/time"
	"math/rand"
	"os"
	"strings"
	"text/template"
	stdlibtime "time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/auth"
	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
)

//nolint:gochecknoinits // We load embedded stuff at runtime.
func init() {
	loadEmailMagicLinkTranslationTemplates()
}

func NewClient(ctx context.Context, userModifier UserModifier, authClient auth.Client) Client {
	cfg := loadConfiguration()
	cfg.validate()
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	cl := &client{
		cfg:          cfg,
		shutdown:     db.Close,
		db:           db,
		emailClient:  email.New(applicationYamlKey),
		authClient:   authClient,
		userModifier: userModifier,
	}
	go cl.startOldLoginAttemptsCleaner(ctx)

	return cl
}

func NewROClient(ctx context.Context) IceUserIDClient {
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &client{
		shutdown: db.Close,
		db:       db,
	}
}

func (c *client) Close() error {
	return errors.Wrap(c.shutdown(), "closing auth/emaillink repository failed")
}

func loadConfiguration() *config {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	loadEmailValidationConfiguration(&cfg)
	loadLoginSessionConfiguration(&cfg)

	return &cfg
}

func loadEmailValidationConfiguration(cfg *config) {
	if cfg.EmailValidation.JwtSecret == "" {
		module := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
		cfg.EmailValidation.JwtSecret = os.Getenv(fmt.Sprintf("%s_EMAIL_JWT_SECRET", module))
		if cfg.EmailValidation.JwtSecret == "" {
			cfg.EmailValidation.JwtSecret = os.Getenv("EMAIL_JWT_SECRET")
		}
		// If specific one for emails for found - let's use the same one as wintr/auth/ice uses for token generation.
		if cfg.EmailValidation.JwtSecret == "" {
			module = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
			cfg.EmailValidation.JwtSecret = os.Getenv(fmt.Sprintf("%s_JWT_SECRET", module))
			if cfg.EmailValidation.JwtSecret == "" {
				cfg.EmailValidation.JwtSecret = os.Getenv("JWT_SECRET")
			}
		}
	}
}

func loadLoginSessionConfiguration(cfg *config) {
	if cfg.LoginSession.JwtSecret == "" {
		module := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
		cfg.LoginSession.JwtSecret = os.Getenv(fmt.Sprintf("%s_LOGIN_JWT_SECRET", module))
		if cfg.LoginSession.JwtSecret == "" {
			cfg.LoginSession.JwtSecret = os.Getenv("LOGIN_JWT_SECRET")
		}
		// If specific one for emails for found - let's use the same one as wintr/auth/ice uses for token generation.
		if cfg.LoginSession.JwtSecret == "" {
			module = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(applicationYamlKey, "-", "_"), "/", "_"))
			cfg.LoginSession.JwtSecret = os.Getenv(fmt.Sprintf("%s_JWT_SECRET", module))
			if cfg.LoginSession.JwtSecret == "" {
				cfg.LoginSession.JwtSecret = os.Getenv("JWT_SECRET")
			}
		}
	}
}

func (cfg *config) validate() {
	if cfg.EmailValidation.JwtSecret == "" {
		log.Panic(errors.New("no email jwt secret provided"))
	}
	if cfg.LoginSession.JwtSecret == "" {
		log.Panic(errors.New("no login session jwt secret provided"))
	}
	if cfg.EmailValidation.AuthLink == "" {
		log.Panic("no auth link provided")
	}
	if cfg.FromEmailAddress == "" {
		log.Panic("no from email address provided")
	}
	if cfg.FromEmailName == "" {
		log.Panic("no from email name provided")
	}
	if cfg.EmailValidation.ExpirationTime == 0 {
		log.Panic("no expiration time provided for email validation")
	}
	if cfg.ConfirmationCode.MaxWrongAttemptsCount == 0 {
		log.Panic("no max wrong attempts count provided for confirmation code")
	}
}

func (t *emailTemplate) getSubject(data any) string {
	if data == nil {
		return t.Subject
	}
	bf := new(bytes.Buffer)
	log.Panic(errors.Wrapf(t.subject.Execute(bf, data), "failed to execute subject template for data:%#v", data))

	return bf.String()
}

func (t *emailTemplate) getBody(data any) string {
	if data == nil {
		return t.Body
	}
	bf := new(bytes.Buffer)
	log.Panic(errors.Wrapf(t.body.Execute(bf, data), "failed to execute body template for data:%#v", data))

	return bf.String()
}

func loadEmailMagicLinkTranslationTemplates() { //nolint:funlen,gocognit,revive // .
	const totalLanguages = 50
	allEmailLinkTemplates = make(map[string]map[languageCode]*emailTemplate, len(allEmailTypes))
	for _, emailType := range allEmailTypes {
		files, err := translations.ReadDir(fmt.Sprintf("translations/email/%v", emailType))
		if err != nil {
			panic(err)
		}
		allEmailLinkTemplates[emailType] = make(map[languageCode]*emailTemplate, totalLanguages)
		for _, file := range files {
			content, fErr := translations.ReadFile(fmt.Sprintf("translations/email/%v/%v", emailType, file.Name()))
			if fErr != nil {
				panic(fErr)
			}
			fileName := strings.Split(file.Name(), ".")
			language := fileName[0]
			ext := fileName[1]
			var tmpl emailTemplate
			switch ext {
			case textExtension:
				err = json.Unmarshal(content, &tmpl)
				if err != nil {
					panic(err)
				}
				subject := template.Must(template.New(fmt.Sprintf("email_%v_%v_subject", emailType, language)).Parse(tmpl.Subject))
				if allEmailLinkTemplates[emailType][language] != nil {
					allEmailLinkTemplates[emailType][language].subject = subject
					allEmailLinkTemplates[emailType][language].Subject = tmpl.Subject
				} else {
					tmpl.subject = subject
					allEmailLinkTemplates[emailType][language] = &tmpl
				}
			case htmlExtension:
				body := template.Must(template.New(fmt.Sprintf("email_%v_%v_body", emailType, language)).Parse(string(content)))
				if allEmailLinkTemplates[emailType][language] != nil {
					allEmailLinkTemplates[emailType][language].body = body
					allEmailLinkTemplates[emailType][language].Body = string(content)
				} else {
					tmpl.body = body
					tmpl.Body = string(content)
					allEmailLinkTemplates[emailType][language] = &tmpl
				}
			default:
				log.Panic("wrong translation file extension")
			}
		}
	}
}

func parseJwtToken(jwtToken, secret string, res jwt.Claims) error {
	if _, err := jwt.ParseWithClaims(jwtToken, res, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, errors.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		if iss, err := token.Claims.GetIssuer(); err != nil || iss != jwtIssuer {
			return nil, errors.Wrapf(ErrInvalidToken, "invalid issuer:%v", iss)
		}

		return []byte(secret), nil
	}); err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return errors.Wrapf(ErrExpiredToken, "expired or not valid yet token:%v", jwtToken)
		}

		return errors.Wrapf(ErrInvalidToken, "invalid token:%v (token:%v)", err.Error(), jwtToken)
	}

	return nil
}

func (c *client) deleteOldLoginAttempts(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "[deleteOldLoginAttempts] unexpected deadline")
	}
	prevDaySessionNumber := time.Now().Add(-24*stdlibtime.Hour).Unix() / int64(c.cfg.EmailValidation.SameIPRateCheckPeriod.Seconds())
	sql := `DELETE FROM sign_ins_per_ip WHERE login_session_number < $1`
	if _, err := storage.Exec(ctx, c.db, sql, prevDaySessionNumber); err != nil {
		return errors.Wrap(err, "failed to delete old data from sign_ins_per_ip")
	}

	return nil
}

func (c *client) startOldLoginAttemptsCleaner(ctx context.Context) {
	ticker := stdlibtime.NewTicker(stdlibtime.Duration(1+rand.Intn(24)) * stdlibtime.Minute) //nolint:gosec,gomnd // Not an  issue.
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			const deadline = 30 * stdlibtime.Second
			reqCtx, cancel := context.WithTimeout(ctx, deadline)
			log.Error(errors.Wrap(c.deleteOldLoginAttempts(reqCtx), "failed to deleteOldTrackedActions"))
			cancel()
		case <-ctx.Done():
			return
		}
	}
}
