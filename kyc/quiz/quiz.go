// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	"fmt"
	"sync/atomic"
	stdlibtime "time"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	appcfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func mustLoadConfig() config { //nolint:funlen // .
	var cfg config

	appcfg.MustLoadFromKey(applicationYamlKey, &cfg)

	if cfg.MaxSessionDurationSeconds == 0 {
		panic("maxSessionDurationSeconds is not set")
	}

	if cfg.MaxQuestionsPerSession == 0 {
		panic("maxQuestionsPerSession is not set")
	}

	if cfg.SessionCoolDownSeconds == 0 {
		panic("sessionCoolDownSeconds is not set")
	}

	if cfg.MaxResetCount == nil {
		panic("maxResetCount is not set")
	}

	if cfg.GlobalStartDate == "" {
		panic("globalStartDate is not set")
	}
	globalStartDate, err := stdlibtime.ParseInLocation(stdlibtime.RFC3339Nano, cfg.GlobalStartDate, stdlibtime.UTC)
	log.Panic(err) //nolint:revive // .
	cfg.globalStartDate = time.New(globalStartDate)

	if cfg.MaxWrongAnswersPerSession == 0 {
		panic("maxWrongAnswersPerSession is not set")
	}

	if cfg.AvailabilityWindowSeconds == 0 {
		panic("availabilityWindowSeconds is not set")
	}

	if cfg.MaxAttemptsAllowed == 0 {
		panic("maxAttemptsAllowed is not set")
	}

	defaultAlertFrequency := alertFrequency
	cfg.alertFrequency = new(atomic.Pointer[stdlibtime.Duration])
	cfg.alertFrequency.Store(&defaultAlertFrequency)

	return cfg
}

func (e *quizError) Error() string {
	return e.Msg
}

func newError(msg string) error {
	return &quizError{Msg: msg}
}

func NewRepository(ctx context.Context, userRepo UserRepository) Repository {
	repo := newRepositoryImpl(ctx, userRepo)
	go repo.startAlerter(ctx)
	go repo.startKYCConfigJSONSyncer(ctx)

	return repo
}

func newRepositoryImpl(ctx context.Context, userRepo UserRepository) *repositoryImpl {
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &repositoryImpl{
		DB:       db,
		Shutdown: db.Close,
		Users:    userRepo,
		config:   mustLoadConfig(),
	}
}

func (r *repositoryImpl) Close() (err error) {
	if r.Shutdown != nil {
		err = r.Shutdown()
	}

	return
}

func (r *repositoryImpl) CheckUserKYC(ctx context.Context, userID UserID) error {
	profile, err := r.Users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errors.Wrap(ErrUnknownUser, userID)
		}

		return errors.Wrapf(err, "failed to get user by id: %v", userID)
	}

	return r.validateKycStep(profile.User)
}

func (r *repositoryImpl) validateKycStep(user *users.User) error {
	if user.KYCStepBlocked != nil && *user.KYCStepBlocked > 0 &&
		!(*user.KYCStepBlocked == users.LivenessDetectionKYCStep && user.KYCStepPassed != nil && *user.KYCStepPassed == users.FacialRecognitionKYCStep) {
		return ErrNotAvailable
	}

	if sessionCoolDown := stdlibtime.Duration(r.config.SessionCoolDownSeconds) * stdlibtime.Second; user.KYCStepPassed == nil ||
		*user.KYCStepPassed < users.FacialRecognitionKYCStep ||
		(user.KYCStepPassed != nil &&
			user.KYCStepsLastUpdatedAt != nil &&
			len(*user.KYCStepsLastUpdatedAt) >= int(users.QuizKYCStep) &&
			!(*user.KYCStepsLastUpdatedAt)[users.QuizKYCStep-1].IsNil() &&
			time.Now().Sub(*(*user.KYCStepsLastUpdatedAt)[users.QuizKYCStep-1].Time) < sessionCoolDown) {
		return ErrInvalidKYCState
	}

	return nil
}

func (r *repositoryImpl) addFailedAttempt(ctx context.Context, userID UserID, now *time.Time, tx storage.QueryExecer, skipped bool) (bool, error) {
	// $1: user_id.
	// $2: now.
	// $3: skipped.
	const stmt = `
		insert into failed_quiz_sessions (started_at, ended_at, questions, answers, language, user_id, skipped)
		values ($2, $2, '{}', '{}', 'en', $1, $3)
	`
	_, err := storage.Exec(ctx, tx, stmt, userID, now.Time, skipped)
	if err != nil {
		return false, errors.Wrap(err, "failed to add failed attempt")
	}

	return r.moveFailedAttempts(ctx, tx, now, userID)
}

