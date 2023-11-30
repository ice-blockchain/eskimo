// SPDX-License-Identifier: ice License 1.0

package social

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	social "github.com/ice-blockchain/eskimo/kyc/social/internal"
	"github.com/ice-blockchain/eskimo/users"
	appcfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:gochecknoinits // We load embedded stuff at runtime.
func init() {
	loadTranslations()
}

func loadTranslations() { //nolint:gocognit,revive // .
	for _, kycStep := range AllSupportedKYCSteps {
		for _, socialType := range AllTypes {
			for _, templateType := range allLanguageTemplateType {
				files, err := translations.ReadDir(fmt.Sprintf("translations/%v/%v/%v", kycStep, socialType, templateType))
				log.Panic(err) //nolint:revive // Nope.
				for _, file := range files {
					content, fErr := translations.ReadFile(fmt.Sprintf("translations/%v/%v/%v/%v", kycStep, socialType, templateType, file.Name()))
					log.Panic(fErr) //nolint:revive // Nope.
					language := strings.Split(file.Name(), ".")[0]
					templName := fmt.Sprintf("translations_%v_%v_%v_%v", kycStep, socialType, templateType, language)
					tmpl := languageTemplate{Content: string(content)}
					tmpl.content = template.Must(template.New(templName).Parse(tmpl.Content))
					if _, found := allTemplates[kycStep]; !found {
						allTemplates[kycStep] = make(map[Type]map[languageTemplateType]map[languageCode]*languageTemplate, len(AllTypes))
					}
					if _, found := allTemplates[kycStep][socialType]; !found {
						allTemplates[kycStep][socialType] = make(map[languageTemplateType]map[languageCode]*languageTemplate, len(&allLanguageTemplateType))
					}
					if _, found := allTemplates[kycStep][socialType][templateType]; !found {
						allTemplates[kycStep][socialType][templateType] = make(map[languageCode]*languageTemplate, len(files))
					}
					allTemplates[kycStep][socialType][templateType][language] = &tmpl
				}
			}
		}
	}
}

func New(ctx context.Context, usrRepo UserRepository) Repository {
	var cfg config
	appcfg.MustLoadFromKey(applicationYamlKey, &cfg)

	socialVerifiers := make(map[Type]social.Verifier, len(AllTypes))
	for _, tp := range AllTypes {
		socialVerifiers[tp] = social.New(tp)
	}

	return &repository{
		user:            usrRepo,
		socialVerifiers: socialVerifiers,
		cfg:             &cfg,
		db:              storage.MustConnect(ctx, ddl, applicationYamlKey),
	}
}

func (r *repository) Close() error {
	return errors.Wrap(r.db.Close(), "closing kyc/social repository failed")
}

func (r *repository) SkipVerification(ctx context.Context, kycStep users.KYCStep, userID string) error {
	now := time.Now()
	user, err := r.user.GetUserByID(ctx, userID)
	if err != nil {
		return errors.Wrapf(err, "failed to GetUserByID: %v", userID)
	}
	if user.KYCStepPassed == nil ||
		*user.KYCStepPassed < kycStep-1 ||
		*user.KYCStepPassed >= kycStep {
		return nil
	}

	return errors.Wrapf(r.modifyUser(ctx, false, kycStep, now, user.User), "[skip][%v]failed to modifyUser", kycStep)
}

