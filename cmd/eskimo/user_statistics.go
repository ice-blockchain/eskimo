// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserStatisticsRoutes(router *server.Router) {
	router.
		Group("v1r").
		GET("user-statistics/top-countries", server.RootHandler(s.GetTopCountries)).
		GET("user-statistics/user-growth", server.RootHandler(s.GetUserGrowth))
}

// GetTopCountries godoc
//
//	@Schemes
//	@Description	Returns the paginated view of users per country.
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Param			Authorization		header		string	true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header		string	false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			keyword				query		string	false	"a keyword to look for in all country codes or names"
//	@Param			limit				query		uint64	false	"Limit of elements to return. Defaults to 10"
//	@Param			offset				query		uint64	false	"Number of elements to skip before collecting elements to return"
//	@Success		200					{array}		users.CountryStatistics
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/user-statistics/top-countries [GET].
func (s *service) GetTopCountries( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetTopCountriesArg, []*users.CountryStatistics],
) (*server.Response[[]*users.CountryStatistics], *server.Response[server.ErrorResponse]) {
	if req.Data.Limit == 0 {
		req.Data.Limit = 10
	}
	result, err := s.usersRepository.GetTopCountries(ctx, req.Data.Keyword, req.Data.Limit, req.Data.Offset)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to get top countries for: %#v", req.Data))
	}

	return server.OK(&result), nil
}

// GetUserGrowth godoc
//
//	@Schemes
//	@Description	Returns statistics about user growth.
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Param			Authorization		header		string	true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header		string	false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			days				query		uint64	false	"number of days in the past to look for. Defaults to 3. Max is 90."
//	@Success		200					{object}	users.UserGrowthStatistics
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/user-statistics/user-growth [GET].
func (s *service) GetUserGrowth( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetUserGrowthArg, users.UserGrowthStatistics],
) (*server.Response[users.UserGrowthStatistics], *server.Response[server.ErrorResponse]) {
	const defaultDays, maxDays = 3, 90
	if req.Data.Days == 0 {
		req.Data.Days = defaultDays
	}
	if req.Data.Days > maxDays {
		req.Data.Days = maxDays
	}
	result, err := s.usersRepository.GetUserGrowth(ctx, req.Data.Days)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to get user growth stats for: %#v", req.Data))
	}

	return server.OK(result), nil
}
