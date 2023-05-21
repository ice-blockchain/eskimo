// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserReferralRoutes(router *server.Router) {
	router.
		Group("v1r").
		GET("users/:userId/referral-acquisition-history", server.RootHandler(s.GetReferralAcquisitionHistory)).
		GET("users/:userId/referrals", server.RootHandler(s.GetReferrals))
}

// GetReferralAcquisitionHistory godoc
//
//	@Schemes
//	@Description	Returns the history of referral acquisition for the provided user id.
//	@Tags			Referrals
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			userId			path		string	true	"ID of the user"
//	@Param			days			query		uint64	false	"Always is 5, cannot be changed due to DB schema"
//	@Success		200				{array}		users.ReferralAcquisition
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403				{object}	server.ErrorResponse	"if not allowed"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId}/referral-acquisition-history [GET].
func (s *service) GetReferralAcquisitionHistory( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetReferralAcquisitionHistoryArg, []*users.ReferralAcquisition],
) (*server.Response[[]*users.ReferralAcquisition], *server.Response[server.ErrorResponse]) {
	res, err := s.usersRepository.GetReferralAcquisitionHistory(ctx, req.Data.UserID)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "error getting referral acquisition history for %#v", req.Data))
	}

	return server.OK(&res), nil
}

// GetReferrals godoc
//
//	@Schemes
//	@Description	Returns the referrals of an user.
//	@Tags			Referrals
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			userId			path		string	true	"ID of the user"
//	@Param			type			query		string	true	"Type of referrals: `CONTACTS` or `T1` or `T2`"
//	@Param			limit			query		uint64	false	"Limit of elements to return. Defaults to 10"
//	@Param			offset			query		uint64	false	"Number of elements to skip before collecting elements to return"
//	@Success		200				{object}	users.Referrals
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403				{object}	server.ErrorResponse	"if not allowed"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId}/referrals [GET].
func (s *service) GetReferrals( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetReferralsArg, users.Referrals],
) (*server.Response[users.Referrals], *server.Response[server.ErrorResponse]) {
	if req.Data.Limit == 0 {
		req.Data.Limit = 10
	}
	var validType bool
	for _, referralType := range users.ReferralTypes {
		if strings.EqualFold(req.Data.Type, string(referralType)) {
			validType = true

			break
		}
	}
	if !validType {
		err := errors.Errorf("type '%v' is invalid, valid types are %v", req.Data.Type, users.ReferralTypes)

		return nil, server.UnprocessableEntity(err, invalidPropertiesErrorCode)
	}

	referrals, err := s.usersRepository.GetReferrals(ctx, req.Data.UserID, users.ReferralType(strings.ToUpper(req.Data.Type)), req.Data.Limit, req.Data.Offset)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to get referrals for %#v", req.Data))
	}

	return server.OK(referrals), nil
}
