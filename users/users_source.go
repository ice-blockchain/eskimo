// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/log"
)

func processors(ctx context.Context, db tarantool.Connector, mb messagebroker.Client) map[messagebroker.Topic]messagebroker.Processor {
	return map[messagebroker.Topic]messagebroker.Processor{
		cfg.MessageBroker.Topics[0].Name: &usersSource{db},
	}
}

func (mb *usersSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		log.Panic(errors.Wrap(ctx.Err(), "unexpected deadline while processing message"))
	}

	return nil
}
