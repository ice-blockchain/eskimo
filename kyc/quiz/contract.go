// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"
	"sync/atomic"
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

		SkipQuizSession(ctx context.Context, userID UserID) error

		CheckQuizStatus(ctx context.Context, userID UserID) (*QuizStatus, error)

		ContinueQuizSession(ctx context.Context, userID UserID, question, answer uint8) (*Quiz, error)
	}

	UserRepository interface {
		GetUserByID(ctx context.Context, userID string) (*users.UserProfile, error)
		ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error
	}

	QuizStatus struct { //nolint:revive // Nope cuz we want to be able to embed this
		KYCQuizAvailabilityEndedAt *time.Time   `json:"kycQuizAvailabilityEndedAt" db:"kyc_quiz_availability_ended_at"`
		KYCQuizResetAt             []*time.Time `json:"kycQuizResetAt,omitempty" db:"kyc_quiz_reset_at"`
		KYCQuizRemainingAttempts   uint8        `json:"kycQuizRemainingAttempts,omitempty" db:"kyc_quiz_remaining_attempts"`
		KYCQuizDisabled            bool         `json:"kycQuizDisabled" db:"kyc_quiz_disabled"`
		KYCQuizCompleted           bool         `json:"kycQuizCompleted" db:"kyc_quiz_completed"`
		HasUnfinishedSessions      bool         `json:"-"`
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
	ErrSessionFinished          = newError("session closed")
	ErrSessionFinishedWithError = newError("session closed with error")
	ErrUnknownQuestionNumber    = newError("unknown question number")
	ErrUnknownSession           = newError("unknown session and/or user")
	ErrNotAvailable             = newError("quiz kyc not available")
)

const (
	applicationYamlKey = "kyc/quiz"

	requestDeadline = 25 * stdlibtime.Second
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
		alertFrequency            *atomic.Pointer[stdlibtime.Duration]
		MaxResetCount             *uint8 `yaml:"maxResetCount"`
		Environment               string `yaml:"environment" mapstructure:"environment"`
		AlertSlackWebhook         string `yaml:"alert-slack-webhook" mapstructure:"alert-slack-webhook"` //nolint:tagliatelle // .
		GlobalStartDate           string `yaml:"globalStartDate" example:"YYYY-MM-DD"`                   //nolint:tagliatelle // .
		AvailabilityWindowSeconds int    `yaml:"availabilityWindowSeconds"`
		MaxSessionDurationSeconds int    `yaml:"maxSessionDurationSeconds"`
		MaxQuestionsPerSession    int    `yaml:"maxQuestionsPerSession"`
		MaxWrongAnswersPerSession int    `yaml:"maxWrongAnswersPerSession"`
		SessionCoolDownSeconds    int    `yaml:"sessionCoolDownSeconds"`
		EnableAlerts              bool   `yaml:"enable-alerts" mapstructure:"enable-alerts"` //nolint:tagliatelle // .
		MaxAttemptsAllowed        uint8  `yaml:"maxAttemptsAllowed"`
	}
)
