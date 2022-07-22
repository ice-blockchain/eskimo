// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nolint:paralleltest,nosnakecase // We're not running this in parallel because it counts users, and users from other tests can affect values
func TestUserProcessor_IncrementCountryUserCount_Success_OnUserCreation(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	userID := "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B8"
	user := bogusUser(userID, "").verifyCreateUser(ctx, t, testClientIP, nil)
	verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: user, Before: nil})
	countryStats, err := usersRepository.GetTopCountries(ctx, "US", 1, 0)
	require.NoError(t, err)
	assert.Equal(t, []*CountryStatistics{{Country: "US", UserCount: 1}}, countryStats)
	require.NoError(t, usersProcessor.DeleteUser(ctx, userID))
}

// nolint:paralleltest,nosnakecase // We're not running this in parallel because it counts users, and users from other tests can affect values
func TestUserProcessor_DecrementCountryUserCount_Success_OnUserDeletion(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	userID := "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B9"
	user := bogusUser(userID, "").verifyCreateUser(ctx, t, testClientIP, nil)

	require.NoError(t, usersProcessor.DeleteUser(ctx, userID))
	verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: nil, Before: user})

	countryStats, err := usersProcessor.GetTopCountries(ctx, "US", 1, 0)
	require.NoError(t, err)
	assert.Equal(t, []*CountryStatistics{{Country: "US", UserCount: 0}}, countryStats)
}
