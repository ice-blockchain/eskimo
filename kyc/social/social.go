// SPDX-License-Identifier: ice License 1.0

package social

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"
	stdlibtime "time"

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
	cfg.alertFrequency = new(sync.Map)

	repo := &repository{
		user:            usrRepo,
		socialVerifiers: socialVerifiers,
		cfg:             &cfg,
		db:              storage.MustConnect(ctx, ddl, applicationYamlKey),
	}
	for _, kycStep := range AllSupportedKYCSteps {
		cfg.alertFrequency.Store(kycStep, alertFrequency)
		go repo.startUnsuccessfulKYCStepsAlerter(ctx, kycStep)
	}

	return repo
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
	if err = r.validateKycStep(user.User, kycStep, now); err != nil {
		return errors.Wrap(err, "failed to validateKycStep")
	}
	metadata := &VerificationMetadata{UserID: userID, Social: TwitterType, KYCStep: kycStep}
	skippedCount, err := r.verifySkipped(ctx, metadata, now)
	if err != nil {
		return errors.Wrapf(err, "failed to verifySkipped for metadata:%#v", metadata)
	}
	if err = r.saveUnsuccessfulAttempt(ctx, now, skippedReason, metadata); err != nil {
		return errors.Wrapf(err, "failed to saveUnsuccessfulAttempt reason:%v,metadata:%#v", skippedReason, metadata)
	}

	return errors.Wrapf(r.modifyUser(ctx, skippedCount+1 == r.cfg.MaxSessionsAllowed, true, kycStep, now, user.User),
		"[skip][kycStep:%v][count:%v]failed to modifyUser", kycStep, skippedCount+1)
}

func (r *repository) verifySkipped(ctx context.Context, metadata *VerificationMetadata, now *time.Time) (int, error) {
	sql := `SELECT count(1) AS skipped,
                   max(created_at) AS latest_created_at
		    FROM social_kyc_unsuccessful_attempts 
		    WHERE user_id = $1
			  AND kyc_step = $2
			  AND reason = ANY($3)`
	res, err := storage.Get[struct {
		LatestCreatedAt *time.Time `db:"latest_created_at"`
		SkippedCount    int        `db:"skipped"`
	}](ctx, r.db, sql, metadata.UserID, metadata.KYCStep, []string{skippedReason, exhaustedRetriesReason})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get skipped attempt count for kycStep:%v,userID:%v", metadata.KYCStep, metadata.UserID)
	}
	if !res.LatestCreatedAt.IsNil() && now.Sub(*res.LatestCreatedAt.Time) < r.cfg.DelayBetweenSessions {
		return 0, ErrNotAvailable
	}
	if res.SkippedCount >= r.cfg.MaxSessionsAllowed {
		return 0, errors.Wrap(ErrDuplicate, "potential de-sync between social_kyc_unsuccessful_attempts and users")
	}

	return res.SkippedCount, nil
}

