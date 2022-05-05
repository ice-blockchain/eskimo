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
	mbProducer := messagebroker.MustConnect(ctx, applicationYamlKey)

	mbProcessors, finishers := processors(context.Background(), db, mbProducer)
	mbConsumer := messagebroker.MustConnectAndStartConsuming(context.Background(), cancel, applicationYamlKey, mbProcessors)

	return &processor{
		close:           closeAll(mbConsumer, mbProducer, finishers, db),
		ReadRepository:  &users{db: db},
		WriteRepository: &users{db: db, mb: mbProducer},
	}
}

func processors(ctx context.Context, db tarantool.Connector, mb messagebroker.Client) (map[messagebroker.Topic]messagebroker.Processor, []func()) {
	finishers := make([]func(), 0, 1+1)

	return map[messagebroker.Topic]messagebroker.Processor{
		cfg.MessageBroker.Topics[0].Name: &usersSource{db},
	}, finishers
}

func (p *processor) Close() error {
	return errors.Wrap(p.close(), "closing users processor failed")
}

func (r *repository) Close() error {
	return errors.Wrap(r.close(), "closing users repository failed")
}

//nolint:gocognit // More errors more complexity
func closeAll(mbConsumer, mbProducer messagebroker.Client, finishers []func(), db tarantool.Connector) func() error {
	return func() error {
		err1 := errors.Wrap(mbConsumer.Close(), "closing message broker consumer connection failed")
		for _, finish := range finishers {
			finish()
		}
		err2 := errors.Wrap(mbProducer.Close(), "closing message broker producer connection failed")
		err3 := errors.Wrap(db.Close(), "closing db connection failed")
		errs := make([]error, 0, 1+1+1)
		if err1 != nil {
			errs = append(errs, err1)
		}
		if err2 != nil {
			errs = append(errs, err2)
		}
		if err3 != nil {
			errs = append(errs, err3)
		}
		if len(errs) > 1 {
			return multierror.Append(nil, errs...)
		} else if len(errs) == 1 {
			return errors.Wrapf(errs[0], "failed to close all resources")
		}

		return nil
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

func (mb *usersSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		log.Panic(errors.Wrap(ctx.Err(), "unexpected deadline while processing message"))
	}

	return nil
}

func (p *processor) CheckHealth(_ context.Context) error {
	//nolint:nolintlint    // TODO implement me.
	return nil
}
