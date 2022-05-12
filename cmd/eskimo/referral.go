// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserReferralRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		GET("users/:userId/referrals", server.RootHandler(newRequestGetReferrals, s.GetReferrals)).
		GET("users/:userId/referral-acquisition-history", server.RootHandler(newRequestGetReferralAcquisitionHistory, s.GetReferralAcquisitionHistory))
}

// GetReferralAcquisitionHistory godoc
// @Schemes
// @Description  Returns the history of referral acquisition for the provided user id.
// @Tags         Referrals
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true   "ID of the user"
// @Param        days           query     uint64  false  "The number of days to look in the past. Defaults to 5."
// @Success      200            {array}   users.ReferralAcquisition
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/referral-acquisition-history [GET].
func (s *service) GetReferralAcquisitionHistory(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetReferralAcquisitionHistory)

	//nolint:nolintlint,gocritic // TODO implement me.
	if req.AuthenticatedUser.ID == req.ID { //nolint:nolintlint,staticcheck
		// User is trying to get their own referral acquisition history.
	} else { //nolint:nolintlint,staticcheck
		// User is trying to get some other user's referral acquisition history.
	}

	return server.OK([]*users.ReferralAcquisition{{
		Date: time.Time{},
		T1:   12, //nolint:gomnd    // The number of users where referred_by = :user_id.
		T2:   11, //nolint:gomnd    // The number of users where referred_by in (t1).
	}})
}

func newRequestGetReferralAcquisitionHistory() server.ParsedRequest {
	return new(RequestGetReferralAcquisitionHistory)
}

func (req *RequestGetReferralAcquisitionHistory) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetReferralAcquisitionHistory) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetReferralAcquisitionHistory) Validate() *server.Response {
	return server.RequiredStrings(map[string]string{"userId": req.ID})
}

func (req *RequestGetReferralAcquisitionHistory) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, c.ShouldBindQuery, server.ShouldBindAuthenticatedUser(c)}
}

// GetReferrals godoc
// @Schemes
// @Description  Returns the referrals of an user.
// @Tags         Referrals
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true   "ID of the user"
// @Param        type           query     string  false  "Type of referrals: T1 or T2. Defaults to `T1`"
// @Param        limit          query     uint64  false  "Limit of elements to return"
// @Param        offset         query     uint64  false  "Number of elements to skip before collecting elements to return"
// @Success      200            {array}   users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/referrals [GET].
func (s *service) GetReferrals(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetReferrals)
	var referrals []*users.Referral
	var err error
	// We implement only T1 ones for now.
	// The order of the referrals is : referrals from mobile phone agenda, then the most recent ones (based on createdAt).
	// Referrals from mobile phone agenda will be implemented in the next PR, because it requires a lot of changes.
	if req.Type == tier1Referrals {
		referrals, err = s.usersRepository.GetTier1Referrals(ctx, req.ID, req.Limit, req.Offset)
		if err != nil {
			return server.Unexpected(err)
		}
	} else if req.Type == tier2Referrals {
		return server.Response{
			Data: server.ErrorResponse{
				Error: "Fetching of Tier 2 referrals is not implemented yet",
				Code:  "INVALID_PROPERTIES",
			}.Fail(err),
			Code: http.StatusBadRequest,
		}
	}

	return server.OK(referrals)
}

func newRequestGetReferrals() server.ParsedRequest {
	return new(RequestGetReferrals)
}

func (req *RequestGetReferrals) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetReferrals) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetReferrals) Validate() *server.Response {
	if req.Type == "" {
		req.Type = tier1Referrals
	} else if !strings.EqualFold(req.Type, tier1Referrals) && !strings.EqualFold(req.Type, tier2Referrals) {
		err := errors.Errorf("type '%v' is invalid, valid types are [%v,%v]", req.Type, tier2Referrals, tier2Referrals)

		return &server.Response{
			Data: server.ErrorResponse{
				Error: err.Error(),
				Code:  "INVALID_PROPERTIES",
			}.Fail(err),
			Code: http.StatusBadRequest,
		}
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	return server.RequiredStrings(map[string]string{"userId": req.ID})
}

func (req *RequestGetReferrals) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
