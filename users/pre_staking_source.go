// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (s *preStakingSource) Process(ctx context.Context, msg *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}
	if len(msg.Value) == 0 {
		return nil
	}
	message := new(preStakingSnapshot)
	if err := json.UnmarshalContext(ctx, msg.Value, message); err != nil {
		return errors.Wrapf(err, "cannot unmarshal %v into %#v", string(msg.Value), message)
	}
	if message.UserID == "" {
		return nil
	}
	if message.Allocation == nil || message.Years == nil ||
		(message.Before != nil && message.Before.Allocation == message.Allocation && message.Before.Years == message.Years) {
		return nil
	}
	usr := new(User)
	usr.ID = message.UserID
	usr.Years = message.Years
	usr.Allocation = message.Allocation
	usr.Bonus = message.Bonus

	return errors.Wrapf(s.ModifyUser(ctx, usr, nil), "failed to modify user's prestaking for %#v", usr)
}
