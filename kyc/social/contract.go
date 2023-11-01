// SPDX-License-Identifier: ice License 1.0

package social

// Public API.

const (
	FacebookType Type = "facebook"
	TwitterType  Type = "twitter"
)

const (
	SuccessVerificationResult VerificationResult = "SUCCESS"
	FailureVerificationResult VerificationResult = "FAILURE"
)

type (
	VerificationResult string
	Type               string
	Verification       struct {
		RemainingAttempts *uint8             `json:"remainingAttempts,omitempty" example:"3"`
		Result            VerificationResult `json:"result,omitempty" example:"false"`
		ExpectedPostText  string             `json:"expectedPostText,omitempty" example:"This is a verification post!"`
	}
)
