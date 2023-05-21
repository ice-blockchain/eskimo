// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

func (s *miningSessionSource) Process(ctx context.Context, msg *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}

	var ses miningSession
	if err := json.UnmarshalContext(ctx, msg.Value, &ses); err != nil {
		return errors.Wrapf(err, "process: cannot unmarshall %v into %#v", string(msg.Value), ses)
	}
	var usr *User
	if err := retry(ctx, func() error {
		var err error
		usr, err = s.getUserByID(ctx, ses.UserID)

		return err
	}); err != nil {
		return errors.Wrapf(err, "permanently failed to get current user by ID:%v", ses.UserID)
	}

	now := time.New(msg.Timestamp)
	if err := s.updateMiningSession(ctx, now, &ses); err != nil {
		return errors.Wrapf(err, "failed to updateMiningSession for %#v", ses)
	}

	if err := s.incrementTotalActiveUsers(ctx, usr.LastMiningStartedAt, now); err != nil {
		return errors.Wrapf(err, "failed to incrementTotalActiveUsers for prev:%v,next:%v", usr.LastMiningStartedAt, now)
	}

	return nil
}

func (s *miningSessionSource) updateMiningSession(ctx context.Context, now *time.Time, ses *miningSession) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline ")
	}
	sql := `UPDATE users
   			SET updated_at = $1,
   				last_mining_started_at = $2,
   				last_mining_ended_at = $3
	        WHERE id = $4
	          AND ((last_mining_started_at IS NULL OR last_mining_started_at != $2)
				   OR (last_mining_ended_at IS NULL OR last_mining_ended_at != $3))`
	_, err := storage.Exec(ctx, s.db, sql, time.Now().Time, now.Time, ses.EndedAt.Time, ses.UserID)

	return errors.Wrapf(err,
		"failed to update users.last_mining_started_at to %v, users.last_mining_ended_at to %v, for userID: %v", now, ses.EndedAt, ses.UserID)
}