func (r *repositoryImpl) SkipQuizSession(ctx context.Context, userID UserID) error { //nolint:funlen //.
	// $1: user_id.
	const stmt = `
	select
		ended_at is not null as finished,
		ended_successfully
	from
		quiz_sessions
	where
		user_id = $1
	for update
	`

	if err := r.CheckUserKYC(ctx, userID); err != nil {
		return err
	}

	err := storage.DoInTransaction(ctx, r.DB, func(tx storage.QueryExecer) error {
		now := time.Now()

		blocked := false
		data, err := storage.ExecOne[struct {
			Finished bool `db:"finished"`
			Success  bool `db:"ended_successfully"`
		}](ctx, tx, stmt, userID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				blocked, err = r.addFailedAttempt(ctx, userID, now, tx, true)
				if err == nil {
					err = r.modifyUser(ctx, false, blocked, now, userID)
				}

				return err
			}

			return errors.Wrap(err, "failed to get session data")
		}

		if data.Finished {
			if data.Success {
				return ErrSessionFinished
			}

			return r.modifyUser(ctx, false, blocked, now, userID)
		}

		return r.UserMarkSessionAsFinished(ctx, userID, *now.Time, tx, false, true)
	})

	return errors.Wrap(err, "failed to skip session")
}

func (r *repositoryImpl) prepareUserForReset(ctx context.Context, userID UserID, now *time.Time, tx storage.QueryExecer) error {
	count, err := r.NumberOfFailedAttempts(ctx, tx, userID)
	if err != nil {
		return errors.Wrap(err, "failed to get failed attempts count")
	}

	if count < int(r.config.MaxAttemptsAllowed) {
		for i := 0; i < int(r.config.MaxAttemptsAllowed)-count; i++ {
			ts := now.Add(-stdlibtime.Second * stdlibtime.Duration(i))
			_, err = r.addFailedAttempt(ctx, userID, time.New(ts), tx, true)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *repositoryImpl) CheckQuizStatus(ctx context.Context, userID UserID) (*QuizStatus, error) { //nolint:funlen //.
	status, err := r.getQuizStatus(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	reRead := false
	if status.HasUnfinishedSessions {
		_, err = r.finishUnfinishedSession(ctx, r.DB, now, userID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to finish unfinished session for userID:%v", userID)
		}
		reRead = true
	}

	if !status.KYCQuizCompleted && !status.KYCQuizDisabled && status.KYCQuizAvailabilityEndedAt.Before(*now.Time) {
		err = storage.DoInTransaction(ctx, r.DB, func(tx storage.QueryExecer) error {
			prepareErr := r.prepareUserForReset(ctx, userID, now, tx)
			if prepareErr != nil {
				return prepareErr
			}

			return r.modifyUser(ctx, false, true, now, userID)
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to prepare user for reset for userID:%v", userID)
		}
		reRead = true
	}

	if reRead {
		status, err = r.getQuizStatus(ctx, userID)
	}

	return status, err
}

func (r *repositoryImpl) getQuizStatus(ctx context.Context, userID UserID) (*QuizStatus, error) { //nolint:funlen //.
	// $1: user_id.
	// $2: global start date.
	// $3: availability window (seconds).
	// $4: max reset count.
	// $5: max attempts allowed.
	const sql = `SELECT GREATEST($5 - coalesce(count(fqs.user_id),0),0)			  						                        AS kyc_quiz_remaining_attempts,
				   (qr.user_id IS NOT NULL AND cardinality(qr.resets) > $4) 							                        AS kyc_quiz_disabled,
				   qr.resets  							 									 			                        AS kyc_quiz_reset_at,
				   (qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true)                        AS kyc_quiz_completed,
				   GREATEST(u.created_at,$2)  	  							 							                        AS kyc_quiz_availability_started_at,
				   GREATEST(u.created_at,$2) + (interval '1 second' * $3) 	  							                        AS kyc_quiz_availability_ended_at,
				   ((u.kyc_step_passed >= 2 AND u.kyc_step_blocked = 0) OR (u.kyc_step_passed = 1 AND u.kyc_step_blocked = 2))  AS kyc_quiz_available,
				   (qs.user_id IS NOT NULL AND qs.ended_at IS NULL)			  							                        AS has_unfinished_sessions
			FROM users u
				LEFT JOIN quiz_resets qr 
  					   ON qr.user_id = u.id
				LEFT JOIN quiz_sessions qs
					   ON qs.user_id = u.id
				LEFT JOIN failed_quiz_sessions fqs
					   ON fqs.user_id = u.id
					  AND fqs.started_at >= GREATEST(u.created_at,$2) 
			WHERE u.id = $1
			GROUP BY qr.user_id,
					 qs.user_id,
					 u.id`
	quizStatus, err := storage.ExecOne[QuizStatus](
		ctx,
		r.DB,
		sql,
		userID,
		r.config.globalStartDate.Time,
		r.config.AvailabilityWindowSeconds,
		*r.config.MaxResetCount,
		r.config.MaxAttemptsAllowed,
	)

	if errors.Is(err, storage.ErrNotFound) {
		err = ErrUnknownSession
	}
	if quizStatus != nil {
		quizStatus.KYCQuizAvailable = quizStatus.KYCQuizAvailable && (r.isKYCEnabled(ctx) || r.isKYCStepForced(userID))
	}

	return quizStatus, errors.Wrapf(err, "failed to exec CheckQuizStatus sql for userID:%v", userID)
}

func (r *repositoryImpl) SelectQuestions(ctx context.Context, tx storage.QueryExecer, lang string) ([]*Question, error) {
	const stmt = `
select id, options, question from questions where "language" = $1 order by random() limit $2
	`

	questions, err := storage.Select[Question](ctx, tx, stmt, lang, r.config.MaxQuestionsPerSession)
	if err != nil {
		return nil, errors.Wrap(err, "failed to select questions")
	} else if len(questions) == 0 {
		return nil, errors.Wrap(ErrUnknownLanguage, lang)
	}

	if len(questions) < r.config.MaxQuestionsPerSession {
		panic(fmt.Sprintf("not enough questions for language %v: wanted %d but has only %v",
			lang, r.config.MaxQuestionsPerSession, len(questions)))
	}

	for i := range questions {
		questions[i].Number = uint8(i + 1)
	}

	return questions, nil
}

func questionsToSlice(questions []*Question) []uint {
	result := make([]uint, 0, len(questions))
	for i := range questions {
		result = append(result, questions[i].ID)
	}

	return result
}

func (r *repositoryImpl) NumberOfFailedAttempts(ctx context.Context, tx storage.QueryExecer, userID UserID) (int, error) {
	// $1: user_id.
	// $2: global start date.
	const stmt = `
select
	count(1)
from
	failed_quiz_sessions
join users u on
	id = user_id
where
	user_id = $1 and
	started_at >= GREATEST(u.created_at, $2)
`
	count, err := storage.ExecOne[int](ctx, tx, stmt, userID, r.config.globalStartDate.Time)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return 0, nil
		}

		return 0, errors.Wrap(err, "failed to get failed attempts count")
	}

	return *count, nil
}

func (r *repositoryImpl) moveFailedAttempts(ctx context.Context, tx storage.QueryExecer, now *time.Time, userID UserID) (bool, error) { //nolint:funlen //.
	count, err := r.NumberOfFailedAttempts(ctx, tx, userID)
	if err != nil {
		return false, errors.Wrap(err, "failed to get failed attempts count")
	}

	if count < int(r.config.MaxAttemptsAllowed) {
		return false, nil
	}

	const stmt = `
with failed_sessions as (
	delete from failed_quiz_sessions where user_id = $1 returning *
)
insert into failed_quiz_sessions_history(created_at, started_at, ended_at, questions, answers, language, user_id, skipped)
select
	$2 as created_at,
	failed_sessions.started_at,
	failed_sessions.ended_at,
	failed_sessions.questions,
	failed_sessions.answers,
	failed_sessions.language,
	failed_sessions.user_id,
	failed_sessions.skipped
from
	failed_sessions
`

	_, err = storage.Exec(ctx, tx, stmt, userID, now.Time)
	if err != nil {
		return false, errors.Wrap(err, "failed to move failed attempts to history")
	}

	const stmtReset = `
insert into quiz_resets
	(user_id, resets)
values
	($1, ARRAY[$2::timestamp])
on conflict on constraint quiz_resets_pkey do update set
	resets = quiz_resets.resets || excluded.resets
`

	_, err = storage.Exec(ctx, tx, stmtReset, userID, now.Time)

	return true, errors.Wrap(err, "failed to reset quiz resets")
}

//nolint:funlen //.
func (r *repositoryImpl) finishUnfinishedSession(
	ctx context.Context,
	tx storage.QueryExecer,
	now *time.Time,
	userID UserID,
) (*time.Time, error) {
	// $1: user_id.
	// $2: now.
	// $3: session cool down (seconds).
	const stmt = `
	with result as (
		update quiz_sessions
		set
			ended_at = $2,
			ended_successfully = false
		where
			user_id = $1 and
			ended_at is null
		returning *
	)
	insert into failed_quiz_sessions (started_at, ended_at, questions, answers, language, user_id, skipped)
	select
		result.started_at,
		result.ended_at,
		result.questions,
		result.answers,
		result.language,
		result.user_id,
		false
	from
		result
	returning
		ended_at + make_interval(secs => $3) as cooldown_at
	`
	data, err := storage.ExecOne[struct {
		CooldownAt *time.Time `db:"cooldown_at"`
	}](ctx, tx, stmt, userID, now.Time, r.config.SessionCoolDownSeconds)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			err = nil
		}

		return nil, err
	}

	blocked := false
	var cooldownAt *time.Time
	if data != nil && !data.CooldownAt.IsNil() {
		blocked, err = r.moveFailedAttempts(ctx, tx, now, userID)
		if err != nil {
			return nil, err
		} else if !blocked {
			cooldownAt = data.CooldownAt
		}
	}

	return cooldownAt, errors.Wrapf(r.modifyUser(ctx, false, blocked, now, userID), "failed to modifyUser")
}

