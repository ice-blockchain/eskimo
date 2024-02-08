// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	"mime/multipart"
	"testing"
	stdlibtime "time"

	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

const (
	testQuizMaxAttempts = 3
	testQuizMaxResets   = 2
)

func helperInsertQuestion(t *testing.T, r *repositoryImpl) {
	t.Helper()

	const stmt = `
insert into questions (id, correct_option, options, language, question) values
	(10, 1, '{"Paris",    "Melbourne", "Warsaw", "Guadalajara"}', 'en', 'What is the capital of France?'),
	(20, 2, '{"Kyiv",     "Madrid",    "Milan",  "Schaarzen"}',   'en', 'What is the capital of Spain?'),
	(30, 3, '{"Waalkerk", "İstanbul",  "Berlin", "Wien"}',        'en', 'What is the capital of Germany?'),
	(10, 1, '{"Paris",    "Melbourne", "Warsaw", "Guadalajara"}', 'xx', 'What is the capital of France???'),
	(20, 2, '{"Kyiv",     "Madrid",    "Milan",  "Schaarzen"}',   'xx', 'What is the capital of Spain???'),
	(30, 3, '{"Waalkerk", "İstanbul",  "Berlin", "Wien"}',        'xx', 'What is the capital of Germany???')
on conflict do nothing;
	`

	_, err := storage.Exec(context.Background(), r.DB, stmt)
	require.NoError(t, err)
}

func helperSolveQuestion(t *testing.T, text string) uint8 {
	t.Helper()

	switch text {
	case "What is the capital of France?":
		return 1
	case "What is the capital of Spain?":
		return 2
	case "What is the capital of Germany?":
		return 3
	default:
		t.Errorf("unknown question: %s", text)
	}

	return 0
}

func helperForceFinishSession(t *testing.T, r *repositoryImpl, userID UserID, result bool) {
	t.Helper()

	_, err := storage.Exec(context.TODO(), r.DB, "update quiz_sessions set ended_at = now(), ended_successfully = $2 where user_id = $1", userID, result)
	require.NoError(t, err)

	if result {
		_, err = storage.Exec(context.TODO(), r.DB, `delete from failed_quiz_sessions where user_id = $1`, userID)
		require.NoError(t, err)

		_, err = storage.Exec(context.TODO(), r.DB, `delete from quiz_resets where user_id = $1`, userID)
		require.NoError(t, err)
	}
}

func helperForceResetSessionStartedAt(t *testing.T, r *repositoryImpl, userID UserID) {
	t.Helper()

	_, err := storage.Exec(context.TODO(), r.DB, "update quiz_sessions set ended_at = NULL, started_at = to_timestamp(42) where user_id = $1", userID)
	require.NoError(t, err)
}

func helperUpdateFailedSessionEndedAt(t *testing.T, r *repositoryImpl, userID UserID) {
	t.Helper()

	_, err := storage.Exec(context.TODO(), r.DB, "update failed_quiz_sessions set ended_at = to_timestamp(42) where user_id = $1", userID)
	require.NoError(t, err)

	_, err = storage.Exec(context.TODO(), r.DB, "update quiz_sessions set ended_at = to_timestamp(42) where user_id = $1", userID)
	require.NoError(t, err)
}

func helperSessionReset(t *testing.T, r *repositoryImpl, userID UserID, full bool) {
	t.Helper()

	err := r.ResetQuizSession(context.Background(), userID)
	require.NoError(t, err)

	if full {
		_, err = storage.Exec(context.TODO(), r.DB, "delete from failed_quiz_sessions where user_id = $1", userID)
		require.NoError(t, err)

		_, err = storage.Exec(context.TODO(), r.DB, "delete from failed_quiz_sessions_history where user_id = $1", userID)
		require.NoError(t, err)

		_, err = storage.Exec(context.TODO(), r.DB, `delete from quiz_resets where user_id = $1`, userID)
		require.NoError(t, err)
	}
}

func helperEnsureHistory(t *testing.T, r *repositoryImpl, userID UserID, count uint) {
	t.Helper()

	data, err := storage.Get[int](context.Background(), r.DB, "select count(1) from failed_quiz_sessions_history where user_id = $1", userID)
	require.NoError(t, err)
	require.Equal(t, count, uint(*data), "unexpected history count")
}