//nolint:funlen,gocognit,gocyclo,revive,cyclop // .
func (r *repository) VerifyPost(ctx context.Context, metadata *VerificationMetadata) (*Verification, error) {
	now := time.Now()
	user, err := r.user.GetUserByID(ctx, metadata.UserID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GetUserByID: %v", metadata.UserID)
	}
	if err = r.validateKycStep(user.User, metadata.KYCStep, now); err != nil {
		return nil, errors.Wrap(err, "failed to validateKycStep")
	}
	skippedCount, err := r.verifySkipped(ctx, metadata, now)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to verifySkipped for metadata:%#v", metadata)
	}
	sql := `SELECT ARRAY_AGG(x.created_at) AS unsuccessful_attempts 
			FROM (SELECT created_at 
				  FROM social_kyc_unsuccessful_attempts 
				  WHERE user_id = $1
				    AND kyc_step = $2
				    AND reason != ANY($3)
				  ORDER BY created_at DESC) x`
	res, err := storage.Get[struct {
		UnsuccessfulAttempts *[]time.Time `db:"unsuccessful_attempts"`
	}](ctx, r.db, sql, metadata.UserID, metadata.KYCStep, []string{skippedReason, exhaustedRetriesReason})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get unsuccessful_attempts for kycStep:%v,userID:%v", metadata.KYCStep, metadata.UserID)
	}
	remainingAttempts := r.cfg.MaxAttemptsAllowed
	if res.UnsuccessfulAttempts != nil {
		for _, unsuccessfulAttempt := range *res.UnsuccessfulAttempts {
			if unsuccessfulAttempt.After(now.Add(-r.cfg.SessionWindow)) {
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
		ExpectedPostURL:  r.cfg.SocialLinks[metadata.Social].PostURLs[metadata.KYCStep],
	}
	if pvm.ExpectedPostURL == "" {
		log.Warn(fmt.Sprintf("post url not found for KYCStep:%v,Social:%v", metadata.KYCStep, metadata.Social))
	}
	if true { // Because we want to be less strict, for the moment.
		pvm.ExpectedPostText = fmt.Sprintf("%q", user.Username)
	}
	if metadata.Language == "zzzzzzzzzz" { // This is for testing purposes.
		stdlibtime.Sleep(120 * stdlibtime.Second) //nolint:gomnd // .
	} else if metadata.Language == "yyyyyyyyyy" {
		stdlibtime.Sleep(90 * stdlibtime.Second) //nolint:gomnd // .
	}
	userHandle, err := r.socialVerifiers[metadata.Social].VerifyPost(ctx, pvm)
	if err != nil { //nolint:nestif // .
		log.Error(errors.Wrapf(err, "social verification failed for KYCStep:%v,Social:%v,Language:%v,userID:%v",
			metadata.KYCStep, metadata.Social, metadata.Language, metadata.UserID))
		reason := detectReason(err)
		if userHandle != "" {
			reason = strings.ToLower(userHandle) + ": " + reason
		}
		if err = r.saveUnsuccessfulAttempt(ctx, now, reason, metadata); err != nil {
			return nil, errors.Wrapf(err, "[1]failed to saveUnsuccessfulAttempt reason:%v,metadata:%#v", reason, metadata)
		}
		remainingAttempts--
		if remainingAttempts == 0 {
			if err = r.saveUnsuccessfulAttempt(ctx, time.New(now.Add(stdlibtime.Microsecond)), exhaustedRetriesReason, metadata); err != nil {
				return nil, errors.Wrapf(err, "[1]failed to saveUnsuccessfulAttempt reason:%v,metadata:%#v", exhaustedRetriesReason, metadata)
			}
			end := skippedCount+1 == r.cfg.MaxSessionsAllowed

			if err = r.modifyUser(ctx, end, end, metadata.KYCStep, now, user.User); err != nil {
				return nil, errors.Wrapf(err, "[1failure][%v]failed to modifyUser", metadata.KYCStep)
			}
		}

		return &Verification{RemainingAttempts: &remainingAttempts, Result: FailureVerificationResult}, nil
	}
	if userHandle != "" { //nolint:nestif // .
		userHandle = strings.ToLower(userHandle)
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
					if err = r.saveUnsuccessfulAttempt(ctx, time.New(now.Add(stdlibtime.Microsecond)), exhaustedRetriesReason, metadata); err != nil {
						return nil, errors.Wrapf(err, "[2]failed to saveUnsuccessfulAttempt reason:%v,metadata:%#v", exhaustedRetriesReason, metadata)
					}
					end := skippedCount+1 == r.cfg.MaxSessionsAllowed
					if err = r.modifyUser(ctx, end, end, metadata.KYCStep, now, user.User); err != nil {
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
	if err = r.modifyUser(ctx, true, false, metadata.KYCStep, now, user.User); err != nil {
		return nil, errors.Wrapf(err, "[success][%v]failed to modifyUser", metadata.KYCStep)
	}

	return &Verification{Result: SuccessVerificationResult}, nil
}

func (r *repository) validateKycStep(user *users.User, kycStep users.KYCStep, now *time.Time) error {
	if user.KYCStepPassed == nil ||
		*user.KYCStepPassed < kycStep-1 ||
		(user.KYCStepPassed != nil &&
			*user.KYCStepPassed == kycStep-1 &&
			user.KYCStepsLastUpdatedAt != nil &&
			len(*user.KYCStepsLastUpdatedAt) >= int(kycStep) &&
			!(*user.KYCStepsLastUpdatedAt)[kycStep-1].IsNil() &&
			now.Sub(*(*user.KYCStepsLastUpdatedAt)[kycStep-1].Time) < r.cfg.DelayBetweenSessions) {
		return ErrNotAvailable
	} else if user.KYCStepPassed != nil && *user.KYCStepPassed >= kycStep {
		return ErrDuplicate
	}

	return nil
}

//nolint:revive,funlen,gocognit // Nope.
func (r *repository) modifyUser(ctx context.Context, success, skip bool, kycStep users.KYCStep, now *time.Time, user *users.User) error {
	usr := new(users.User)
	usr.ID = user.ID
	usr.KYCStepsLastUpdatedAt = user.KYCStepsLastUpdatedAt

	switch {
	case success && skip:
		usr.KYCStepPassed = &kycStep
		if len(*usr.KYCStepsLastUpdatedAt) < int(kycStep) {
			*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
		} else {
			(*usr.KYCStepsLastUpdatedAt)[int(kycStep)-1] = now
		}
	case success && !skip:
		usr.KYCStepPassed = &kycStep
		if len(*usr.KYCStepsLastUpdatedAt) < int(kycStep) {
			*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
		} else {
			(*usr.KYCStepsLastUpdatedAt)[int(kycStep)-1] = now
		}
		if kycStep == users.Social1KYCStep {
			nextStep := kycStep + 1
			usr.KYCStepPassed = &nextStep
			if len(*usr.KYCStepsLastUpdatedAt) < int(nextStep) {
				*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
			} else {
				(*usr.KYCStepsLastUpdatedAt)[int(nextStep)-1] = now
			}
		}
	case !success:
		if len(*usr.KYCStepsLastUpdatedAt) < int(kycStep) {
			*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
		} else {
			(*usr.KYCStepsLastUpdatedAt)[int(kycStep)-1] = now
		}
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
	if err != nil && storage.IsErr(err, storage.ErrDuplicate, "pk") {
		sql = `SELECT 1 WHERE EXISTS (SELECT true AS bogus FROM socials WHERE user_id = $1 AND social = $2 AND lower(user_handle) = $3)`
		if _, err2 := storage.ExecOne[struct{ Bogus bool }](ctx, r.db, sql, userID, socialType, userHandle); err2 == nil {
			return nil
		} else if !storage.IsErr(err2, storage.ErrNotFound) {
			err = errors.Wrapf(err2, "failed to check if user used the same userhandle previously; userID:%v, social:%v, userHandle:%v",
				userID, socialType, userHandle)
		}
	}

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
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancellation"
	case errors.Is(err, social.ErrFetchFailed):
		return "post fetch failed"
	case storage.IsErr(err, storage.ErrDuplicate):
		if tErr := terror.As(err); tErr != nil {
			if unwrapped := tErr.Unwrap(); storage.IsErr(unwrapped, storage.ErrDuplicate, "pk") {
				return fmt.Sprintf("duplicate socials '%v'", tErr.Data["user_handle"])
			} else if storage.IsErr(unwrapped, storage.ErrDuplicate) {
				return fmt.Sprintf("duplicate userhandle '%v'", tErr.Data["user_handle"])
			}
		}

		fallthrough
	default:
		return "unexpected"
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
