// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
)

func (mb *usersSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline while processing message")
	}

	var u UserSnapshot
	if err := json.Unmarshal(m.Value, &u); err != nil {
		return errors.Wrap(err, "error unmarshalling msg broker data")
	}

	switch {
	case u.User.Country == "" || u.User.Country == u.Before.Country:
		return nil
	case u.Before.Country != "":
		err := mb.incrementOrDecrementCountryUserCount(ctx, u.Before.Country, Substract)
		if err != nil {
			return errors.Wrap(err, "user modify: counter error")
		}

		return errors.Wrap(mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Add), "user modify: counter error")
	case u.User.DeletedAt != nil:
		return errors.Wrap(mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Substract), "user delete: counter error")
	default:
		return errors.Wrap(mb.incrementOrDecrementCountryUserCount(ctx, u.User.Country, Add), "user add: counter error ")
	}
}

func (mb *usersSource) incrementOrDecrementCountryUserCount(ctx context.Context, country string, operation arithmeticOperation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}

	var res []*usersPerCountry
	key := tarantool.StringKey{S: country}
	arOp := []tarantool.Op{{Op: string(operation), Field: 1, Arg: 1}}

	err := mb.db.UpdateTyped("USERS_PER_COUNTRY", "pk_unnamed_USERS_PER_COUNTRY_1", key, arOp, &res)
	if err != nil {
		return errors.Wrap(err, "error changing country count")
	}

	return nil
}
