// SPDX-License-Identifier: ice License 1.0

package api

import (
	"context"
	"io"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

type (
	Client interface {
		io.Closer
		GetQuizStatus(ctx context.Context, userIDs []string) (map[string]*QuizStatus, error)
		CheckHealth(ctx context.Context) error
	}

	QuizStatus struct {
		KYCQuizAvailabilityStartedAt *time.Time   `json:"kycQuizAvailabilityStartedAt" db:"kyc_quiz_availability_started_at"`
		KYCQuizAvailabilityEndedAt   *time.Time   `json:"kycQuizAvailabilityEndedAt" db:"kyc_quiz_availability_ended_at"`
		KYCQuizResetAt               []*time.Time `json:"kycQuizResetAt,omitempty" db:"kyc_quiz_reset_at"`
		KYCQuizRemainingAttempts     uint8        `json:"kycQuizRemainingAttempts,omitempty" db:"kyc_quiz_remaining_attempts"`
		KYCQuizAvailable             bool         `json:"kycQuizAvailable" db:"kyc_quiz_available"`
		KYCQuizDisabled              bool         `json:"kycQuizDisabled" db:"kyc_quiz_disabled"`
		KYCQuizCompleted             bool         `json:"kycQuizCompleted" db:"kyc_quiz_completed"`
		HasUnfinishedSessions        bool         `json:"-"`
	}
)

const (
	applicationYamlKey = "kyc/quiz"
)

type (
	config struct {
		globalStartDate           *time.Time
		MaxResetCount             *uint8 `yaml:"maxResetCount"`
		GlobalStartDate           string `yaml:"globalStartDate" example:"2022-01-03T16:20:52.156534Z"` //nolint:revive // .
		AvailabilityWindowSeconds int    `yaml:"availabilityWindowSeconds"`
		MaxAttemptsAllowed        uint8  `yaml:"maxAttemptsAllowed"`
	}
	quizStatus struct {
		*QuizStatus
		UserID string `db:"id"`
	}

	client struct {
		cfg *config
		db  *storage.DB
	}
)
