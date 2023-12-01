// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"embed"
	"io"
	"mime/multipart"
	"text/template"
	stdlibtime "time"

	"github.com/pkg/errors"

	social "github.com/ice-blockchain/eskimo/kyc/social/internal"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
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

var (
	ErrNotAvailable = errors.New("social kyc not available")
	ErrDuplicate    = errors.New("social kyc already finished")
)

var (
	//nolint:gochecknoglobals // Its loaded once at startup.
	AllTypes = []Type{FacebookType, TwitterType}
	//nolint:gochecknoglobals // Its loaded once at startup.
	AllSupportedKYCSteps = []users.KYCStep{users.Social1KYCStep, users.Social2KYCStep}
)

type (
	Type               = social.StrategyType
	VerificationResult string
	Verification       struct {
		RemainingAttempts *uint8             `json:"remainingAttempts,omitempty" example:"3"`
		Result            VerificationResult `json:"result,omitempty" example:"false"`
		ExpectedPostText  string             `json:"expectedPostText,omitempty" example:"This is a verification post!"`
	}
	Twitter struct {
		TweetURL string `json:"tweetUrl" required:"true" example:"https://twitter.com/elonmusk/status/1716230049408434540"`
	}
	Facebook struct {
		AccessToken string `json:"accessToken" required:"true" example:"some token to access the 3rd party social API on behalf of the user"`
	}
	VerificationMetadata struct {
		Twitter  Twitter       `json:"twitter,omitempty"`
		Facebook Facebook      `json:"facebook,omitempty"`
		UserID   string        `uri:"userId" required:"true" swaggerignore:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Language string        `form:"language" required:"true" swaggerignore:"true" example:"en"`
		Social   Type          `form:"social" required:"true" swaggerignore:"true" example:"twitter"`
		KYCStep  users.KYCStep `form:"kycStep" required:"true" swaggerignore:"true" example:"1"`
	}
	Repository interface {
		io.Closer
		VerifyPost(ctx context.Context, metadata *VerificationMetadata) (*Verification, error)
		SkipVerification(ctx context.Context, kycStep users.KYCStep, userID string) error
	}
	UserRepository interface {
		GetUserByID(ctx context.Context, userID string) (*users.UserProfile, error)
		ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error
	}
)

// Private API.

const (
	applicationYamlKey = "kyc/social"
)

const (
	postContentLanguageTemplateType languageTemplateType = "post_content"
)

const (
	skippedReason          = "skipped"
	exhaustedRetriesReason = "exhausted_retries"
)

var (
	//go:embed DDL.sql
	ddl string
	//go:embed translations
	translations embed.FS
	//nolint:gochecknoglobals // Its loaded once at startup.
	allLanguageTemplateType = [1]languageTemplateType{postContentLanguageTemplateType}
	//nolint:gochecknoglobals // Its loaded once at startup.
	allTemplates = make(map[users.KYCStep]map[Type]map[languageTemplateType]map[languageCode]*languageTemplate, len(AllSupportedKYCSteps))
)

type (
	languageTemplateType string
	languageCode         = string
	languageTemplate     struct {
		content *template.Template
		Content string //nolint:revive // That's intended.
	}
	repository struct {
		user            UserRepository
		socialVerifiers map[Type]social.Verifier
		cfg             *config
		db              *storage.DB
	}
	config struct {
		DelayBetweenSessions stdlibtime.Duration `yaml:"delay-between-sessions" mapstructure:"delay-between-sessions"` //nolint:tagliatelle // .
		SessionWindow        stdlibtime.Duration `yaml:"session-window" mapstructure:"session-window"`                 //nolint:tagliatelle // .
		MaxSessionsAllowed   int                 `yaml:"max-sessions-allowed" mapstructure:"max-sessions-allowed"`     //nolint:tagliatelle // .
		MaxAttemptsAllowed   uint8               `yaml:"max-attempts-allowed" mapstructure:"max-attempts-allowed"`     //nolint:tagliatelle // .
	}
)
