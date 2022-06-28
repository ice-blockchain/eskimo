// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (s *miningStartedSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}
	var ms miningStarted
	if err := json.Unmarshal(m.Value, &ms); err != nil {
		return errors.Wrapf(err, "Process: cannot unmarshall %v into %#v", string(m.Value), ms)
	}
	userID := m.Key
	existing, err := s.mustGetUserByID(ctx, userID)
	if err != nil {
		return errors.Wrapf(err, "failed to get current user by id %v, before updating the users.last_mining_started_at", userID)
	}
	//nolint:gomnd // It's not a magic number, it's the index of that field.
	ops := []tarantool.Op{{Op: "=", Field: 2, Arg: ms.TS}}
	var result []*User
	if err = s.db.UpdateTyped("USERS", "pk_unnamed_USERS_1", tarantool.StringKey{S: userID}, ops, &result); err != nil {
		return errors.Wrapf(err, "could not update users.last_mining_started_at to %v for userID:%v ", ms.TS, userID)
	}

	return errors.Wrapf(s.sendUserSnapshotMessage(ctx, &UserSnapshot{User: result[0], Before: existing}), "failed to send user snapshot message")
}
