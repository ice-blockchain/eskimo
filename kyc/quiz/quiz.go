// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	"fmt"
	"strconv"
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

func NewRepository(ctx context.Context, userReader UserReader) Repository {
	return newRepositoryImpl(ctx, userReader)
}

func newRepositoryImpl(ctx context.Context, userReader UserReader) *repositoryImpl {
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &repositoryImpl{
		DB:       db,
		Shutdown: db.Close,
		Users:    userReader,
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

	if profile.KYCStepPassed == nil || *profile.KYCStepPassed != (users.QuizKYCStep-1) {
		state := "not set"
		if profile.KYCStepPassed != nil {
			state = strconv.Itoa(int(*profile.KYCStepPassed))
		}

		return errors.Wrap(ErrInvalidKYCState, state)
	}

	return nil
}

func (r *repositoryImpl) CheckUserFailedSession(ctx context.Context, userID UserID, now stdlibtime.Time, tx storage.QueryExecer) error {
	type failedSession struct {
		EndedAt stdlibtime.Time `db:"ended_at"`
	}

	const stmt = `
select max(ended_at) as ended_at from failed_quizz_sessions where user_id = $1 having max(ended_at) > $2
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
		StartedAt         stdlibtime.Time `db:"started_at"`
		Finished          bool            `db:"finished"`
		FinishedSuccfully bool            `db:"ended_successfully"`
	}
	const stmt = `select started_at, ended_at is not null as finished, ended_successfully from quizz_sessions where user_id = $1`

	data, err := storage.Get[userSession](ctx, tx, stmt, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}

		return errors.Wrap(err, "failed to get active session data")
	}

	if data.Finished {
		if data.FinishedSuccfully {
			return ErrSessionFinished
		}

		return ErrSessionFinishedWithError
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
		questions[i].Number = uint(i + 1)
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
insert into quizz_sessions (user_id, language, questions, started_at, answers) values ($1, $2, $3, $4, '{}'::smallint[])
	on conflict on constraint quizz_sessions_pkey do update
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

func (r *repositoryImpl) StartQuizSession(ctx context.Context, userID UserID, lang string) (quiz *Quiz, err error) { //nolint:funlen //.
	fnCheck := []func(context.Context, UserID, stdlibtime.Time, storage.QueryExecer) error{
		r.CheckUserFailedSession,
		r.CheckUserActiveSession,
	}

	err = r.CheckUserKYC(ctx, userID)
	if err != nil {
		return nil, err
	}

	err = storage.DoInTransaction(ctx, r.DB, func(tx storage.QueryExecer) error {
		now := stdlibtime.Now().Truncate(stdlibtime.Second).UTC()
		for _, fn := range fnCheck {
			if err = fn(ctx, userID, now, tx); err != nil {
				return wrapErrorInTx(err)
			}
		}

		questions, qErr := r.SelectQuestions(ctx, tx, lang)
		if qErr != nil {
			return wrapErrorInTx(qErr)
		}

		err = r.CreateSessionEntry(ctx, userID, lang, questions, now, tx)
		if err != nil {
			return wrapErrorInTx(err)
		}

		quiz = &Quiz{
			Progress: &Progress{
				ExpiresAt:    time.New(now.Add(stdlibtime.Duration(r.config.MaxSessionDurationSeconds) * stdlibtime.Second)),
				NextQuestion: questions[0],
				MaxQuestions: uint8(len(questions)),
			},
		}

		return nil
	})

	return quiz, err
}

func calculateProgress(correctAnswers, currentAnswers []uint) (correctNum, incorrectNum uint8) {
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
		Finished          bool `db:"finished"`
		FinishedSuccfully bool `db:"ended_successfully"`
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
	quizz_sessions session,
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
		if data.FinishedSuccfully {
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

func (*repositoryImpl) CheckQuestionNumber(ctx context.Context, questions []uint, num uint, tx storage.QueryExecer) (uint, error) {
	type currentQuestion struct {
		CorrectOption uint8 `db:"correct_option"`
	}

	if num == 0 || num > uint(len(questions)) {
		return 0, ErrUnknownQuestionNumber
	}

	data, err := storage.Get[currentQuestion](ctx, tx, `select correct_option from questions where id = $1`, questions[num-1])
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return 0, ErrUnknownQuestionNumber
		}

		return 0, errors.Wrap(err, "failed to get current question data")
	}

	return uint(data.CorrectOption), nil
}

func (*repositoryImpl) UserAddAnswer(ctx context.Context, userID UserID, tx storage.QueryExecer, answer uint8) ([]uint, error) {
	const stmt = `
update quizz_sessions
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

func (*repositoryImpl) LoadQuestionByID(ctx context.Context, tx storage.QueryExecer, lang string, questionID uint) (*Question, error) {
	const stmt = `
select id, options, question from questions where "language" = $1 and id = $2
	`

	question, err := storage.Get[Question](ctx, tx, stmt, lang, questionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to select questions")
	}

	return question, nil
}

func (*repositoryImpl) UserMarkSessionAsFinished(ctx context.Context, userID UserID, now stdlibtime.Time, tx storage.QueryExecer, successful bool) error {
	const stmt = `
with result as (
	update quizz_sessions
	set
		ended_at = $3,
		ended_successfully = $2
	where
		user_id = $1
	returning *
)
insert into failed_quizz_sessions (started_at, ended_at, questions, answers, language, user_id)
select
	result.started_at,
	result.ended_at,
	result.questions,
	result.answers,
	result.language,
	result.user_id
from result
where
	result.ended_successfully = false
	`

	if _, err := storage.Exec(ctx, tx, stmt, userID, successful, now); err != nil {
		return errors.Wrap(err, "failed to mark session as finished")
	}

	return nil
}

func (r *repositoryImpl) ContinueQuizSession( //nolint:funlen,revive //.
	ctx context.Context,
	userID UserID,
	question uint,
	answer uint8,
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
		} else if uint(len(progress.Answers)) != question-1 {
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
			err = r.UserMarkSessionAsFinished(ctx, userID, now, tx, false)
		} else {
			quiz.Result = SuccessResult
			err = r.UserMarkSessionAsFinished(ctx, userID, now, tx, true)
		}

		return wrapErrorInTx(err)
	})

	return quiz, err
}

func (r *repositoryImpl) ResetQuizSession(ctx context.Context, userID UserID) error {
	// $1: user_id.
	const stmt = `
		delete from quizz_sessions
		where
			user_id = $1
	`
	_, err := storage.Exec(ctx, r.DB, stmt, userID)

	return errors.Wrap(err, "failed to reset session")
}
