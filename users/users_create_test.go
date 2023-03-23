// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/terror"
	. "github.com/ice-blockchain/wintr/testing"
	"github.com/ice-blockchain/wintr/time"
)

func TestRepository_CreateUser_Success_FirstUser(t *testing.T) { //nolint:paralleltest // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	GIVEN("we have nothing in the database", func() {})
	var (
		firstUser = new(User).completelyRandomizeForCreate()
		err       error
	)
	WHEN("creating the first user", func() {
		err = firstUser.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is successfully created", func() {
			require.NoError(t, err)
		})
		IT("refers to itself", func() {
			assert.Equal(t, firstUser.ID, firstUser.ReferredBy)
		})
		IT("generated a new created user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: firstUser, Before: nil})
		})
		IT("incremented `users_per_country` by 1 for its country(default one); resulting in 1 because its the first user", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
		})
	})
}

func TestRepository_CreateUser_Success_RandomReferral(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	GIVEN("we have some users in the database", func() {
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	GIVEN("we have some users from the default country", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 3)
	})
	var (
		usr = new(User).completelyRandomizeForCreate()
		err error
	)
	WHEN("creating the user", func() {
		err = usr.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is successfully created", func() {
			require.NoError(t, err)
		})
		IT("does not refer to itself, but to some other random one", func() {
			assert.NotEqual(t, usr.ID, usr.ReferredBy)
			assert.NotEmpty(t, usr.ReferredBy)
		})
		IT("generated a new created user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: nil})
		})
		IT("incremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 4)
		})
	})
}

func TestRepository_CreateUser_Success_DuplicatedEmail(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	var existingUsr *User
	GIVEN("we have some users in the database", func() {
		existingUsr = new(User)
		require.NoError(t, existingUsr.completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	GIVEN("we have some users from the default country", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 3)
	})
	var (
		usr = new(User).completelyRandomizeForCreate()
		err error
	)
	WHEN("creating the user with an existing email", func() {
		usr.Email = existingUsr.Email
		err = usr.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is successfully created", func() {
			require.NoError(t, err)
		})
		IT("has the exact same email as some other user", func() {
			assert.Equal(t, existingUsr.Email, new(User).bindExisting(ctx, t, usr.ID).Email)
		})
		IT("generated a new created user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: nil})
		})
		IT("incremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 4)
		})
	})
}

func TestRepository_CreateUser_Success_DuplicatedPhoneNumber(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	var existingUsr *User
	GIVEN("we have some users in the database", func() {
		existingUsr = new(User)
		require.NoError(t, existingUsr.completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	GIVEN("we have some users from the default country", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 3)
	})
	var (
		usr = new(User).completelyRandomizeForCreate()
		err error
	)
	WHEN("creating the user with an existing phoneNumber", func() {
		usr.PhoneNumber = existingUsr.PhoneNumber
		usr.PhoneNumberHash = existingUsr.PhoneNumberHash
		err = usr.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is successfully created", func() {
			require.NoError(t, err)
		})
		IT("has the exact same email as some other user", func() {
			created := new(User).bindExisting(ctx, t, usr.ID)
			assert.Equal(t, existingUsr.PhoneNumber, created.PhoneNumber)
			assert.Equal(t, existingUsr.PhoneNumberHash, created.PhoneNumberHash)
		})
		IT("generated a new created user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: nil})
		})
		IT("incremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 4)
		})
	})
}

