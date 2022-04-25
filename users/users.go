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

func StartRepository(ctx context.Context, cancel context.CancelFunc) Repository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)
	mb := messagebroker.MustConnect(ctx, applicationYamlKey)

	return &repository{
		close:          closeAll(db, mb),
		UserRepository: &users{mb: mb, db: db},
	}
}

func StartProcessor(ctx context.Context, cancel context.CancelFunc) Processor {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)
	mb := messagebroker.MustConnect(ctx, applicationYamlKey)

	return &processor{
		close:          closeAll(db, mb),
		UserRepository: &users{db: db, mb: mb},
	}
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

func (r *repository) Close() error {
	log.Info("closing users repository...")

	return errors.Wrap(r.close(), "closing users repository failed")
}

func (u *users) sendUsersMessage(ctx context.Context, user *User) {
	valueBytes, err := json.Marshal(user)
	if err != nil {
		log.Error(errors.Wrapf(err, "failed to marshal user %v", user))

		return
	}

	//nolint:govet // Because we don`t need to cancel it cuz its a fire and forget action.
	pCtx, _ := context.WithTimeout(context.Background(), messageBrokerProduceRecordDeadline)

	var responder chan<- error
	if ctx.Value(messageBrokerProduceMessageResponseChanKey{}) != nil {
		responder = ctx.Value(messageBrokerProduceMessageResponseChanKey{}).(chan error)
	}

	u.mb.SendMessage(pCtx, &messagebroker.Message{
		Key:     user.ID,
		Value:   valueBytes,
		Headers: map[string]string{"producer": "eskimo"},
		Topic:   cfg.MessageBroker.Topics[0].Name,
	}, responder)
}

func (p processor) Close() error {
	//nolint:nolintlint    // TODO implement me.
	return nil
}

func (p processor) CheckHealth(ctx context.Context) error {
	//nolint:nolintlint    // TODO implement me.
	return nil
}
