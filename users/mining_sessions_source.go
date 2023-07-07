// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

func (s *miningSessionSource) Process(ctx context.Context, msg *messagebroker.Message) error {
	if ctx.Err() != nil || len(msg.Value) == 0 {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}
	ses := new(miningSession)
	if err := json.UnmarshalContext(ctx, msg.Value, ses); err != nil || ses.UserID == "" || ses.StartedAt.IsNil() {
		return errors.Wrapf(err, "process: cannot unmarshall %v into %#v", string(msg.Value), ses)
	}
	if err := s.updateMiningSession(ctx, ses); err != nil {
		return errors.Wrapf(err, "failed to updateMiningSession for %#v", ses)
	}

	if err := s.incrementTotalActiveUsers(ctx, ses); err != nil {
		return errors.Wrapf(err, "failed to incrementTotalActiveUsers for %#v", ses)
	}

	return nil
}

func (s *miningSessionSource) updateMiningSession(ctx context.Context, ses *miningSession) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline ")
	}
	sql := fmt.Sprintf(`
		UPDATE users
		SET updated_at = $1,
			last_mining_started_at = $2,
			last_mining_ended_at = $3
		WHERE id = $4
		  AND (last_mining_started_at IS NULL OR (extract(epoch from last_mining_started_at)::bigint/%[1]v) != (extract(epoch from $2::timestamp)::bigint/%[1]v))
		  AND (last_mining_ended_at IS NULL OR (extract(epoch from last_mining_ended_at)::bigint/%[1]v) != (extract(epoch from $3::timestamp)::bigint/%[1]v))
	          `,
		uint64(s.cfg.GlobalAggregationInterval.MinMiningSessionDuration/stdlibtime.Second))
	affectedRows, err := storage.Exec(ctx, s.db, sql,
		time.Now().Time,
		ses.LastNaturalMiningStartedAt.Time,
		ses.EndedAt.Time,
		ses.UserID,
	)
	if affectedRows == 0 && err == nil {
		err = ErrDuplicate
	}

	return errors.Wrapf(err,
		"failed to update users.last_mining_started_at to %v, users.last_mining_ended_at to %v, for userID: %v", ses.LastNaturalMiningStartedAt.Time, ses.EndedAt, ses.UserID) //nolint:lll // .
}
