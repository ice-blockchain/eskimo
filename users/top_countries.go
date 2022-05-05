// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"
)

func (mb *usersSource) incrementCountryUserCount(ctx context.Context, countryNew string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "inc countries failed because context failed")
	}

	return errors.Wrapf(mb.db.UpdateTyped("USERS_PER_COUNTRY", "pk_unnamed_USERS_PER_COUNTRY_1",
		tarantool.StringKey{S: countryNew}, []tarantool.Op{{Op: "+", Field: 1, Arg: 1}}, &[]*user{}),
		"error updating USERS_PER_COUNTRY")
}

func (mb *usersSource) decrementCountryUserCount(ctx context.Context, countryOld string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "dec countries failed because context failed")
	}

	return errors.Wrapf(mb.db.UpdateTyped("USERS_PER_COUNTRY", "pk_unnamed_USERS_PER_COUNTRY_1",
		tarantool.StringKey{S: countryOld}, []tarantool.Op{{Op: "-", Field: 1, Arg: 1}}, &[]*user{}),
		"error updating USERS_PER_COUNTRY")
}

func (u *users) GetTopCountries(ctx context.Context, limit Limit, offset Offset) (cs []*CountryStatistics, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get top countries failed because context failed")
	}

	var result []*CountryStatistics

	sql := `SELECT country, user_count FROM users_per_country ORDER BY user_count desc LIMIT :limit OFFSET :offset`
	params := map[string]interface{}{"limit": limit, "offset": offset}

	if err = u.db.PrepareExecuteTyped(sql, params, &result); err != nil {
		return nil, errors.Wrap(err, "get top countries failed")
	}

	return result, nil
}
