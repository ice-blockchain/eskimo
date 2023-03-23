// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/time"
)

func (s *userPingSource) Process(ctx context.Context, msg *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}
	if len(msg.Value) == 0 {
		return nil
	}
	type (
		userPing struct {
			LastPingCooldownEndedAt *time.Time `json:"lastPingCooldownEndedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
			UserID                  string     `json:"userId,omitempty" example:"edfd8c02-75e0-4687-9ac2-1ce4723865c4"`
		}
	)
	message := new(userPing)
	if err := json.UnmarshalContext(ctx, msg.Value, message); err != nil {
		return errors.Wrapf(err, "cannot unmarshal %v into %#v", string(msg.Value), message)
	}
	if message.UserID == "" {
		return nil
	}
	usr := new(User)
	usr.ID = message.UserID
	usr.LastPingCooldownEndedAt = message.LastPingCooldownEndedAt

	return errors.Wrapf(s.ModifyUser(ctx, usr, nil), "failed to modify user's LastPingCooldownEndedAt for %#v", usr)
}
