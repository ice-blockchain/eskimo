// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/go-tarantool-client"
)

func (r *repository) GetTopCountries(ctx context.Context, keyword string, limit, offset uint64) (cs []*CountryStatistics, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get top countries failed because context failed")
	}
	countries, params := r.getTopCountriesParams(keyword, offset)
	sql := fmt.Sprintf(`
						SELECT  country, 
								user_count 
						FROM users_per_country
						WHERE lower(country) in (%v)
						ORDER BY user_count desc 
						LIMIT %v OFFSET :offset`, countries, limit)
	err = errors.Wrapf(r.db.PrepareExecuteTyped(sql, params, &cs), "get top countries failed for %v %v %v", keyword, limit, offset)

	return
}

func (r *repository) getTopCountriesParams(countryKeyword string, offset uint64) (countriesSQLEnumeration string, params map[string]any) {
	countriesSQLEnumeration = "''"
	params = map[string]any{
		"offset": offset,
	}
	keyword := strings.ToLower(countryKeyword)
	if keyword == "" {
		countriesSQLEnumeration = "lower(country)"
	} else if countries := r.LookupCountries(keyword); len(countries) != 0 {
		var countryParams []string
		for i, country := range countries {
			k := fmt.Sprintf("country%v", i)
			countryParams = append(countryParams, fmt.Sprintf(":%v", k))
			params[k] = strings.ToLower(country)
		}
		countriesSQLEnumeration = strings.Join(countryParams, ",")
	}

	return
}

func (r *repository) incrementOrDecrementCountryUserCount(ctx context.Context, usr *UserSnapshot) error { //nolint:gocognit // .
	if (usr.User != nil && usr.Before != nil && usr.User.Country == usr.Before.Country) || ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	if usr.User != nil {
		arOp := []tarantool.Op{{Op: "+", Field: 1, Arg: uint64(1)}}
		tuple := &CountryStatistics{Country: usr.User.Country, UserCount: 1}

		if err := r.db.UpsertTyped("USERS_PER_COUNTRY", tuple, arOp, &[]*CountryStatistics{}); err != nil {
			return errors.Wrapf(err, "error increasing country count for country:%v", usr.User.Country)
		}
	}
	if usr.Before != nil {
		arOp := []tarantool.Op{{Op: "-", Field: 1, Arg: uint64(1)}}
		tuple := &CountryStatistics{Country: usr.Before.Country, UserCount: 1}

		if err := r.db.UpsertTyped("USERS_PER_COUNTRY", tuple, arOp, &[]*CountryStatistics{}); err != nil {
			var revertErr error
			if usr.User != nil {
				tuple = &CountryStatistics{Country: usr.User.Country, UserCount: 0}
				revertErr = errors.Wrapf(r.db.UpsertTyped("USERS_PER_COUNTRY", tuple, arOp, &[]*CountryStatistics{}),
					"failed to decrement USERS_PER_COUNTRY due to rollback for incrementing for %v", usr.User.Country)
			}

			return multierror.Append( //nolint:wrapcheck // Not needed.
				errors.Wrapf(err, "error decreasing country count for country:%v", usr.Before.Country),
				revertErr).ErrorOrNil()
		}
	}

	return nil
}
