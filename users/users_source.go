// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (mb *usersSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}

	var u UserSnapshot
	if err := json.Unmarshal(m.Value, &u); err != nil {
		return errors.Wrapf(err, "Process: cannot unmarshall %v into %#v", string(m.Value), u)
	}

	if u.User != nil {
		if err := mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Add); err != nil {
			return errors.Wrap(err, "error incrementing country user count")
		}
	}

	if u.Before != nil {
		if err := mb.incrementOrDecrementCountryUserCount(ctx, u.Before.Country, Subtract); err != nil {
			return errors.Wrap(err, "error incrementing country user count")
		}
	}

	return nil
}