type mockUserReader struct {
	OnModifyUser func(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error
}

func (*mockUserReader) GetUserByID(ctx context.Context, userID UserID) (*UserProfile, error) {
	profile := &UserProfile{
		User: &users.User{},
	}

	switch userID {
	case "bogus":
		s := users.Social1KYCStep
		profile.KYCStepPassed = &s

	case "invalid_kyc":
		s := users.LivenessDetectionKYCStep
		profile.KYCStepPassed = &s

	case "storage_error":
		return nil, storage.ErrCheckFailed

	case "unknown_user":
		return nil, storage.ErrNotFound
	}

	return profile, nil
}

func (m *mockUserReader) ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error {
	if m.OnModifyUser != nil {
		return m.OnModifyUser(ctx, usr, profilePicture)
	}

	return nil
}

func testManagerSessionStart(ctx context.Context, t *testing.T, r *repositoryImpl) {
	helperSessionReset(t, r, "bogus", true)

	t.Run("UnknownUser", func(t *testing.T) {
		_, err := r.StartQuizSession(ctx, "unknown_user", "en")
		require.ErrorIs(t, err, ErrUnknownUser)
	})

	t.Run("UnknownLanguage", func(t *testing.T) {
		_, err := r.StartQuizSession(ctx, "bogus", "ff")
		require.ErrorIs(t, err, ErrUnknownLanguage)
	})

	t.Run("InvalidKYCState", func(t *testing.T) {
		_, err := r.StartQuizSession(ctx, "invalid_kyc", "en")
		require.ErrorIs(t, err, ErrInvalidKYCState)
	})

	t.Run("StorageError", func(t *testing.T) {
		_, err := r.StartQuizSession(ctx, "storage_error", "en")
		require.ErrorIs(t, err, storage.ErrCheckFailed)
	})

	t.Run("Sessions", func(t *testing.T) {
		t.Run("OK", func(t *testing.T) {
			session, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)
			require.NotNil(t, session)
			require.NotNil(t, session.Progress)
			require.NotNil(t, session.Progress.ExpiresAt)
			require.NotEmpty(t, session.Progress.NextQuestion)
			require.Equal(t, uint8(3), session.Progress.MaxQuestions)
			require.Equal(t, uint8(1), session.Progress.NextQuestion.Number)
		})
		t.Run("AlreadyExists", func(t *testing.T) {
			_, err := r.StartQuizSession(ctx, "bogus", "en")
			require.ErrorIs(t, err, ErrSessionFinishedWithError)
		})
		t.Run("Finished", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				helperForceFinishSession(t, r, "bogus", true)
				_, err := r.StartQuizSession(ctx, "bogus", "en")
				require.ErrorIs(t, err, ErrSessionFinished)
			})
			t.Run("Error", func(t *testing.T) {
				helperForceFinishSession(t, r, "bogus", false)
				_, err := r.StartQuizSession(ctx, "bogus", "en")
				require.ErrorIs(t, err, ErrSessionFinishedWithError)
			})
			t.Run("CoolDown", func(t *testing.T) {
				helperForceFinishSession(t, r, "bogus", false)
				_, err := storage.Exec(ctx, r.DB, "update quiz_sessions set started_at = to_timestamp(40), ended_at = to_timestamp(42), ended_successfully = false where user_id = $1", "bogus")
				require.NoError(t, err)
				_, err = r.StartQuizSession(ctx, "bogus", "en")
				require.NoError(t, err)
			})
		})
		t.Run("Expired", func(t *testing.T) {
			helperForceResetSessionStartedAt(t, r, "bogus")
			session, err := r.StartQuizSession(ctx, "bogus", "en")
			require.ErrorIs(t, err, ErrSessionFinishedWithError)
			require.Nil(t, session)
		})
		t.Run("CoolDown", func(t *testing.T) {
			helperSessionReset(t, r, "bogus", true)
			session, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)

			for i := uint8(0); i < uint8(r.config.MaxWrongAnswersPerSession+1); i++ {
				session, err = r.ContinueQuizSession(ctx, "bogus", i+uint8(1), 0)
				require.NoError(t, err)
				require.NotNil(t, session)
			}
			require.Equal(t, FailureResult, session.Result)

			_, err = r.StartQuizSession(ctx, "bogus", "en")
			require.ErrorIs(t, err, ErrSessionFinishedWithError)
		})
	})

	t.Run("MaxAttempts", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		for i := uint8(0); i < uint8(r.config.MaxAttemptsAllowed); i++ {
			_, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)

			err = r.SkipQuizSession(ctx, "bogus")
			require.NoError(t, err)

			helperUpdateFailedSessionEndedAt(t, r, "bogus")
		}

		helperEnsureHistory(t, r, "bogus", uint(r.config.MaxAttemptsAllowed))
	})

	t.Run("ResetAttempts", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		for reset := uint8(1); reset <= *r.config.MaxResetCount+1; reset++ {
			for i := uint8(0); i < uint8(r.config.MaxAttemptsAllowed); i++ {
				_, err := r.StartQuizSession(ctx, "bogus", "en")
				require.NoError(t, err)

				err = r.SkipQuizSession(ctx, "bogus")
				require.NoError(t, err)

				helperUpdateFailedSessionEndedAt(t, r, "bogus")
			}
			helperEnsureHistory(t, r, "bogus", uint(r.config.MaxAttemptsAllowed*reset))
		}

		_, err := r.StartQuizSession(ctx, "bogus", "en")
		require.ErrorIs(t, err, ErrNotAvailable)
	})
}