func TestRepository_CreateUser_Success_WithEverythingSet(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	referral := new(User)
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	GIVEN("we have some users in the database", func() {
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, referral.completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	GIVEN("we have some users from the default country and none from `CH`", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 3)
		assertUsersPerCountry(ctx, t, "CH", 0)
	})
	var (
		usr = new(User).randomizeForCreateWithReferredBy(referral.ID)
		err error
	)
	WHEN("creating the user with specified `referredBy` and a custom clientIP", func() {
		err = usr.mustCreate(ctx, t, "9.9.9.11")
	})
	THEN(func() {
		IT("is successfully created", func() {
			require.NoError(t, err)
		})
		IT("has the correct geolocation", func() {
			require.Equal(t, "CH", usr.Country)
			require.Equal(t, "Zurich", usr.City)
		})
		IT("refers to the expected referral", func() {
			assert.Equal(t, referral.ID, usr.ReferredBy)
		})
		IT("generated a new created user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: nil})
		})
		IT("incremented `users_per_country` by 1 for its country and didnt change the one for the default country", func() {
			assertUsersPerCountry(ctx, t, "CH", 1)
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 3)
		})
	})
}

func TestRepository_CreateUser_Success_MissingRandomReferral(t *testing.T) {
	t.Parallel()
	//nolint:godox // .
	//TODO: If we have issues with a lot of referrals getting deleted while creating users that point to those referrals,
	// then we should invest in writing this test.
	assert.True(t, true)
}

func TestRepository_CreateUser_Failure_DuplicateUserID(t *testing.T) { //nolint:funlen // Slightly bigger testcase; It's fine.
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	someExistingUser := new(User)
	GIVEN("we have some users in the database", func() {
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		u := new(User)
		require.NoError(t, u.completelyRandomizeForCreate().mustCreate(ctx, t))
		someExistingUser.randomizeForCreateWithReferredBy(u.ID)
		someExistingUser.CreatedAt = time.Now()
		someExistingUser.UpdatedAt = someExistingUser.CreatedAt
		someExistingUser.DeviceLocation = DeviceLocation{Country: defaultClientIPCountry, City: defaultClientIPCity}
		someExistingUser.HashCode = xxh3.HashStringSeed(someExistingUser.ID, uint64(someExistingUser.CreatedAt.UnixNano()))
		someExistingUser.ProfilePictureURL = defaultUserImage
		require.NoError(t, dbConnector.InsertTyped("USERS", someExistingUser, &[]*User{}))
	})
	var (
		usr = new(User).completelyRandomizeForCreate()
		err error
	)
	WHEN("creating the user with an ID that already exists", func() {
		usr.ID = someExistingUser.ID
		err = usr.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is is not created", func() {
			require.NotNil(t, err)
		})
		IT("returns specific error", func() {
			require.ErrorIs(t, err, storage.ErrDuplicate)
			require.NotNil(t, terror.As(err))
			assert.EqualValues(t, map[string]any{"field": "id"}, terror.As(err).Data)
		})
		IT("did not generate any new user snapshot messages", func() {
			verifyNoUserSnapshotMessages(ctx, t, ANY, usr.ID)
		})
	})
}

func TestRepository_CreateUser_Failure_DuplicateUserName(t *testing.T) { //nolint:funlen // Slightly bigger testcase; It's fine.
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	someExistingUser := new(User)
	GIVEN("we have some users in the database", func() {
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, someExistingUser.completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	var (
		usr = new(User).completelyRandomizeForCreate()
		err error
	)
	WHEN("creating the user with an username that already exists", func() {
		usr.Username = someExistingUser.Username
		err = usr.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is is not created", func() {
			require.NotNil(t, err)
		})
		IT("returns specific error", func() {
			require.ErrorIs(t, err, storage.ErrDuplicate)
			require.NotNil(t, terror.As(err))
			assert.EqualValues(t, map[string]any{"field": "username"}, terror.As(err).Data)
		})
		IT("did not generate any new user snapshot messages", func() {
			verifyNoUserSnapshotMessages(ctx, t, ANY, usr.ID)
		})
	})
}

