// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

const (
	SuccessResult Result = "SUCCESS"
	FailureResult Result = "FAILURE"
)

type (
	Result string
	Quiz   struct {
		Progress *Progress `json:"progress,omitempty"`
		Result   Result    `json:"result,omitempty"`
	}
	Progress struct {
		ExpiresAt        *time.Time `json:"expiresAt" example:"2022-01-03T16:20:52.156534Z"`
		NextQuestion     *Question  `json:"nextQuestion"`
		MaxQuestions     uint8      `json:"maxQuestions" example:"21"`
		CorrectAnswers   uint8      `json:"correctAnswers" example:"16"`
		IncorrectAnswers uint8      `json:"incorrectAnswers" example:"2"`
	}
	Question struct {
		Text    string   `json:"text" example:"Какая температура на улице?"`
		Options []string `json:"options" example:"+21,-2,+33,0"`
		Number  uint8    `json:"number" example:"1"`
	}
)