//nolint:funlen,gocognit,gocyclo,revive,cyclop // .
func (r *repository) VerifyPost(ctx context.Context, metadata *VerificationMetadata) (*Verification, error) {
	now := time.Now()
	user, err := r.user.GetUserByID(ctx, metadata.UserID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GetUserByID: %v", metadata.UserID)
	}
	if user.KYCStepPassed == nil || *user.KYCStepPassed < metadata.KYCStep-1 { //nolint:gocritic // Nope.
		return nil, ErrNotAvailable
	} else if *user.KYCStepPassed >= metadata.KYCStep {
		return nil, ErrDuplicate
	} else if now.Sub(*(*user.KYCStepsLastUpdatedAt)[metadata.KYCStep-1].Time) < retryWindow {
		return nil, ErrNotAvailable
	}
	sql := `SELECT ARRAY_AGG(x.created_at) AS unsuccessful_attempts 
			FROM (SELECT created_at 
				  FROM social_kyc_unsuccessful_attempts 
				  WHERE user_id = $1
				    AND kyc_step = $2
				  ORDER BY created_at DESC) x`
	res, err := storage.Get[struct {
		UnsuccessfulAttempts *[]time.Time `db:"unsuccessful_attempts"`
	}](ctx, r.db, sql, metadata.UserID, metadata.KYCStep)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get unsuccessful_attempts for kycStep:%v,userID:%v", metadata.KYCStep, metadata.UserID)
	}
	remainingAttempts := maxAttempts
	if res.UnsuccessfulAttempts != nil {
		for _, unsuccessfulAttempt := range *res.UnsuccessfulAttempts {
			if unsuccessfulAttempt.After(now.Add(-retryWindow)) {
				remainingAttempts--
			}
		}
	}
	if remainingAttempts < 1 {
		return nil, ErrNotAvailable
	}
	if metadata.Twitter.TweetURL == "" && metadata.Facebook.AccessToken == "" {
		return &Verification{ExpectedPostText: metadata.expectedPostText(user.User)}, nil
	}
	pvm := &social.Metadata{
		AccessToken:      metadata.Facebook.AccessToken,
		PostURL:          metadata.Twitter.TweetURL,
		ExpectedPostText: metadata.expectedPostText(user.User),
	}
	userHandle, err := r.socialVerifiers[metadata.Social].VerifyPost(ctx, pvm)
	if err != nil {
		log.Error(errors.Wrapf(err, "social verification failed for KYCStep:%v,Social:%v,Language:%v,userID:%v",
			metadata.KYCStep, metadata.Social, metadata.Language, metadata.UserID))
		reason := detectReason(err)
		if err = r.saveUnsuccessfulAttempt(ctx, now, reason, metadata); err != nil {
			return nil, errors.Wrapf(err, "[1]failed to saveUnsuccessfulAttempt reason:%v,metadata:%#v", reason, metadata)
		}
		remainingAttempts--
		if remainingAttempts == 0 {
			if err = r.modifyUser(ctx, false, metadata.KYCStep, now, user.User); err != nil {
				return nil, errors.Wrapf(err, "[1failure][%v]failed to modifyUser", metadata.KYCStep)
			}
		}

		return &Verification{RemainingAttempts: &remainingAttempts, Result: FailureVerificationResult}, nil
	}
	if userHandle != "" { //nolint:nestif // .
		if err = r.saveSocial(ctx, metadata.Social, metadata.UserID, userHandle); err != nil {
			if storage.IsErr(err, storage.ErrDuplicate) {
				log.Error(errors.Wrapf(err, "[duplicate]social verification failed for KYCStep:%v,Social:%v,Language:%v,userID:%v,userHandle:%v",
					metadata.KYCStep, metadata.Social, metadata.Language, metadata.UserID, userHandle))
				reason := detectReason(terror.New(err, map[string]any{"user_handle": userHandle}))
				if err = r.saveUnsuccessfulAttempt(ctx, now, reason, metadata); err != nil {
					return nil, errors.Wrapf(err, "[2]failed to saveUnsuccessfulAttempt reason:%v,metadata:%#v", reason, metadata)
				}
				remainingAttempts--
				if remainingAttempts == 0 {
					if err = r.modifyUser(ctx, false, metadata.KYCStep, now, user.User); err != nil {
						return nil, errors.Wrapf(err, "[2failure][%v]failed to modifyUser", metadata.KYCStep)
					}
				}

				return &Verification{RemainingAttempts: &remainingAttempts, Result: FailureVerificationResult}, nil
			}

			return nil, errors.Wrapf(err, "failed to saveSocial social:%v, userID:%v, userHandle:%v", metadata.Social, metadata.UserID, userHandle)
		}
	} else {
		userHandle = metadata.UserID
	}
	if err = r.saveSocialKYCStep(ctx, now, userHandle, metadata); err != nil {
		return nil, errors.Wrapf(err, "failed to saveSocialKYCStep, userHandle:%v, metadata:%#v", userHandle, metadata)
	}
	if err = r.modifyUser(ctx, true, metadata.KYCStep, now, user.User); err != nil {
		return nil, errors.Wrapf(err, "[success][%v]failed to modifyUser", metadata.KYCStep)
	}

	return &Verification{Result: SuccessVerificationResult}, nil
}

