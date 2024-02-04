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
	if message.Before != nil && message.Before.Allocation == message.Allocation && message.Before.Years == message.Years {
		return nil
	}

	return errors.Wrapf(s.updatePreStaking(ctx, &message.PreStakingSummary), "can't update pre staking for userID:%v", message.UserID)
}

func (s *preStakingSource) updatePreStaking(ctx context.Context, ps *PreStakingSummary) error {
	sql := `
		UPDATE users
			SET updated_at = $1,
				pre_staking_years = $2,
				pre_staking_allocation = $3,
				pre_staking_bonus = $4	
		WHERE id = $5
			  AND (pre_staking_years != $2
			  OR pre_staking_allocation != $3
			  OR pre_staking_bonus != $4)`

	affectedRows, err := storage.Exec(ctx, s.db, sql,
		time.Now().Time,
		ps.Years,
		ps.Allocation,
		ps.Bonus,
		ps.UserID,
	)
	if affectedRows == 0 {
		err = ErrDuplicate
	}

	return errors.Wrapf(err,
		"failed to update users.years to %v, users.allocation to %v, users.bonus to %v, for userID: %v", ps.Years, ps.Allocation, ps.Bonus, ps.UserID)
}