func (r *repositoryImpl) startNewSession( //nolint:funlen //.
	ctx context.Context,
	userID UserID,
	lang string,
	questions []*Question,
) (*Quiz, error) {
	// $1: user_id.
	// $2: language.
	// $3: questions.
	// $4: session cool down (seconds).
	// $5: max session duration (seconds).
	const stmt = `
	with session_failed as (
		select
			max(ended_at) as ended_at
		from
			failed_quiz_sessions
		where
			user_id = $1
		having
			max(ended_at) > (now() - make_interval(secs => $4))
	),
	session_active as (
		select
			quiz_sessions.started_at,
			quiz_sessions.started_at + make_interval(secs => $5) as deadline,
			quiz_sessions.ended_at,
			quiz_sessions.ended_at is not null as finished,
			quiz_sessions.ended_successfully
		from
			quiz_sessions
		where
			quiz_sessions.user_id = $1 and
			not exists (select false from session_failed)
		for update
	),
	session_upsert as (
		insert into quiz_sessions
			(user_id, language, questions, started_at, answers)
		select
			$1,
			$2,
			$3,
			now(),
			'{}'::smallint[]
		where
			coalesce((select false from session_failed), true) and
			coalesce((select
						(finished is false and session_active.deadline < now()) or
						(finished is true and ended_successfully is false and ((ended_at + make_interval(secs => $4)) < now()))
					from
						session_active), true)
		on conflict on constraint quiz_sessions_pkey do
		update
		set
			ended_at = null,
			ended_successfully = false,
			started_at = excluded.started_at,
			questions = excluded.questions,
			answers = excluded.answers,
			language = excluded.language
		returning
			quiz_sessions.*,
			quiz_sessions.started_at + make_interval(secs => $5) as deadline
	)
	select
		session_failed.ended_at as failed_at,
		session_active.started_at as active_started_at,
		session_active.deadline as active_deadline,
		session_active.finished as active_finished,
		session_active.ended_successfully as active_ended_successfully,
		session_active.ended_at as active_ended_at,
		session_upsert.started_at as upsert_started_at,
		session_upsert.deadline as upsert_deadline
	from
		(values(true)) dummy
	full outer join session_failed on true
	full outer join session_active on true
	full outer join session_upsert on true
`

	data, err := storage.ExecOne[struct {
		FailedAt                *time.Time `db:"failed_at"`
		ActiveStartedAt         *time.Time `db:"active_started_at"`
		ActiveDeadline          *time.Time `db:"active_deadline"`
		ActiveFinished          *bool      `db:"active_finished"`
		ActiveEndedSuccessfully *bool      `db:"active_ended_successfully"`
		ActiveEndedAt           *time.Time `db:"active_ended_at"`
		UpsertStartedAt         *time.Time `db:"upsert_started_at"`
		UpsertDeadline          *time.Time `db:"upsert_deadline"`
	}](ctx, r.DB, stmt, userID, lang, questionsToSlice(questions), r.config.SessionCoolDownSeconds, r.config.MaxSessionDurationSeconds)
	if err != nil {
		if errors.Is(err, storage.ErrRelationNotFound) {
			err = ErrUnknownUser
		}

		return nil, errors.Wrap(err, "failed to start session")
	}

	switch {
	case data.FailedAt != nil: // Failed session is still in cool down.
		return nil, errors.Wrapf(ErrSessionFinishedWithError, "wait until %v",
			data.FailedAt.Add(stdlibtime.Duration(r.config.SessionCoolDownSeconds)*stdlibtime.Second))

	case data.ActiveStartedAt != nil && data.UpsertStartedAt == nil: // Active session is still running or ended with some result.
		if *data.ActiveFinished {
			if *data.ActiveEndedSuccessfully {
				return nil, ErrSessionFinished
			}

			return nil, ErrSessionFinishedWithError
		}

	case data.UpsertStartedAt != nil: // New session is started.
		return &Quiz{
			Progress: &Progress{
				ExpiresAt:    data.UpsertDeadline,
				NextQuestion: questions[0],
				MaxQuestions: uint8(len(questions)),
			},
		}, nil
	}

	panic("unreachable: " + userID)
}