//nolint:revive // Nope.
func (r *repository) modifyUser(ctx context.Context, success bool, kycStep users.KYCStep, now *time.Time, user *users.User) error {
	usr := new(users.User)
	usr.ID = user.ID
	if success {
		usr.KYCStepPassed = &kycStep
	}
	usr.KYCStepsLastUpdatedAt = user.KYCStepsLastUpdatedAt
	if len(*usr.KYCStepsLastUpdatedAt) < int(kycStep) {
		*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
	} else {
		(*usr.KYCStepsLastUpdatedAt)[int(kycStep)-1] = now
	}

	return errors.Wrapf(r.user.ModifyUser(ctx, usr, nil), "failed to modify user %#v", usr)
}

func (r *repository) saveSocialKYCStep(ctx context.Context, now *time.Time, userHandle string, metadata *VerificationMetadata) error {
	sql := `insert into social_kyc_steps(created_at,kyc_step,user_id,social,user_handle) VALUES ($1,$2,$3,$4,$5)`
	_, err := storage.Exec(ctx, r.db, sql, now.Time, metadata.KYCStep, metadata.UserID, metadata.Social, userHandle)

	return errors.Wrapf(err, "failed to `%v`;KYCStep:%v, userID:%v, social:%v, userHandle:%v", sql, metadata.KYCStep, metadata.UserID, metadata.Social, userHandle)
}

func (r *repository) saveUnsuccessfulAttempt(ctx context.Context, now *time.Time, reason string, metadata *VerificationMetadata) error {
	sql := `INSERT INTO social_kyc_unsuccessful_attempts(created_at, kyc_step, reason, user_id, social) VALUES ($1,$2,$3,$4,$5)`
	_, err := storage.Exec(ctx, r.db, sql, now.Time, metadata.KYCStep, reason, metadata.UserID, metadata.Social)

	return errors.Wrapf(err, "failed to `%v`; kycStep:%v,userId:%v,social:%v,reason:%v", sql, metadata.KYCStep, metadata.UserID, metadata.Social, reason)
}

func (r *repository) saveSocial(ctx context.Context, socialType Type, userID, userHandle string) error {
	sql := `INSERT INTO socials(user_id,social,user_handle) VALUES ($1,$2,$3)`
	_, err := storage.Exec(ctx, r.db, sql, userID, socialType, userHandle)

	return errors.Wrapf(err, "failed to `%v`; userID:%v, social:%v, userHandle:%v", sql, userID, socialType, userHandle)
}

func detectReason(err error) string {
	switch {
	case errors.Is(err, social.ErrInvalidPageContent):
		return "invalid page content"
	case errors.Is(err, social.ErrTextNotFound):
		return "expected text not found"
	case errors.Is(err, social.ErrUsernameNotFound):
		return "username not found"
	case errors.Is(err, social.ErrPostNotFound):
		return "post not found"
	case errors.Is(err, social.ErrInvalidURL):
		return "invalid URL"
	case storage.IsErr(err, storage.ErrDuplicate):
		if tErr := terror.As(err); tErr != nil && !storage.IsErr(err, storage.ErrDuplicate, "pk") {
			return fmt.Sprintf("duplicate userhandle '%v'", tErr.Data["user_handle"])
		}

		fallthrough
	default:
		return err.Error()
	}
}

func (vm *VerificationMetadata) expectedPostText(user *users.User) string {
	var templ *languageTemplate
	if val, found := allTemplates[vm.KYCStep][vm.Social][postContentLanguageTemplateType][vm.Language]; found {
		templ = val
	} else {
		templ = allTemplates[vm.KYCStep][vm.Social][postContentLanguageTemplateType]["en"]
	}
	bf := new(bytes.Buffer)
	log.Panic(errors.Wrapf(templ.content.Execute(bf, user), "failed to execute postContentLanguageTemplateType template for data:%#v", user))

	return bf.String()
}
