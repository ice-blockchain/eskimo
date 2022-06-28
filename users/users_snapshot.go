// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (s *userSnapshotSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}

	var u UserSnapshot
	if err := json.Unmarshal(m.Value, &u); err != nil {
		return errors.Wrapf(err, "Process: cannot unmarshall %v into %#v", string(m.Value), u)
	}

	if u.User != nil {
		if err := s.incrementOrDecrementCountryUserCount(ctx, u.User.Country, add); err != nil {
			return errors.Wrapf(err, "error incrementing country user count for country %v", u.User.Country)
		}
	}

	if u.Before != nil {
		if err := s.incrementOrDecrementCountryUserCount(ctx, u.Before.Country, subtract); err != nil {
			return errors.Wrapf(err, "error incrementing country user count for country %v", u.Before.Country)
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

	m := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     key,
		Topic:   cfg.MessageBroker.Topics[0].Name,
		Value:   valueBytes,
	}

	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, m, responder)

	return errors.Wrapf(<-responder, "failed to send user snapshot message to broker")
}
