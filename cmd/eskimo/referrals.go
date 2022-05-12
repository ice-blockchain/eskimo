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
// @Success      200            {array}   users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/referrals [GET].
func (s *service) GetReferrals(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetReferrals)

	//nolint:nolintlint,godox // TODO implement me
	if req.AuthenticatedUser.ID == req.ID { //nolint:nolintlint,gocritic,staticcheck
		// User is trying to get their own referrals.
	} else { //nolint:nolintlint,gocritic,staticcheck
		// User is trying to get some other user's referrals.
	}

	return server.OK([]*users.User{{
		// We implement only T1 ones for now.
		// The order of the referrals is : referrals from mobile phone agenda, then the most recent ones (based on createdAt).
		// Return only those fields:.
		ID:                "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2",
		Username:          "jdoe",
		PhoneNumber:       "+12099216581",
		ProfilePictureURL: "a.jpg",
		//nolint:nolintlint,godox // TODO we need to find out how to find out if someone is an agenda contact of another.
	}})
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
		req.Type = "T1"
	} else if strings.ToUpper(req.Type) != "T1" && strings.ToUpper(req.Type) != "T2" { //nolint:gocritic // later
		err := errors.Errorf("type '%v' is invalid, valid types are [T1,T2]", req.Type)

		return &server.Response{
			Data: server.ErrorResponse{
				Error: err.Error(),
				Code:  "INVALID_PROPERTIES",
			}.Fail(err),
			Code: http.StatusBadRequest,
		}
	}

	return server.RequiredStrings(map[string]string{"userId": req.ID})
}

func (req *RequestGetReferrals) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
