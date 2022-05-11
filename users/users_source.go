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

	switch {
	case u.User.Country == "" || u.User.Country == u.Before.Country:
		return nil
	case u.User.DeletedAt != nil:
		return errors.Wrap(mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Substract), "error decrementing user country count")
	case u.User.Country != u.Before.Country:
		if err := mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Add); err != nil {
			return errors.Wrap(err, "error incrementing country user count")
		}

		fallthrough
	case u.Before.Country != "":
		return errors.Wrap(mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Substract), "error decrementing user country count")
	}

	return nil
}
