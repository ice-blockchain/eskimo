// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (s *userSnapshotSource) Process(ctx context.Context, msg *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}
	if len(msg.Value) == 0 {
		return nil
	}
	usr := new(UserSnapshot)
	if err := json.UnmarshalContext(ctx, msg.Value, usr); err != nil {
		return errors.Wrapf(err, "process: cannot unmarshall %v into %#v", string(msg.Value), usr)
	}

	return multierror.Append( //nolint:wrapcheck // Not needed.
		errors.Wrap(s.incrementTotalUsers(ctx, usr), "failed to incrementTotalUsers"),
		errors.Wrap(s.incrementOrDecrementCountryUserCount(ctx, usr), "failed to incrementOrDecrementCountryUserCount"),
		errors.Wrap(s.updateReferralCount(ctx, msg.Timestamp, usr), "failed to updateReferralCount"),
		errors.Wrap(s.deleteUserTracking(ctx, usr), "failed to deleteUserTracking"),
	).ErrorOrNil()
}

func (r *repository) sendTombstonedUserMessage(ctx context.Context, userID string) error {
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     userID,
		Topic:   r.cfg.MessageBroker.Topics[1].Name,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send tombstoned user message to broker")
}

func (r *repository) sendUserSnapshotMessage(ctx context.Context, user *UserSnapshot) error {
	valueBytes, err := json.MarshalContext(ctx, user)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal %#v", user)
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
		Topic:   r.cfg.MessageBroker.Topics[1].Name,
		Value:   valueBytes,
	}

	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send user snapshot message to broker")
}