func (r *repositoryImpl) IsQuizEnabledForUser(ctx context.Context, userID UserID) (bool, error) {
	const stmt = `select false as val from quiz_resets where user_id = $1 and cardinality(resets) > $2`

	_, err := storage.ExecOne[struct {
		Val bool `db:"val"`
	}](ctx, r.DB, stmt, userID, *r.config.MaxResetCount)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return true, nil
		}

		return false, errors.Wrap(err, "failed to check quiz resets")
	}

	return false, nil
}

func (r *repositoryImpl) StartQuizSession(ctx context.Context, userID UserID, lang string) (*Quiz, error) {
	questions, err := r.SelectQuestions(ctx, r.DB, lang)
	if err != nil {
		return nil, err
	}

	cooldown, err := r.finishUnfinishedSession(ctx, r.DB, time.Now(), userID)
	if err != nil {
		return nil, err
	} else if cooldown != nil {
		return nil, errors.Wrapf(ErrSessionFinishedWithError, "cooldown until %v", cooldown)
	}

	enabled, err := r.IsQuizEnabledForUser(ctx, userID)
	if err != nil {
		return nil, err
	} else if !enabled {
		return nil, ErrNotAvailable
	}

	err = r.CheckUserKYC(ctx, userID)
	if err != nil {
		return nil, err
	}

	return r.startNewSession(ctx, userID, lang, questions)
}

