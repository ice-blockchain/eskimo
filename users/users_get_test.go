// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/go-tarantool-client"
)

func (u *User) bindExisting(ctx context.Context, tb testing.TB, userID string) *User {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	require.NoError(tb, dbConnector.GetTyped("USERS", "pk_unnamed_USERS_3", tarantool.StringKey{S: userID}, u))

	return u
}
