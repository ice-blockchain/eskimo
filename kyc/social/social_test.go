// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/terror"
)

func helperRemoveSocials(t *testing.T, db storage.Execer, userID string) {
	t.Helper()

	_, err := storage.Exec(context.TODO(), db, "DELETE FROM socials where user_id = $1", userID)
	require.NoError(t, err)
}

func TestSocialSave(t *testing.T) {
	const userName = `icenetwork`

	ctx := context.Background()
	usersRepo := users.New(ctx, nil)
	require.NotNil(t, usersRepo)

	db := storage.MustConnect(ctx, ddl, applicationYamlKey)
	repo := &repository{db: db}

	helperRemoveSocials(t, db, userName)

	t.Run("OK", func(t *testing.T) {
		err := repo.saveSocial(ctx, TwitterType, userName, "foo")
		require.NoError(t, err)

		err = repo.saveSocial(ctx, TwitterType, userName, "bar")
		require.NoError(t, err)
	})

	t.Run("Duplicate", func(t *testing.T) {
		err := repo.saveSocial(ctx, TwitterType, userName, "foo")
		require.ErrorIs(t, err, storage.ErrDuplicate)

		reason := detectReason(terror.New(err, map[string]any{"user_handle": "foo"}))
		require.Equal(t, `duplicate userhandle 'foo'`, reason)
	})

	require.NoError(t, db.Close())
}