func calculateProgress(correctAnswers, currentAnswers []uint8) (correctNum, incorrectNum uint8) {
	correct := correctAnswers
	if len(currentAnswers) < len(correctAnswers) {
		correct = correctAnswers[:len(currentAnswers)]
	}

	for i := range correct {
		if correct[i] == currentAnswers[i] {
			correctNum++
		} else {
			incorrectNum++
		}
	}

	return
}

func (r *repositoryImpl) CheckUserRunningSession( //nolint:funlen //.
	ctx context.Context,
	userID UserID,
	now stdlibtime.Time,
	tx storage.QueryExecer,
) (userProgress, error) {
	type userSession struct {
		userProgress
		Finished             bool `db:"finished"`
		FinishedSuccessfully bool `db:"ended_successfully"`
	}

	// $1: user_id.
	// $2: max session duration (seconds).
	const stmt = `
select
	started_at,
	started_at + make_interval(secs => $2) as deadline,
	ended_at is not null as finished,
	questions,
	session.language,
	answers,
	array_agg(questions.correct_option order by q.nr) as correct_answers,
	ended_successfully
from
	quiz_sessions session,
	questions
	inner join unnest(session.questions) with ordinality AS q(id, nr)
	on questions.id = q.id
where
	user_id = $1 and
	questions."language" = session.language
group by
	started_at,
	ended_at,
	questions,
	session.language,
	answers,
	ended_successfully
`

	data, err := storage.ExecOne[userSession](ctx, tx, stmt, userID, r.config.MaxSessionDurationSeconds)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return userProgress{}, ErrUnknownSession
		}

		return userProgress{}, errors.Wrap(err, "failed to get running session data")
	}

	if data.Finished {
		if data.FinishedSuccessfully {
			return userProgress{}, ErrSessionFinished
		}

		return userProgress{}, ErrSessionFinishedWithError
	}

	deadline := data.StartedAt.Add(stdlibtime.Duration(r.config.MaxSessionDurationSeconds) * stdlibtime.Second)
	if deadline.Before(now) {
		return userProgress{}, errSessionExpired
	}

	return data.userProgress, nil
}

