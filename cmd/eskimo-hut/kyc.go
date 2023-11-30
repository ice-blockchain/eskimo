// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	stdlibtime "time"

	"github.com/pkg/errors"

	kycquiz "github.com/ice-blockchain/eskimo/kyc/quiz"
	kycsocial "github.com/ice-blockchain/eskimo/kyc/social"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
	"github.com/ice-blockchain/wintr/time"
)

func (s *service) setupKYCRoutes(router *server.Router) {
	router.
		Group("v1w").
		POST("kyc/startOrContinueKYCStep4Session/users/:userId", server.RootHandler(s.StartOrContinueKYCStep4Session)).
		POST("kyc/verifySocialKYCStep/users/:userId", server.RootHandler(s.VerifySocialKYCStep)).
		POST("kyc/tryResetKYCSteps/users/:userId", server.RootHandler(s.TryResetKYCSteps))
}

// StartOrContinueKYCStep4Session godoc
//
//	@Schemes
//	@Description	Starts or continues the kyc 4 session (Quiz), if available and if not already finished successfully.
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//
//	@Param			Authorization		header		string	true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header		string	false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			userId				path		string	true	"ID of the user"
//	@Param			language			query		string	true	"language of the user"
//	@Param			selectedOption		query		int		true	"index of the options array. Set it to 222 for the first call."
//	@Param			questionNumber		query		int		true	"previous question number. Set it to 222 for the first call."
//	@Success		200					{object}	kycquiz.Quiz
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403					{object}	server.ErrorResponse	"not allowed due to various reasons"
//	@Failure		404					{object}	server.ErrorResponse	"user is not found"
//	@Failure		409					{object}	server.ErrorResponse	"if any conflicts occur or any prerequisites are not met"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/kyc/startOrContinueKYCStep4Session/users/{userId} [POST].
func (s *service) StartOrContinueKYCStep4Session( //nolint:gocritic,funlen,revive // .
	_ context.Context,
	req *server.Request[StartOrContinueKYCStep4SessionRequestBody, kycquiz.Quiz],
) (*server.Response[kycquiz.Quiz], *server.Response[server.ErrorResponse]) {
	//nolint:godox // .
	// TODO add validations for "selectedOption" && "questionNumber".
	// TODO if we don`t support a specific language, default to 'en'.
	// TODO return 404 USER_NOT_FOUND if user is not found.
	// TODO implement the proper logic for the use cases bellow.
	if req.Data.QuestionNumber != 222 { //nolint:gomnd // .
		switch rand.Intn(10) { //nolint:gosec,gomnd // .
		case 0:
			return server.OK(&kycquiz.Quiz{Result: kycquiz.FailureResult}), nil
		case 1:
			return server.OK(&kycquiz.Quiz{Result: kycquiz.SuccessResult}), nil
		case 2: //nolint:gomnd // .
			return nil, server.Conflict(errors.Errorf("question already answered, retry with fresh a call (222)"), questionAlreadyAnsweredErrorCode)
		}
	}
	switch rand.Intn(10) { //nolint:gosec,gomnd // .
	case 0:
		return nil, server.Conflict(errors.Errorf("quiz already finished successfully, ignore it and proceed with mining"), quizAlreadyCompletedSuccessfullyErrorCode)
	case 1:
		return nil, server.ForbiddenWithCode(errors.Errorf("quiz not available, ignore it and proceed with mining"), quizNotAvailableErrorCode)
	}

	//nolint:lll // .
	return server.OK(&kycquiz.Quiz{
		Progress: &kycquiz.Progress{
			ExpiresAt: time.New(stdlibtime.Now().Add(stdlibtime.Hour)),
			NextQuestion: &kycquiz.Question{
				Options: []string{
					fmt.Sprintf("[%v]You don't need to do anything and the ice is mined automatically", strings.Repeat("bogus", rand.Intn(20))),                                 //nolint:gosec,gomnd,lll // .
					fmt.Sprintf("[%v]You need to check in every 24 hours by tapping the Ice button to begin your daily mining session", strings.Repeat("bogus", rand.Intn(20))), //nolint:gosec,gomnd,lll // .
					fmt.Sprintf("[%v]Ice is not mined, but it turns out immediately after registration", strings.Repeat("bogus", rand.Intn(20))),                                //nolint:gosec,gomnd,lll // .
					fmt.Sprintf("[%v]Ice is cool", strings.Repeat("bogus", rand.Intn(20))),                                                                                      //nolint:gosec,gomnd // .
				},
				Number: uint8(11 + rand.Intn(20)),                                                                                                                  //nolint:gosec,gomnd // .
				Text:   fmt.Sprintf("[%v][%v] What are the major differences between Ice, Pi and Bee?", req.Data.Language, strings.Repeat("bogus", rand.Intn(20))), //nolint:gosec,gomnd,lll // .
			},
			MaxQuestions:     uint8(30 + rand.Intn(40)), //nolint:gosec,gomnd // .
			CorrectAnswers:   uint8(1 + rand.Intn(6)),   //nolint:gosec,gomnd // .
			IncorrectAnswers: uint8(1 + rand.Intn(3)),   //nolint:gosec,gomnd // .
		},
	}), nil
}