func TestRepository_CreateUser_Failure_NonExistingReferral(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	GIVEN("we have some users in the database", func() {
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, new(User).completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	var (
		usr = new(User).randomizeForCreateWithReferredBy("bogusUserID")
		err error
	)
	WHEN("creating the user with a non existing/bogus `referredBy` specified", func() {
		err = usr.mustCreate(ctx, t)
	})
	THEN(func() {
		IT("is is not created", func() {
			require.NotNil(t, err)
		})
		IT("returns specific error", func() {
			require.ErrorIs(t, err, storage.ErrRelationNotFound)
		})
		IT("did not generate any new user snapshot messages", func() {
			verifyNoUserSnapshotMessages(ctx, t, ANY, usr.ID)
		})
	})
}

func (u *User) mustCreate(ctx context.Context, tb testing.TB, clientIPs ...string) (err error) { //nolint:funlen // A lot of stuff to check.
	tb.Helper()
	require.NoError(tb, ctx.Err())
	clientIP := defaultClientIP
	if len(clientIPs) == 1 {
		clientIP = net.ParseIP(clientIPs[0])
	}
	before := new(User).bindExisting(ctx, tb, u.ID)

	err = usersRepository.CreateUser(ctx, u, clientIP)

	after := new(User).bindExisting(ctx, tb, u.ID)

	if err == nil {
		assert.Empty(tb, *before)
		expected := *u
		expected.ProfilePictureURL = after.ProfilePictureURL
		assert.Nil(tb, after.LastMiningStartedAt)
		assert.Nil(tb, after.LastMiningEndedAt)
		assert.Nil(tb, after.LastPingCooldownEndedAt)
		assert.NotNil(tb, after.CreatedAt)
		assert.NotNil(tb, after.UpdatedAt)
		assert.EqualValues(tb, after.CreatedAt, after.UpdatedAt)
		assert.InDelta(tb, after.CreatedAt.Unix(), time.Now().Unix(), 2)
		assert.NotEmpty(tb, after.ReferredBy)
		assert.NotEmpty(tb, after.City)
		assert.NotEmpty(tb, after.Country)
		if len(clientIPs) != 1 {
			require.Equal(tb, defaultClientIPCountry, after.Country)
			require.Equal(tb, defaultClientIPCity, after.City)
		}
		assert.Equal(tb, xxh3.HashStringSeed(after.ID, uint64(after.CreatedAt.UnixNano())), after.HashCode)
		assert.Equal(tb, defaultUserImage, after.ProfilePictureURL)
		assert.Equal(tb, fmt.Sprintf("%v/%v", cfg.PictureStorage.URLDownload, after.ProfilePictureURL), u.ProfilePictureURL)
		assert.EqualValues(tb, &expected, after)
	} else {
		assert.EqualValues(tb, before, after)
	}
	if before.ID == "" && after.ID == "" {
		assert.NotNil(tb, err)
		assert.EqualValues(tb, before, after)
	}

	return err //nolint:wrapcheck // That's what we intend, to proxy it as-is.
}

func (u *User) completelyRandomizeForCreate() *User {
	return u.randomizeForCreateWithReferredBy("")
}

func (u *User) randomizeForCreateWithReferredBy(referredBy string) *User {
	return u.randomizeForCreate(uuid.NewString(), referredBy, uuid.NewString(), uuid.NewString(), uuid.NewString()+"@"+uuid.NewString()+".io")
}

func (u *User) randomizeForCreate(userID, referredBy, username, phoneNumber, email string) *User {
	*u = User{
		PublicUserInformation: PublicUserInformation{
			ID:          userID,
			Username:    username,
			FirstName:   fmt.Sprintf("FirstName-%v", uuid.NewString()),
			LastName:    fmt.Sprintf("LastName-%v", uuid.NewString()),
			PhoneNumber: phoneNumber,
			DeviceLocation: DeviceLocation{ // This is not needed, cuz it's overridden by CreateUser, but is a nice to have when used for other APIs.
				Country: defaultClientIPCountry,
				City:    defaultClientIPCity,
			},
		},
		Email:           email,
		ReferredBy:      referredBy,
		PhoneNumberHash: fmt.Sprint(xxh3.HashStringSeed(phoneNumber, uint64(time.Now().UnixNano()))),
	}

	return u
}
