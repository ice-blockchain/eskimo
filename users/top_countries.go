// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (r *repository) GetTopCountries(ctx context.Context, keyword string, limit, offset uint64) (cs []*CountryStatistics, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get top countries failed because context failed")
	}
	countries, countryParams := r.getTopCountriesParams(keyword)
	params := []any{limit, offset}
	params = append(params, countryParams...)
	sql := fmt.Sprintf(`
						SELECT  country, 
								user_count 
						FROM users_per_country
						WHERE lower(country) in (%v)
						ORDER BY user_count desc 
						LIMIT $1 OFFSET $2`, countries)
	cs, err = storage.Select[CountryStatistics](ctx, r.db, sql, params...)
	if err != nil {
		return nil, errors.Wrapf(err, "get top countries failed for %v %v %v", keyword, limit, offset)
	}

	return
}

func (r *repository) getTopCountriesParams(countryKeyword string) (countriesSQLEnumeration string, params []any) {
	countriesSQLEnumeration = "''"
	params = make([]any, 0)
	const initialParamIdx = 3 // 1 and 2 are limit and offset.
	keyword := strings.ToLower(countryKeyword)
	if keyword == "" {
		countriesSQLEnumeration = "lower(country)"
	} else if countries := r.LookupCountries(keyword); len(countries) != 0 {
		var countryParams []string
		for i, country := range countries {
			countryParams = append(countryParams, fmt.Sprintf("$%v", i+initialParamIdx))
			params = append(params, strings.ToLower(country))
		}
		countriesSQLEnumeration = strings.Join(countryParams, ",")
	}

	return
}

func (r *repository) incrementOrDecrementCountryUserCount(ctx context.Context, usr *UserSnapshot) error { //nolint:gocognit // .
	if (usr.User != nil && usr.Before != nil && usr.User.Country == usr.Before.Country) || ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	sqlTemplate := `
INSERT INTO users_per_country (country, user_count) 
VALUES ($1, 1) ON CONFLICT (country) DO UPDATE
	SET user_count = GREATEST(users_per_country.user_count %v 1, 0)`

	return errors.Wrapf(storage.DoInTransaction(ctx, r.db, func(conn storage.QueryExecer) error {
		if usr.User != nil {
			if _, err := storage.Exec(ctx, r.db, fmt.Sprintf(sqlTemplate, "+"), usr.User.Country); err != nil {
				return errors.Wrapf(err, "error increasing country count for country:%v", usr.User.Country)
			}
		}
		if usr.Before != nil {
			if _, err := storage.Exec(ctx, r.db, fmt.Sprintf(sqlTemplate, "-"), usr.Before.Country); err != nil {
				return errors.Wrapf(err, "error decreasing country count for country:%v", usr.Before.Country)
			}
		}

		return nil
	}), "failed to execute transaction updating users_per_countries")
}