func (*repositoryImpl) CheckQuestionNumber(ctx context.Context, lang string, questions []uint8, num uint8, tx storage.QueryExecer) (uint8, error) {
	type currentQuestion struct {
		CorrectOption uint8 `db:"correct_option"`
	}

	if num == 0 || num > uint8(len(questions)) {
		return 0, ErrUnknownQuestionNumber
	}

	data, err := storage.ExecOne[currentQuestion](ctx, tx, `select correct_option from questions where id = $1 and "language" = $2`, questions[num-1], lang)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return 0, ErrUnknownQuestionNumber
		}

		return 0, errors.Wrap(err, "failed to get current question data")
	}

	return data.CorrectOption, nil
}

func (*repositoryImpl) UserAddAnswer(ctx context.Context, userID UserID, tx storage.QueryExecer, answer uint8) ([]uint8, error) {
	const stmt = `
update quiz_sessions
set
	answers = array_append(answers, $2)
where
	user_id = $1
returning answers
	`

	data, err := storage.ExecOne[userProgress](ctx, tx, stmt, userID, answer)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrUnknownSession
		}

		return nil, errors.Wrap(err, "failed to update session")
	}

	return data.Answers, nil
}

func (*repositoryImpl) LoadQuestionByID(ctx context.Context, tx storage.QueryExecer, lang string, questionID uint8) (*Question, error) {
	const stmt = `
select id, options, question from questions where "language" = $1 and id = $2
	`

	question, err := storage.Get[Question](ctx, tx, stmt, lang, questionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to select questions")
	}

	return question, nil
}

//nolint:revive,funlen // .
func (r *repositoryImpl) UserMarkSessionAsFinished(
	ctx context.Context, userID UserID, now stdlibtime.Time, tx storage.QueryExecer, successful, skipped bool,
) error {
	const stmt = `
with result as (
	update quiz_sessions
	set
		ended_at = $3,
		ended_successfully = $2
	where
		user_id = $1
	returning *
)
insert into failed_quiz_sessions (started_at, ended_at, questions, answers, language, user_id, skipped)
select
	result.started_at,
	result.ended_at,
	result.questions,
	result.answers,
	result.language,
	result.user_id,
	$4 AS skipped
from result
where
	result.ended_successfully = false
	`
	if _, err := storage.Exec(ctx, tx, stmt, userID, successful, now, skipped); err != nil {
		return errors.Wrap(err, "failed to mark session as finished")
	}

	blocked, err := r.moveFailedAttempts(ctx, tx, time.New(now), userID)
	if err != nil {
		return err
	}

	return errors.Wrap(r.modifyUser(ctx, successful, blocked, time.New(now), userID), "failed to modifyUser")
}

func (r *repositoryImpl) fetchUserProfileForModify(ctx context.Context, userID UserID) (*users.User, error) {
	profile, err := r.Users.GetUserByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id: %v", userID)
	}

	usr := new(users.User)
	usr.ID = userID
	usr.KYCStepPassed = profile.KYCStepPassed
	usr.KYCStepsLastUpdatedAt = profile.KYCStepsLastUpdatedAt
	usr.KYCStepsCreatedAt = profile.KYCStepsCreatedAt

	if usr.KYCStepsLastUpdatedAt == nil {
		s := make([]*time.Time, 0, 1)
		usr.KYCStepsLastUpdatedAt = &s
	}
	if usr.KYCStepsCreatedAt == nil {
		s := make([]*time.Time, 0, 1)
		usr.KYCStepsCreatedAt = &s
	}

	return usr, nil
}