func testManagerSessionSkip(ctx context.Context, t *testing.T, r *repositoryImpl) {
	t.Run("OK", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		_, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)

		err = r.SkipQuizSession(ctx, "bogus")
		require.NoError(t, err)

		_, err = r.StartQuizSession(ctx, "bogus", "en")
		require.ErrorIs(t, err, ErrSessionFinishedWithError)
	})
	t.Run("UnknownSession", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		err := r.SkipQuizSession(ctx, "bogus")
		require.NoError(t, err)
	})
	t.Run("Expired", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		_, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)

		helperForceResetSessionStartedAt(t, r, "bogus")

		err = r.SkipQuizSession(ctx, "bogus")
		require.NoError(t, err)
	})
	t.Run("Finished", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			helperSessionReset(t, r, "bogus", true)

			_, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)

			helperForceFinishSession(t, r, "bogus", true)

			err = r.SkipQuizSession(ctx, "bogus")
			require.ErrorIs(t, err, ErrSessionFinished)
		})
		t.Run("Error", func(t *testing.T) {
			helperSessionReset(t, r, "bogus", true)

			_, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)

			err = r.SkipQuizSession(ctx, "bogus")
			require.NoError(t, err)

			err = r.SkipQuizSession(ctx, "bogus")
			require.NoError(t, err)
		})
	})
}

func testManagerSessionContinueErrors(ctx context.Context, t *testing.T, r *repositoryImpl) {
	helperSessionReset(t, r, "bogus", true)

	t.Run("UnknownSession", func(t *testing.T) {
		_, err := r.ContinueQuizSession(ctx, "unknown_user", 1, 1)
		require.ErrorIs(t, err, ErrUnknownSession)
	})

	t.Run("Finished", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			helperSessionReset(t, r, "bogus", true)
			data, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)
			helperForceFinishSession(t, r, "bogus", true)
			_, err = r.ContinueQuizSession(ctx, "bogus", data.Progress.NextQuestion.Number, 1)
			require.ErrorIs(t, err, ErrSessionFinished)
		})
		t.Run("Error", func(t *testing.T) {
			helperSessionReset(t, r, "bogus", true)
			data, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)
			helperForceFinishSession(t, r, "bogus", false)
			_, err = r.ContinueQuizSession(ctx, "bogus", data.Progress.NextQuestion.Number, 1)
			require.ErrorIs(t, err, ErrSessionFinishedWithError)
		})
		helperSessionReset(t, r, "bogus", true)
	})

	t.Run("Expired", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)
		data, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		helperForceResetSessionStartedAt(t, r, "bogus")
		data, err = r.ContinueQuizSession(ctx, "bogus", data.Progress.NextQuestion.Number, 1)
		require.NoError(t, err)
		require.Nil(t, data.Progress)
		require.Equal(t, FailureResult, data.Result)

		_, err = r.StartQuizSession(ctx, "bogus", "en")
		require.ErrorIs(t, err, ErrSessionFinishedWithError)
	})

	t.Run("UnknownQuestionNumber", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)
		_, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		for _, n := range []uint8{0, 4, 5, 6, 7, 10, 20, 100} {
			_, err = r.ContinueQuizSession(ctx, "bogus", n, 1)
			require.ErrorIs(t, err, ErrUnknownQuestionNumber)
		}
	})

	t.Run("AnswersOrder", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)
		data, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		_, err = r.ContinueQuizSession(ctx, "bogus", 1, helperSolveQuestion(t, data.Progress.NextQuestion.Text))
		require.NoError(t, err)
		// Skip 2nd question.
		_, err = r.ContinueQuizSession(ctx, "bogus", 3, 1)
		require.ErrorIs(t, err, ErrUnknownQuestionNumber)
	})
}

