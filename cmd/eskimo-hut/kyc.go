// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	stdlibtime "time"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/kyc4"
	"github.com/ice-blockchain/wintr/server"
	"github.com/ice-blockchain/wintr/time"
)

func (s *service) setupKYCRoutes(router *server.Router) {
	router.
		Group("v1w").
		POST("kyc/startOrContinueKYCStep4Session", server.RootHandler(s.StartOrContinueKYCStep4Session))
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
//	@Param			language			query		string	true	"language of the user"
//	@Param			selectedOption		query		int		true	"index of the options array. Set it to 222 for the first call."
//	@Param			questionNumber		query		int		true	"previous question number. Set it to 222 for the first call."
//	@Success		200					{object}	kyc4.Quiz
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403					{object}	server.ErrorResponse	"not allowed due to various reasons"
//	@Failure		404					{object}	server.ErrorResponse	"user is not found"
//	@Failure		409					{object}	server.ErrorResponse	"if any conflicts occur or any prerequisites are not met"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/kyc/startOrContinueKYCStep4Session [POST].
func (s *service) StartOrContinueKYCStep4Session( //nolint:gocritic,funlen,revive // .
	_ context.Context,
	req *server.Request[StartOrContinueKYCStep4SessionRequestBody, kyc4.Quiz],
) (*server.Response[kyc4.Quiz], *server.Response[server.ErrorResponse]) {
	if req.Data.QuestionNumber != 222 { //nolint:gomnd // .
		switch rand.Intn(10) { //nolint:gosec,gomnd // .
		case 0:
			return server.OK(&kyc4.Quiz{Result: kyc4.FailureQuizResult}), nil
		case 1:
			return server.OK(&kyc4.Quiz{Result: kyc4.SuccessQuizResult}), nil
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
	return server.OK(&kyc4.Quiz{
		Progress: &kyc4.QuizProgress{
			ExpiresAt: time.New(stdlibtime.Now().Add(stdlibtime.Hour)),
			NextQuestion: &kyc4.Question{
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
