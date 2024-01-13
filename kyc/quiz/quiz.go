// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	"fmt"
	stdlibtime "time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	appcfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

func mustLoadConfig() config {
	var cfg config

	appcfg.MustLoadFromKey(applicationYamlKey, &cfg)

	if cfg.MaxSessionDurationSeconds == 0 {
		panic("max_session_duration_seconds is not set")
	}

	if cfg.MaxQuestionsPerSession == 0 {
		panic("max_questions_per_session is not set")
	}

	if cfg.SessionCoolDownSeconds == 0 {
		panic("session_cool_down_seconds is not set")
	}

	return cfg
}

func (e *quizError) Error() string {
	return e.Msg
}

func newError(msg string) error {
	return &quizError{Msg: msg}
}

func NewRepository(ctx context.Context, userRepo UserRepository) Repository {
	return newRepositoryImpl(ctx, userRepo)
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

//nolint:revive // .
func (r *repositoryImpl) validateKycStep(user *users.User) error {
	if sessionCoolDown := stdlibtime.Duration(r.config.SessionCoolDownSeconds) * stdlibtime.Second; user.KYCStepPassed == nil ||
		*user.KYCStepPassed < users.QuizKYCStep-1 ||
		(user.KYCStepPassed != nil &&
			*user.KYCStepPassed == users.QuizKYCStep-1 &&
			user.KYCStepsLastUpdatedAt != nil &&
			len(*user.KYCStepsLastUpdatedAt) >= int(users.QuizKYCStep) &&
			!(*user.KYCStepsLastUpdatedAt)[users.QuizKYCStep-1].IsNil() &&
			time.Now().Sub(*(*user.KYCStepsLastUpdatedAt)[users.QuizKYCStep-1].Time) < sessionCoolDown) ||
		user.KYCStepPassed != nil && *user.KYCStepPassed >= users.QuizKYCStep {
		return ErrInvalidKYCState
	}

	return nil
}

func (r *repositoryImpl) SkipQuizSession(ctx context.Context, userID UserID) error {
	if err := r.CheckUserKYC(ctx, userID); err != nil {
		return err
	}

	now := time.Now()
	for _, fn := range []func(context.Context, UserID, stdlibtime.Time, storage.QueryExecer) error{
		r.CheckUserFailedSession,
		r.CheckUserActiveSession,
	} {
		if err := fn(ctx, userID, *now.Time, r.DB); err != nil {
			return err
		}
	}

	return errors.Wrapf(r.UserMarkSessionAsFinished(ctx, userID, *now.Time, r.DB, false, true),
		"failed to UserMarkSessionAsFinished for userID:%v", userID)
}

func (r *repositoryImpl) CheckUserFailedSession(ctx context.Context, userID UserID, now stdlibtime.Time, tx storage.QueryExecer) error {
	type failedSession struct {
		EndedAt stdlibtime.Time `db:"ended_at"`
	}

	const stmt = `
select max(ended_at) as ended_at from failed_quiz_sessions where user_id = $1 having max(ended_at) > $2
	`

	term := now.
		Add(stdlibtime.Duration(-r.config.SessionCoolDownSeconds) * stdlibtime.Second).
		Truncate(stdlibtime.Second)
	data, err := storage.Get[failedSession](ctx, tx, stmt, userID, term)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}

		return errors.Wrap(err, "failed to get failed session data")
	}

	next := data.EndedAt.
		Add(stdlibtime.Duration(r.config.SessionCoolDownSeconds) * stdlibtime.Second).
		Truncate(stdlibtime.Second).
		UTC()

	return errors.Wrapf(ErrSessionFinishedWithError, "wait until %v", next)
}

func (r *repositoryImpl) CheckUserActiveSession(ctx context.Context, userID UserID, now stdlibtime.Time, tx storage.QueryExecer) error {
	type userSession struct {
		StartedAt            time.Time  `db:"started_at"`
		EndedAt              *time.Time `db:"ended_at"`
		Finished             bool       `db:"finished"`
		FinishedSuccessfully bool       `db:"ended_successfully"`
	}
	const stmt = `select started_at, ended_at, ended_at is not null as finished, ended_successfully from quiz_sessions where user_id = $1`

	data, err := storage.Get[userSession](ctx, tx, stmt, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}

		return errors.Wrap(err, "failed to get active session data")
	}

	if data.Finished {
		if data.FinishedSuccessfully {
			return ErrSessionFinished
		}

		cooldown := data.EndedAt.Add(stdlibtime.Duration(r.config.SessionCoolDownSeconds) * stdlibtime.Second)
		if cooldown.After(now) {
			return ErrSessionFinishedWithError
		}
	}

	deadline := data.StartedAt.Add(stdlibtime.Duration(r.config.MaxSessionDurationSeconds) * stdlibtime.Second)
	if deadline.After(now) {
		return ErrSessionIsAlreadyRunning
	}

	return nil
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

func (*repositoryImpl) CreateSessionEntry( //nolint:revive //.
	ctx context.Context,
	userID UserID,
	lang string,
	questions []*Question,
	now stdlibtime.Time,
	tx storage.QueryExecer,
) error {
	const stmt = `
insert into quiz_sessions (user_id, language, questions, started_at, answers) values ($1, $2, $3, $4, '{}'::smallint[])
	on conflict on constraint quiz_sessions_pkey do update
	set
		started_at = excluded.started_at,
		questions = excluded.questions,
		answers = excluded.answers,
		language = excluded.language,
		ended_successfully = false
	`

	_, err := storage.Exec(ctx, tx, stmt, userID, lang, questionsToSlice(questions), now)
	if err != nil {
		if errors.Is(err, storage.ErrRelationNotFound) {
			err = ErrUnknownUser
		}
	}

	return errors.Wrap(err, "failed to create session entry")
}

