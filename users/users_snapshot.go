// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (s *userSnapshotSource) Process(ctx context.Context, msg *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}

	var usr UserSnapshot
	if err := json.Unmarshal(msg.Value, &usr); err != nil {
		return errors.Wrapf(err, "process: cannot unmarshall %v into %#v", string(msg.Value), usr)
	}

	if usr.User != nil {
		if err := s.incrementOrDecrementCountryUserCount(ctx, usr.User.Country, add); err != nil {
			return errors.Wrapf(err, "error incrementing country user count for country %v", usr.User.Country)
		}
	}

	if usr.Before != nil {
		if err := s.incrementOrDecrementCountryUserCount(ctx, usr.Before.Country, subtract); err != nil {
			return errors.Wrapf(err, "error incrementing country user count for country %v", usr.Before.Country)
		}
	}

	return nil
}

func (r *repository) sendUserSnapshotMessage(ctx context.Context, user *UserSnapshot) error {
	valueBytes, err := json.Marshal(user)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal user %#v", user)
	}

	var key string
	if user.User == nil {
		key = user.Before.ID
	} else {
		key = user.ID
	}

	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     key,
		Topic:   cfg.MessageBroker.Topics[0].Name,
		Value:   valueBytes,
	}

	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send user snapshot message to broker")
}