//nolint:revive // .
func (r *repositoryImpl) modifyUser(ctx context.Context, success, blocked bool, now *time.Time, userID UserID) error {
	user, err := r.fetchUserProfileForModify(ctx, userID)
	if err != nil {
		return err
	}
	usr := new(users.User)
	usr.ID = user.ID

	newKYCStep := users.QuizKYCStep
	if success || blocked {
		if user.KYCStepPassed != nil && *user.KYCStepPassed == newKYCStep-1 {
			usr.KYCStepPassed = &newKYCStep
		}

		if blocked {
			usr.KYCStepBlocked = &newKYCStep
		}
	}

	usr.KYCStepsLastUpdatedAt = user.KYCStepsLastUpdatedAt
	for len(*usr.KYCStepsLastUpdatedAt) < int(newKYCStep) {
		*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
	}
	(*usr.KYCStepsLastUpdatedAt)[int(newKYCStep)-1] = now

	return errors.Wrapf(r.Users.ModifyUser(ctx, usr, nil), "failed to modify user %#v", usr)
}

func (r *repositoryImpl) ContinueQuizSession( //nolint:funlen,revive,gocognit //.
	ctx context.Context,
	userID UserID,
	question, answer uint8,
) (quiz *Quiz, err error) {
	err = storage.DoInTransaction(ctx, r.DB, func(tx storage.QueryExecer) error {
		now := stdlibtime.Now().Truncate(stdlibtime.Second).UTC()
		progress, pErr := r.CheckUserRunningSession(ctx, userID, now, tx)
		if pErr != nil {
			if errors.Is(pErr, errSessionExpired) {
				quiz = &Quiz{Result: FailureResult}
				pErr = r.UserMarkSessionAsFinished(ctx, userID, now, tx, false, false)
			}

			return pErr
		}
		_, err = r.CheckQuestionNumber(ctx, progress.Lang, progress.Questions, question, tx)
		if err != nil {
			return err
		}
		var answeredQuestionsCount int
		switch {
		case int(question)-len(progress.Answers) < 0 || question-uint8(len(progress.Answers)) > 1:
			return errors.Wrap(ErrUnknownQuestionNumber, "please answer questions in order")
		case uint8(len(progress.Answers)) == question-1:
			newAnswers, aErr := r.UserAddAnswer(ctx, userID, tx, answer)
			if aErr != nil {
				return aErr
			}
			answeredQuestionsCount = len(newAnswers)
			correctNum, incorrectNum := calculateProgress(progress.CorrectAnswers, newAnswers)
			quiz = &Quiz{
				Progress: &Progress{
					MaxQuestions:     uint8(len(progress.Questions)),
					CorrectAnswers:   correctNum,
					IncorrectAnswers: incorrectNum,
				},
			}

			if int(incorrectNum) > r.config.MaxWrongAnswersPerSession {
				quiz.Result = FailureResult

				return r.UserMarkSessionAsFinished(ctx, userID, now, tx, false, false)
			}
		default:
			answeredQuestionsCount = len(progress.Answers)
			correctNum, incorrectNum := calculateProgress(progress.CorrectAnswers, progress.Answers)
			quiz = &Quiz{
				Progress: &Progress{
					MaxQuestions:     uint8(len(progress.Questions)),
					CorrectAnswers:   correctNum,
					IncorrectAnswers: incorrectNum,
				},
			}
		}

		if answeredQuestionsCount != len(progress.CorrectAnswers) {
			nextQuestion, nErr := r.LoadQuestionByID(ctx, tx, progress.Lang, progress.Questions[question])
			if nErr != nil {
				return nErr
			}
			nextQuestion.Number = question + 1
			quiz.Progress.ExpiresAt = progress.Deadline
			quiz.Progress.NextQuestion = nextQuestion

			return nil
		}

		quiz.Result = SuccessResult

		return r.UserMarkSessionAsFinished(ctx, userID, now, tx, true, false)
	})

	return quiz, err
}

func (r *repositoryImpl) ResetQuizSession(ctx context.Context, userID UserID) error {
	// $1: user_id.
	const stmt = `
		delete from quiz_sessions
		where
			user_id = $1
	`
	_, err := storage.Exec(ctx, r.DB, stmt, userID)

	return errors.Wrap(err, "failed to reset session")
}
