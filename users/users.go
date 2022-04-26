// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"

	"github.com/framey-io/go-tarantool"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	appCfg "github.com/ICE-Blockchain/wintr/config"
	messagebroker "github.com/ICE-Blockchain/wintr/connectors/message_broker"
	"github.com/ICE-Blockchain/wintr/connectors/storage"
	"github.com/ICE-Blockchain/wintr/log"
)

func New(ctx context.Context, cancel context.CancelFunc) ReadRepository {
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

	u := &users{db: db, mb: mb}

	return &processor{
		close:           closeAll(db, mb),
		ReadRepository:  u,
		WriteRepository: u,
	}
}

func (u *users) Close() error {
	return nil
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
