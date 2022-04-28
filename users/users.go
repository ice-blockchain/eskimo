// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"

	"github.com/framey-io/go-tarantool"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
)

func New(ctx context.Context, cancel context.CancelFunc) Repository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)

	return &repository{
		close:          closeDB(db),
		ReadRepository: &users{db: db},
	}
}

func StartProcessor(ctx context.Context, cancel context.CancelFunc) Processor {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)
	mb := messagebroker.MustConnect(ctx, applicationYamlKey)

	return &processor{
		close:                           closeAll(db, mb),
		ReadRepository:                  &users{db: db},
		WriteRepository:                 &users{db: db, mb: mb},
		PhoneNumberValidationRepository: &phoneNumberValidationCodes{db: db},
	}
}

func (p *processor) Close() error {
	return errors.Wrap(p.close(), "closing users processor failed")
}

func (r *repository) Close() error {
	return errors.Wrap(r.close(), "closing users repository failed")
}

func closeAll(db tarantool.Connector, mb messagebroker.Client) func() error {
	return func() error {
		err1 := errors.Wrap(db.Close(), "closing db connection failed")
		err2 := errors.Wrap(mb.Close(), "closing message broker connection failed")
		if err1 != nil && err2 != nil {
			return multierror.Append(err1, err2)
		}
		var err error
		if err1 != nil {
			err = err1
		}
		if err2 != nil {
			err = err2
		}

		return errors.Wrapf(err, "failed to close all resources")
	}
}

func closeDB(db tarantool.Connector) func() error {
	return func() error {
		m := "closing db connection failed"
		log.Info(m)

		return errors.Wrap(db.Close(), m)
	}
}

func (u *users) sendUsersMessage(ctx context.Context, user *User) error {
	valueBytes, err := json.Marshal(user)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal user %#v", user)
	}

	m := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     user.ID,
		Topic:   cfg.MessageBroker.Topics[0].Name,
		Value:   valueBytes,
	}

	responder := make(chan error, 1)
	defer close(responder)
	u.mb.SendMessage(ctx, m, responder)

	return errors.Wrapf(<-responder, "failed to send users message to broker")
}

func (p *processor) CheckHealth(_ context.Context) error {
	//nolint:nolintlint    // TODO implement me.
	return nil
}
