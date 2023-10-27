// SPDX-License-Identifier: ice License 1.0

package social

import (
	social "github.com/ice-blockchain/eskimo/kyc/social/internal"
)

// Public API.

const (
	FacebookType = social.StrategyFacebook
	TwitterType  = social.StrategyTwitter
)

const (
	SuccessVerificationResult VerificationResult = "SUCCESS"
	FailureVerificationResult VerificationResult = "FAILURE"
)

type (
	Verifier interface {
		social.Verifier
	}
	Type               = social.StrategyType
	VerificationResult string
	Verification       struct {
		RemainingAttempts *uint8             `json:"remainingAttempts,omitempty" example:"3"`
		Result            VerificationResult `json:"result,omitempty" example:"false"`
		ExpectedPostText  string             `json:"expectedPostText,omitempty" example:"This is a verification post!"`
	}
)