// VerifySocialKYCStep godoc
//
//	@Schemes
//	@Description	Verifies if the user has posted the expected verification post on their social media account.
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//
//	@Param			Authorization		header		string							true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header		string							false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			userId				path		string							true	"ID of the user"
//	@Param			language			query		string							true	"language of the user"
//	@Param			kycStep				query		int								true	"the value of the social kyc step to verify"	Enums(3,5)
//	@Param			social				query		string							true	"the desired social you wish to verify it with"	Enums(facebook,twitter)
//	@Param			request				body		VerifySocialKYCStepRequestBody	false	"Request params"
//	@Success		200					{object}	kycsocial.Verification
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403					{object}	server.ErrorResponse	"not allowed due to various reasons"
//	@Failure		404					{object}	server.ErrorResponse	"user is not found"
//	@Failure		409					{object}	server.ErrorResponse	"if any conflicts occur or any prerequisites are not met"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/kyc/verifySocialKYCStep/users/{userId} [POST].
func (s *service) VerifySocialKYCStep( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[VerifySocialKYCStepRequestBody, kycsocial.Verification],
) (*server.Response[kycsocial.Verification], *server.Response[server.ErrorResponse]) {
	if err := validateVerifySocialKYCStep(req); err != nil {
		return nil, server.UnprocessableEntity(errors.Wrapf(err, "validations failed for %#v", req.Data), invalidPropertiesErrorCode)
	}
	result, err := s.socialRepository.VerifyPost(ctx, &req.Data.VerificationMetadata)
	if err != nil {
		err = errors.Wrapf(err, "failed to verify post for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrRelationNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, kycsocial.ErrDuplicate):
			return nil, server.Conflict(err, socialKYCStepAlreadyCompletedSuccessfullyErrorCode)
		case errors.Is(err, kycsocial.ErrNotAvailable):
			return nil, server.ForbiddenWithCode(err, socialKYCStepNotAvailableErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(result), nil
}

func validateVerifySocialKYCStep(req *server.Request[VerifySocialKYCStepRequestBody, kycsocial.Verification]) error {
	if !slices.Contains(kycsocial.AllSupportedKYCSteps, req.Data.KYCStep) {
		return errors.Errorf("unsupported kycStep `%v`", req.Data.KYCStep)
	}
	if !slices.Contains(kycsocial.AllTypes, req.Data.Social) {
		return errors.Errorf("unsupported social `%v`", req.Data.Social)
	}
	switch req.Data.Social {
	case kycsocial.FacebookType:
		if req.Data.Facebook.AccessToken == "" {
			return errors.Errorf("unsupported facebook.accessToken `%v`", req.Data.Facebook.AccessToken)
		}
	case kycsocial.TwitterType:
		if req.Data.Twitter.TweetURL == "" {
			return errors.Errorf("unsupported twitter.tweetUrl `%v`", req.Data.Twitter.TweetURL)
		}
	}

	return nil
}

// TryResetKYCSteps godoc
//
//	@Schemes
//	@Description	Checks if there are any kyc steps that should be reset, if so, it resets them and returns the updated latest user state.
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//
//	@Param			Authorization		header		string	true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header		string	false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			userId				path		string	true	"ID of the user"
//	@Param			skipKYCSteps		query		int		false	"the kyc steps you wish to skip"
//	@Success		200					{object}	User
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403					{object}	server.ErrorResponse	"not allowed due to various reasons"
//	@Failure		404					{object}	server.ErrorResponse	"user is not found"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/kyc/tryResetKYCSteps/users/{userId} [POST].
func (s *service) TryResetKYCSteps( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[TryResetKYCStepsRequestBody, User],
) (*server.Response[User], *server.Response[server.ErrorResponse]) {
	if req.AuthenticatedUser.Role != "admin" && req.Data.UserID != req.AuthenticatedUser.UserID {
		return nil, server.Forbidden(errors.New("operation not allowed"))
	}
	ctx = users.ContextWithXAccountMetadata(ctx, req.Data.XAccountMetadata) //nolint:revive // .
	ctx = users.ContextWithAuthorization(ctx, req.Data.Authorization)       //nolint:revive // .
	for _, kycStep := range req.Data.SkipKYCSteps {
		switch kycStep { //nolint:exhaustive // .
		case users.Social1KYCStep, users.Social2KYCStep:
			if err := s.socialRepository.SkipVerification(ctx, kycStep, req.Data.UserID); err != nil {
				return nil, server.Unexpected(errors.Wrapf(err, "failed to skip kycStep %v", kycStep))
			}
		}
	}
	resp, err := s.usersProcessor.TryResetKYCSteps(ctx, req.Data.UserID)
	if err = errors.Wrapf(err, "failed to TryResetKYCSteps for userID:%v", req.Data.UserID); err != nil {
		switch {
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(&User{User: resp, Checksum: resp.Checksum()}), nil
}
