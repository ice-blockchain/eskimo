// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
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
}

func helperForceResetSessionStartedAt(t *testing.T, r *repositoryImpl, userID UserID) {
	t.Helper()

	_, err := storage.Exec(context.TODO(), r.DB, "update quiz_sessions set ended_at = NULL, started_at = to_timestamp(42) where user_id = $1", userID)
	require.NoError(t, err)
}

func helperSessionReset(t *testing.T, r *repositoryImpl, userID UserID, full bool) {
	t.Helper()

	err := r.ResetQuizSession(context.Background(), userID)
	require.NoError(t, err)

	if full {
		_, err = storage.Exec(context.TODO(), r.DB, "delete from failed_quiz_sessions where user_id = $1", userID)
		require.NoError(t, err)
	}
}

type mockUserReader struct{}

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

func (*mockUserReader) ModifyUser(ctx context.Context, usr *users.User, profilePicture *multipart.FileHeader) error {
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
			require.ErrorIs(t, err, ErrSessionIsAlreadyRunning)
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
			require.NoError(t, err)
			require.NotNil(t, session)
			require.NotNil(t, session.Progress)
			require.NotNil(t, session.Progress.ExpiresAt)
			require.NotEmpty(t, session.Progress.NextQuestion)
			require.Equal(t, uint8(3), session.Progress.MaxQuestions)
			require.Equal(t, uint8(1), session.Progress.NextQuestion.Number)
		})
		t.Run("CoolDown", func(t *testing.T) {
			helperSessionReset(t, r, "bogus", true)
			session, err := r.StartQuizSession(ctx, "bogus", "en")
			require.NoError(t, err)

			for i := uint8(0); i < uint8(session.Progress.MaxQuestions); i++ {
				session, err = r.ContinueQuizSession(ctx, "bogus", i+uint8(1), 0)
				require.NoError(t, err)
				require.NotNil(t, session)
			}
			require.Equal(t, FailureResult, session.Result)

			_, err = r.StartQuizSession(ctx, "bogus", "en")
			require.ErrorIs(t, err, ErrSessionFinishedWithError)
		})
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
		require.ErrorIs(t, err, ErrUnknownSession)
	})
	t.Run("Expired", func(t *testing.T) {
		helperSessionReset(t, r, "bogus", true)

		_, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)

		helperForceResetSessionStartedAt(t, r, "bogus")

		err = r.SkipQuizSession(ctx, "bogus")
		require.ErrorIs(t, err, ErrSessionExpired)
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
			require.ErrorIs(t, err, ErrSessionFinishedWithError)
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
		_, err = r.ContinueQuizSession(ctx, "bogus", data.Progress.NextQuestion.Number, 1)
		require.ErrorIs(t, err, ErrSessionExpired)
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
		_, err := r.StartQuizSession(ctx, "bogus", "en")
		require.NoError(t, err)
		_, err = r.ContinueQuizSession(ctx, "bogus", 1, 1)
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

	ans = helperSolveQuestion(t, session.Progress.NextQuestion.Text)
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
	require.Empty(t, session.Result)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(0), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(1), session.Progress.IncorrectAnswers)
	require.Equal(t, uint8(2), session.Progress.NextQuestion.Number)

	ans := helperSolveQuestion(t, session.Progress.NextQuestion.Text)
	t.Logf("q: %v, ans: %d", session.Progress.NextQuestion.Text, ans)
	session, err = r.ContinueQuizSession(ctx, "bogus", session.Progress.NextQuestion.Number, ans)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Empty(t, session.Result)
	require.NotNil(t, session.Progress)
	require.NotNil(t, session.Progress.ExpiresAt)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.NotEmpty(t, session.Progress.NextQuestion.Text)
	require.Equal(t, uint8(1), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(1), session.Progress.IncorrectAnswers)
	require.Equal(t, uint8(3), session.Progress.NextQuestion.Number)

	session, err = r.ContinueQuizSession(ctx, "bogus", session.Progress.NextQuestion.Number, 0)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, FailureResult, session.Result)
	require.NotNil(t, session.Progress)
	require.Nil(t, session.Progress.NextQuestion)
	require.Equal(t, uint8(3), session.Progress.MaxQuestions)
	require.Equal(t, uint8(1), session.Progress.CorrectAnswers)
	require.Equal(t, uint8(2), session.Progress.IncorrectAnswers)
}

func TestSessionManager(t *testing.T) {
	t.Parallel()

	ctx := context.TODO()

	// Create user repo because we need its schema.
	usersRepo := users.New(ctx, nil)
	require.NotNil(t, usersRepo)

	repo := newRepositoryImpl(ctx, new(mockUserReader))
	require.NotNil(t, repo)

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

	require.NoError(t, repo.Close())
}
