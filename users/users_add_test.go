// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/ice-blockchain/wintr/time"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	messagebroker_fixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
)

// nolint:paralleltest // We cannot use parallel tests in case of empty (=random) referral, cuz of it can fetch referredBy-user from another test
// and it cannot be deleted in this case because of reference in DDL.
func TestUserProcessor_CreateUser_Success_NoReferral(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	user := bogusUser("did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376C1", "").
		createUserArg(testClientIP).
		verifyCreateUser(ctx, t, nil)
	verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: user, Before: nil})
	// Additional fields are calculated before save.
	assert.Equal(t, user.DeviceLocation, DeviceLocation{Country: "US", City: "New York City"})
	now := time.Now()
	assert.NotNil(t, user.CreatedAt)
	assert.InDelta(t, now.Unix(), user.CreatedAt.Unix(), 1) // +-1 sec from now, should be enough not to fail in case of sec change during the test.
	assert.NotNil(t, user.UpdatedAt)
	assert.InDelta(t, now.Unix(), user.UpdatedAt.Unix(), 1)
	assert.Greater(t, user.HashCode, uint64(0))
	require.NoError(t, usersProcessor.DeleteUser(ctx, user.ID))
}

func TestUserProcessor_CreateUser_Success_WithReferral(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	t0 := bogusUser("did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B4", "").
		createUserArg(testClientIP).
		verifyCreateUser(ctx, t, nil)
	referralUser := bogusUser("did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B3", t0.ID).
		createUserArg(testClientIP).
		verifyCreateUser(ctx, t, nil)
	verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: referralUser, Before: nil})
	assert.Equal(t, referralUser.ReferredBy, t0.ID)
	require.NoError(t, usersProcessor.DeleteUser(ctx, referralUser.ID))
	require.NoError(t, usersProcessor.DeleteUser(ctx, t0.ID))
}

func TestUserProcessor_CreateUser_Failure_NonExistingReferredBy(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	_ = bogusUser("did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B5", "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B7").
		createUserArg(testClientIP).
		verifyCreateUser(ctx, t, ErrRelationNotFound)
}

// nolint:paralleltest // We cannot use parallel tests in case of empty (=random) referral, cuz of it can fetch referredBy-user from another test
// and it cannot be deleted in this case because of reference in DDL.
func TestUserProcessor_CreateUser_Failure_Duplicate(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	duplicatedUserID := "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B6"
	_ = bogusUser(duplicatedUserID, "").
		createUserArg(testClientIP).
		verifyCreateUser(ctx, t, nil)

	// Duplicated user with same ID.
	_ = bogusUser(duplicatedUserID, "").
		createUserArg(testClientIP).
		verifyCreateUser(ctx, t, ErrDuplicate)
	require.NoError(t, usersProcessor.DeleteUser(ctx, duplicatedUserID))
}

func (arg *CreateUserArg) verifyCreateUser(ctx context.Context, t *testing.T, errorMatcher error) *User {
	t.Helper()
	err := usersProcessor.CreateUser(ctx, arg)
	if errorMatcher != nil {
		require.Error(t, err, errorMatcher)
		assert.True(t, errors.Is(err, errorMatcher))

		return nil
	}
	require.NoError(t, err)
	user := new(User)
	// nolint:forcetypeassert // We have only one implementation for now
	err = usersRepository.(*repository).db.GetTyped("USERS", "pk_unnamed_USERS_1", tarantool.StringKey{S: arg.User.ID}, user)
	require.NoError(t, err)

	return user
}

func verifyUserSnapshotMessages(ctx context.Context, t *testing.T, userSnapshots ...*UserSnapshot) {
	t.Helper()
	for _, userSnapshot := range userSnapshots {
		var id UserID
		if userSnapshot.User != nil {
			id = userSnapshot.User.ID
		} else {
			id = userSnapshot.Before.ID
		}
		message := messagebroker_fixture.RawMessage{
			Key:   id,
			Value: userSnapshot.requireMarshallJSON(t),
			Topic: cfg.MessageBroker.Topics[0].Name, // | users-events.
		}
		require.NoError(t, mbConnector.VerifyMessages(ctx, message))
	}
}

func (u *User) createUserArg(ip net.IP) *CreateUserArg {
	return &CreateUserArg{
		ReferredBy:      u.ReferredBy,
		Username:        u.Username,
		PhoneNumber:     u.PhoneNumber,
		PhoneNumberHash: u.PhoneNumberHash,
		Email:           u.Email,
		User:            *u,
		ClientIP:        ip,
	}
}

func bogusUser(userID UserID, referredBy string) *User {
	return &User{
		PublicUserInformation: PublicUserInformation{
			ID:          userID,
			Username:    "userName:" + userID,
			FirstName:   "FirstName",
			LastName:    "LastName",
			PhoneNumber: "+12345678901",
		},
		Email:           "user@example.com",
		ReferredBy:      referredBy,
		PhoneNumberHash: "10e6f0b47054a83359477dcb35231db6de5c69fb1816e1a6b98e192de9e5b9ee",
	}
}

func (userSnapshot *UserSnapshot) requireMarshallJSON(t *testing.T) string {
	t.Helper()
	valueBytes, err := json.Marshal(userSnapshot)
	require.NoError(t, err)

	return string(valueBytes)
}
