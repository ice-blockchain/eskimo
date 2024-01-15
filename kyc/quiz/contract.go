// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"

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

		SkipQuizSession(ctx context.Context, userID UserID) error

		ContinueQuizSession(ctx context.Context, userID UserID, question, answer uint8) (*Quiz, error)
	}

	UserRepository interface {
		GetUserByID(ctx context.Context, userID string) (*users.UserProfile, error)
		ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error
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
		Number  uint8    `json:"number" example:"1"`
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
	ErrUnknownQuestionNumber    = newError("unknown question number")
	ErrUnknownSession           = newError("unknown session and/or user")
)

const (
	applicationYamlKey = "kyc/quiz"
)

var (
	//go:embed DDL.sql
	ddl string

	errSessionExpired = newError("session expired")
)

type (
	quizError struct {
		Msg string
	}
	userProgress struct {
		StartedAt      *time.Time `db:"started_at"`
		Deadline       *time.Time `db:"deadline"`
		Lang           string     `db:"language"`
		Questions      []uint8    `db:"questions"`
		Answers        []uint8    `db:"answers"`
		CorrectAnswers []uint8    `db:"correct_answers"`
	}
	repositoryImpl struct {
		DB       *storage.DB
		Shutdown func() error
		Users    UserRepository
		config
	}
	config struct {
		MaxSessionDurationSeconds int `yaml:"maxSessionDurationSeconds"`
		MaxQuestionsPerSession    int `yaml:"maxQuestionsPerSession"`
		MaxWrongAnswersPerSession int `yaml:"maxWrongAnswersPerSession"`
		SessionCoolDownSeconds    int `yaml:"sessionCoolDownSeconds"`
	}
)
