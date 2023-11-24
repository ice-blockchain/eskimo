// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	_ "embed"
	"io"
	stdlibtime "time"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

const (
	SuccessResult Result = "SUCCESS"
	FailureResult Result = "FAILURE"
)

type (
	UserID      = users.UserID
	UserProfile = users.UserProfile

	Repository interface {
		io.Closer

		StartQuizSession(ctx context.Context, userID UserID, lang string) (*Quiz, error)

		ContinueQuizSession(ctx context.Context, userID UserID, question uint, answer uint8) (*Quiz, error)
	}

	UserReader interface {
		GetUserByID(ctx context.Context, userID UserID) (*UserProfile, error)
	}

	Result string

	Quiz struct {
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
		Text    string   `json:"text" example:"Какая температура на улице?" db:"question"`
		Options []string `json:"options" example:"+21,-2,+33,0" db:"options"`
		Number  uint     `json:"number" example:"1"`
		ID      uint     `json:"-" db:"id"`
	}
)

var (
	ErrUnknownLanguage          = newError("unknown language")
	ErrInvalidKYCState          = newError("invalid KYC state")
	ErrUnknownUser              = newError("unknown user")
	ErrSessionIsAlreadyRunning  = newError("another session is already running")
	ErrSessionFinished          = newError("session closed")
	ErrSessionFinishedWithError = newError("session closed with error")
	ErrSessionExpired           = newError("session expired")
	ErrUnknownQuestionNumber    = newError("unknown question number")
	ErrUnknownSession           = newError("unknown session and/or user")
)

const (
	applicationYamlKey = "kyc/quiz"
)

var ( //nolint:gofumpt //.
	//go:embed DDL.sql
	ddl string
)

type (
	quizError struct {
		Msg string
	}
	userProgress struct {
		StartedAt      stdlibtime.Time `db:"started_at"`
		Lang           string          `db:"language"`
		Questions      []uint          `db:"questions"`
		Answers        []uint          `db:"answers"`
		CorrectAnswers []uint          `db:"correct_answers"`
	}
	repositoryImpl struct {
		DB       *storage.DB
		Shutdown func() error
		Users    UserReader
		config
	}
	config struct {
		MaxSessionDurationSeconds int `yaml:"maxSessionDurationSeconds"`
		MaxQuestionsPerSession    int `yaml:"maxQuestionsPerSession"`
		MaxWrongAnswersPerSession int `yaml:"maxWrongAnswersPerSession"`
		SessionCoolDownSeconds    int `yaml:"sessionCoolDownSeconds"`
	}
)