func testManagerSessionContinueWithCorrectAnswers(ctx context.Context, t *testing.T, r *repositoryImpl) {
	helperSessionReset(t, r, "bogus", true)

	session, err := r.StartQuizSession(ctx, "bogus", "en")
	require.NoError(t, err)
	require.NotNil(t, session)
	t.Logf("next q: %v, deadline %v", session.Progress.NextQuestion.Text, session.Progress.ExpiresAt)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.NotEmpty(t, session.Progress.NextQuestion)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)

	ans := helperSolveQuestion(t, session.Progress.NextQuestion.Text)
	t.Logf("q: %v, ans: %d", session.Progress.NextQuestion.Text, ans)
	session, err = r.ContinueQuizSession(ctx, "bogus", session.Progress.NextQuestion.Number, ans)
	require.NoError(t, err)
	require.NotNil(t, session)
	t.Logf("next q: %v, deadline %v", session.Progress.NextQuestion.Text, session.Progress.ExpiresAt)
	require.Empty(t, session.Result)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(1), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(0), session.Progress.IncorrectAnswers)

	nextQNum := session.Progress.NextQuestion.Number
	ans = helperSolveQuestion(t, session.Progress.NextQuestion.Text)
	t.Logf("q: %v, ans: %d", session.Progress.NextQuestion.Text, ans)
	session, err = r.ContinueQuizSession(ctx, "bogus", nextQNum, ans)
	require.NoError(t, err)
	require.NotNil(t, session)
	t.Logf("next q: %v, deadline %v", session.Progress.NextQuestion.Text, session.Progress.ExpiresAt)
	require.Empty(t, session.Result)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(2), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(0), session.Progress.IncorrectAnswers)

	ans = helperSolveQuestion(t, session.Progress.NextQuestion.Text)
	t.Logf("q: %v, ans: %d", session.Progress.NextQuestion.Text, ans)
	session, err = r.ContinueQuizSession(ctx, "bogus", nextQNum, ans)
	require.NoError(t, err)
	require.NotNil(t, session)
	t.Logf("next q: %v, deadline %v", session.Progress.NextQuestion.Text, session.Progress.ExpiresAt)
	require.Empty(t, session.Result)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(2), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(0), session.Progress.IncorrectAnswers)

	ans = helperSolveQuestion(t, session.Progress.NextQuestion.Text)
	t.Logf("q: %v, ans: %d", session.Progress.NextQuestion.Text, ans)
	session, err = r.ContinueQuizSession(ctx, "bogus", session.Progress.NextQuestion.Number, ans)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, SuccessResult, session.Result)
	require.NotNil(t, session.Progress)
	require.Nil(t, session.Progress.NextQuestion)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.Equal(t, uint8(3), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(0), session.Progress.IncorrectAnswers)
}

func testManagerSessionContinueWithIncorrectAnswers(ctx context.Context, t *testing.T, r *repositoryImpl) {
	helperSessionReset(t, r, "bogus", true)

	t.Logf("max incorrect answers: %d", r.config.MaxWrongAnswersPerSession)

	session, err := r.StartQuizSession(ctx, "bogus", "en")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.NotEmpty(t, session.Progress.NextQuestion)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(1), session.Progress.NextQuestion.Number)

	session, err = r.ContinueQuizSession(ctx, "bogus", session.Progress.NextQuestion.Number, 0)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, session.Progress)
	require.Empty(t, session.Result)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.NotEmpty(t, session.Progress.NextQuestion)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(2), session.Progress.NextQuestion.Number)

	session, err = r.ContinueQuizSession(ctx, "bogus", session.Progress.NextQuestion.Number, 0)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, FailureResult, session.Result)
	require.NotNil(t, session.Progress)
	require.Nil(t, session.Progress.NextQuestion)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.Equal(t, uint8(0), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(2), session.Progress.IncorrectAnswers)
}