func wrapErrorInTx(err error) error {
	if err == nil {
		return nil
	}

	var quizErr *quizError
	if errors.As(err, &quizErr) {
		// Wa want to stop/abort the transaction in case of logic/flow error.
		return multierror.Append(storage.ErrCheckFailed, err)
	}

	return err
}

func (r *repositoryImpl) StartQuizSession(ctx context.Context, userID UserID, lang string) (*Quiz, error) { //nolint:funlen //.
	err := r.CheckUserKYC(ctx, userID)
	if err != nil {
		return nil, err
	}

	questions, err := r.SelectQuestions(ctx, r.DB, lang)
	if err != nil {
		return nil, err
	}

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
		(values(true))
	full outer join session_failed on true
	full outer join session_active on true
	full outer join session_upsert on true
`

	data, err := storage.Get[struct {
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

	now := stdlibtime.Now().Truncate(stdlibtime.Second).UTC()
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

		if data.ActiveDeadline.After(now) {
			return nil, errors.Wrapf(ErrSessionIsAlreadyRunning, "wait %s before next session", data.ActiveDeadline.Sub(now))
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
	const stmt = `
select
	started_at,
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

	data, err := storage.Get[userSession](ctx, tx, stmt, userID)
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
		return userProgress{}, ErrSessionExpired
	}

	return data.userProgress, nil
}

func (*repositoryImpl) CheckQuestionNumber(ctx context.Context, questions []uint8, num uint8, tx storage.QueryExecer) (uint8, error) {
	type currentQuestion struct {
		CorrectOption uint8 `db:"correct_option"`
	}

	if num == 0 || num > uint8(len(questions)) {
		return 0, ErrUnknownQuestionNumber
	}

	data, err := storage.Get[currentQuestion](ctx, tx, `select correct_option from questions where id = $1`, questions[num-1])
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

	data, err := storage.Get[userProgress](ctx, tx, stmt, userID, answer)
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

//nolint:revive // .
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

	return errors.Wrap(r.modifyUser(ctx, successful, time.New(now), userID), "failed to modifyUser")
}

func (r *repositoryImpl) fetchUserProfileForModify(ctx context.Context, userID UserID) (*users.User, error) {
	profile, err := r.Users.GetUserByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id: %v", userID)
	}

	usr := new(users.User)
	usr.ID = userID
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
func (r *repositoryImpl) modifyUser(ctx context.Context, success bool, now *time.Time, userID UserID) error {
	user, err := r.fetchUserProfileForModify(ctx, userID)
	if err != nil {
		return err
	}
	usr := new(users.User)
	usr.ID = user.ID

	newKYCStep := users.QuizKYCStep
	if success {
		usr.KYCStepPassed = &newKYCStep
	}

	usr.KYCStepsLastUpdatedAt = user.KYCStepsLastUpdatedAt
	if len(*usr.KYCStepsLastUpdatedAt) < int(newKYCStep) {
		*usr.KYCStepsLastUpdatedAt = append(*usr.KYCStepsLastUpdatedAt, now)
	} else {
		(*usr.KYCStepsLastUpdatedAt)[int(newKYCStep)-1] = now
	}

	return errors.Wrapf(r.Users.ModifyUser(ctx, usr, nil), "failed to modify user %#v", usr)
}

func (r *repositoryImpl) ContinueQuizSession( //nolint:funlen,revive //.
	ctx context.Context,
	userID UserID,
	question, answer uint8,
) (quiz *Quiz, err error) {
	err = storage.DoInTransaction(ctx, r.DB, func(tx storage.QueryExecer) error {
		now := stdlibtime.Now().Truncate(stdlibtime.Second).UTC()
		progress, pErr := r.CheckUserRunningSession(ctx, userID, now, tx)
		if pErr != nil {
			return wrapErrorInTx(pErr)
		}
		_, err = r.CheckQuestionNumber(ctx, progress.Questions, question, tx)
		if err != nil {
			return wrapErrorInTx(err)
		} else if uint8(len(progress.Answers)) != question-1 {
			return wrapErrorInTx(errors.Wrap(ErrUnknownQuestionNumber, "please answer questions in order"))
		}
		newAnswers, aErr := r.UserAddAnswer(ctx, userID, tx, answer)
		if aErr != nil {
			return wrapErrorInTx(aErr)
		}
		correctNum, incorrectNum := calculateProgress(progress.CorrectAnswers, newAnswers)
		quiz = &Quiz{
			Progress: &Progress{
				MaxQuestions:     uint8(len(progress.Questions)),
				CorrectAnswers:   correctNum,
				IncorrectAnswers: incorrectNum,
			},
		}

		if len(newAnswers) != len(progress.CorrectAnswers) {
			nextQuestion, nErr := r.LoadQuestionByID(ctx, tx, progress.Lang, progress.Questions[question])
			if nErr != nil {
				return wrapErrorInTx(nErr)
			}
			nextQuestion.Number = question + 1
			quiz.Progress.ExpiresAt = time.New(now.Add(stdlibtime.Duration(r.config.MaxSessionDurationSeconds) * stdlibtime.Second))
			quiz.Progress.NextQuestion = nextQuestion

			return nil
		}

		if int(incorrectNum) > r.config.MaxWrongAnswersPerSession {
			quiz.Result = FailureResult
			err = r.UserMarkSessionAsFinished(ctx, userID, now, tx, false, false)
		} else {
			quiz.Result = SuccessResult
			err = r.UserMarkSessionAsFinished(ctx, userID, now, tx, true, false)
		}

		return wrapErrorInTx(err)
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
