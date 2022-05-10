// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/log"
)

func (mb *usersSource) Process(ctx context.Context, m *messagebroker.Message) error {
	if ctx.Err() != nil {
		log.Panic(errors.Wrap(ctx.Err(), "unexpected deadline while processing message"))
	}

	var u User
	if err := json.Unmarshal(m.Value, &u); err != nil {
		return errors.Wrap(err, "error unmarshalling msg broker data")
	}

	switch {
	case u.Country == "" || u.Country == m.Headers["countryBefore"]:
		return nil
	case m.Headers["countryBefore"] != "":
		mb.changeCountryUserCount(ctx, m.Headers["countryBefore"], Substract)
		mb.changeCountryUserCount(ctx, u.Country, Add)
	case u.DeletedAt != nil:
		mb.changeCountryUserCount(ctx, u.Country, Substract)
	default:
		mb.changeCountryUserCount(ctx, u.Country, Add)
	}

	return nil
}

func (mb *usersSource) changeCountryUserCount(ctx context.Context, country string, operation arithmeticOperation) {
	if ctx.Err() != nil {
		log.Panic(errors.Wrap(ctx.Err(), "context failed"))
	}

	var res []*usersPerCountry
	key := tarantool.StringKey{S: country}
	arOp := []tarantool.Op{{Op: string(operation), Field: 1, Arg: 1}}

	err := mb.db.UpdateTyped("USERS_PER_COUNTRY", "pk_unnamed_USERS_PER_COUNTRY_1", key, arOp, &res)
	if err != nil {
		log.Error(err, errors.Wrap(err, "error changing country count"))
	}
}