func testManagerSessionStatus(ctx context.Context, t *testing.T, r *repositoryImpl) {
	t.Run("Check", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		session, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		require.NotNil(t, session)

		status, err := r.getQuizStatus(ctx, "bogus")
		require.NoError(t, err)
		require.NotNil(t, status)

		t.Logf("status: %#v", status)
		require.True(t, status.HasUnfinishedSessions)
		require.True(t, status.KYCQuizAvailabilityEndedAt.After(stdlibtime.Now()))
		require.Equal(t, uint8(testQuizMaxAttempts), status.KYCQuizRemainingAttempts)
		require.False(t, status.KYCQuizDisabled)
		require.False(t, status.KYCQuizCompleted)
	})

	t.Run("UnknownUserOrSession", func(t *testing.T) {
		_, err := r.getQuizStatus(ctx, "unknown_user")
		require.ErrorIs(t, err, ErrUnknownSession)
	})

	t.Run("CheckWithFinish", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		session, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		require.NotNil(t, session)

		// Check + reset.
		status, err := r.CheckQuizStatus(ctx, "bogus")
		require.NoError(t, err)
		require.NotNil(t, status)
		require.False(t, status.HasUnfinishedSessions)

		// Just check.
		status, err = r.getQuizStatus(ctx, "bogus")
		require.NoError(t, err)
		require.NotNil(t, session)
		t.Logf("status: %#v", status)
		require.False(t, status.HasUnfinishedSessions)
		require.True(t, status.KYCQuizAvailabilityEndedAt.After(stdlibtime.Now()))
		require.Equal(t, uint8(testQuizMaxAttempts-1), status.KYCQuizRemainingAttempts)
		require.False(t, status.KYCQuizDisabled)
		require.False(t, status.KYCQuizCompleted)
	})

	t.Run("PrepareForReset", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		session, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		require.NotNil(t, session)

		configCopy := r.config
		r.config.globalStartDate = time.New(stdlibtime.Now().Add(-stdlibtime.Hour * 100))
		r.config.AvailabilityWindowSeconds = 1

		reader := r.Users.(*mockUserReader)
		require.NotNil(t, reader)
		callNum := 0
		reader.OnModifyUser = func(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error {
			t.Logf("user motify key blocked: %#v", usr.KYCStepBlocked)

			if callNum == 0 {
				require.Nil(t, usr.KYCStepBlocked)
			} else if callNum == 1 {
				require.NotNil(t, usr.KYCStepBlocked)
				require.Equal(t, users.QuizKYCStep, *usr.KYCStepBlocked)
			} else {
				require.FailNow(t, "unexpected call num")
			}

			callNum++

			return nil
		}

		status, err := r.CheckQuizStatus(ctx, "bogus")
		require.NoError(t, err)

		t.Logf("status: %#v", status)

		require.False(t, status.HasUnfinishedSessions)
		require.False(t, status.KYCQuizDisabled)
		require.False(t, status.KYCQuizCompleted)

		require.NotNil(t, status.KYCQuizAvailabilityEndedAt)
		require.Len(t, status.KYCQuizResetAt, 1)
		require.True(t, status.KYCQuizResetAt[0].Before(stdlibtime.Now()))
		require.Equal(t, uint8(testQuizMaxAttempts), status.KYCQuizRemainingAttempts)

		reader.OnModifyUser = nil
		r.config = configCopy
	})
}

func TestSessionManager(t *testing.T) {
	t.Parallel()

	ctx := context.TODO()

	// Create user repo because we need its schema.
	usersRepo := users.New(ctx, nil)
	require.NotNil(t, usersRepo)

	repo := newRepositoryImpl(ctx, new(mockUserReader))
	require.NotNil(t, repo)

	cnt := uint8(testQuizMaxResets)
	repo.config.MaxAttemptsAllowed = testQuizMaxAttempts
	repo.config.MaxResetCount = &cnt
	repo.config.globalStartDate = time.New(time.Now().Add(-stdlibtime.Hour))
	repo.config.AvailabilityWindowSeconds = 60 * 60 * 24 * 7 // 1 week.

	helperInsertQuestion(t, repo)

	t.Run("Start", func(t *testing.T) {
		testManagerSessionStart(ctx, t, repo)
	})
	t.Run("Continue", func(t *testing.T) {
		testManagerSessionContinueErrors(ctx, t, repo)
		testManagerSessionContinueWithCorrectAnswers(ctx, t, repo)
		testManagerSessionContinueWithIncorrectAnswers(ctx, t, repo)
	})

	t.Run("Skip", func(t *testing.T) {
		testManagerSessionSkip(ctx, t, repo)
	})

	t.Run("Status", func(t *testing.T) {
		testManagerSessionStatus(ctx, t, repo)
	})

	require.NoError(t, repo.Close())
}
