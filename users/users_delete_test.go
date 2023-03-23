// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"sync"
	"testing"
	stdlibtime "time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicemetadatafixture "github.com/ice-blockchain/eskimo/users/internal/device/metadata/fixture"
	"github.com/ice-blockchain/wintr/connectors/storage"
	. "github.com/ice-blockchain/wintr/testing"
)

func TestRepository_DeleteUser_Success_FirstUser(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	var (
		usr1 = new(User)
		usr2 = new(User)
	)
	GIVEN("we have 2 users in the database", func() {
		require.NoError(t, usr1.completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, usr2.completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	GIVEN("we have 2 user count for their country(default one)", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 2)
	})
	var err error
	WHEN("deleting the first user", func() {
		err = usr1.mustDelete(ctx, t)
	})
	THEN(func() {
		IT("is successfully deleted", func() {
			require.NoError(t, err)
		})
		IT("generated a new deleted user snapshot message for it", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: nil, Before: usr1})
		})
		IT("updated the other one as well", func() {
			updatedUsr2 := new(User).bindExisting(ctx, t, usr2.ID)
			updatedUsr2.setCorrectProfilePictureURL()
			IT("updated the other user and forced it to become the `first`/genesis user, that refers to itself", func() {
				assert.Equal(t, updatedUsr2.ID, updatedUsr2.ReferredBy)
			})
			IT("generated a new updated user snapshot message for other user", func() {
				verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: updatedUsr2, Before: usr2})
			})
		})
		IT("decremented `users_per_country` by 1 for its country(default one); resulting in 1 because 1 user is left", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
		})
	})
}

func TestRepository_DeleteUser_Success_UpdateReferredByForT1Referrals(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	var (
		firstUser = new(User)
		usr1      = new(User)
		usr2      = new(User)
	)
	GIVEN("we have some users", func() {
		require.NoError(t, firstUser.completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, usr1.completelyRandomizeForCreate().mustCreate(ctx, t))
		require.NoError(t, usr2.completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	var (
		ref11 = new(User)
		ref12 = new(User)
		ref21 = new(User)
		ref22 = new(User)
	)
	GIVEN("those users have some T1 referrals", func() {
		require.NoError(t, ref11.randomizeForCreateWithReferredBy(usr1.ID).mustCreate(ctx, t))
		require.NoError(t, ref12.randomizeForCreateWithReferredBy(usr1.ID).mustCreate(ctx, t))
		require.NoError(t, ref21.randomizeForCreateWithReferredBy(usr2.ID).mustCreate(ctx, t))
		require.NoError(t, ref22.randomizeForCreateWithReferredBy(usr2.ID).mustCreate(ctx, t))
	})
	GIVEN("we have 7 user count for their country(default one)", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 7)
	})
	var err error
	WHEN("deleting a user that has some T1 referrals", func() {
		err = usr2.mustDelete(ctx, t)
	})
	THEN(func() {
		IT("is successfully deleted", func() {
			require.NoError(t, err)
		})
		IT("generated a new deleted user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: nil, Before: usr2})
		})
		IT("updated ONLY its referrals", func() {
			updatedRef21 := new(User).bindExisting(ctx, t, ref21.ID)
			updatedRef22 := new(User).bindExisting(ctx, t, ref22.ID)
			updatedRef21.setCorrectProfilePictureURL()
			updatedRef22.setCorrectProfilePictureURL()
			IT("updated the `referredBy` ONLY for its referrals", func() {
				assert.NotEmpty(t, updatedRef21.ReferredBy)
				assert.NotEmpty(t, updatedRef22.ReferredBy)
				assert.True(t, updatedRef21.ReferredBy == ref11.ID || updatedRef21.ReferredBy == ref12.ID || updatedRef21.ReferredBy == firstUser.ID || updatedRef21.ReferredBy == usr1.ID) //nolint:lll // .
				assert.True(t, updatedRef22.ReferredBy == ref11.ID || updatedRef22.ReferredBy == ref12.ID || updatedRef22.ReferredBy == firstUser.ID || updatedRef22.ReferredBy == usr1.ID) //nolint:lll // .
			})
			IT("didn't update any other user", func() {
				assert.Equal(t, usr1.ID, new(User).bindExisting(ctx, t, ref11.ID).ReferredBy)
				assert.Equal(t, usr1.ID, new(User).bindExisting(ctx, t, ref12.ID).ReferredBy)
				assert.Equal(t, firstUser.ID, new(User).bindExisting(ctx, t, firstUser.ID).ReferredBy)
				assert.Equal(t, firstUser.ID, new(User).bindExisting(ctx, t, usr1.ID).ReferredBy)
			})
			IT("generated a new updated user snapshot message ONLY for its referrals", func() {
				verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: updatedRef21, Before: ref21})
				verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: updatedRef22, Before: ref22})
			})
			IT("didn't generate new user snapshot messages for any other user", func() {
				verifyNoUserSnapshotMessages(ctx, t, UPDATE, ref11.ID, ref12.ID, usr1.ID, firstUser.ID)
			})
		})
		IT("decremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 6)
		})
	})
}

func TestRepository_DeleteUser_Success_Everything(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	usr := new(User)
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	GIVEN("we have an user", func() {
		require.NoError(t, usr.completelyRandomizeForCreate().mustCreate(ctx, t))
	})
	deviceIDs := make([]device.ID, 0, 4)
	GIVEN("that user has some devices and some progress", func() {
		new(PhoneNumberValidation).randomize(usr.ID).mustCreate(ctx, t)
		new(EmailValidation).randomize(usr.ID).mustCreate(ctx, t)
		ds1 := devicesettingsfixture.CompletelyRandomizeDeviceSettings(usr.ID)
		ds2 := devicesettingsfixture.CompletelyRandomizeDeviceSettings(usr.ID)

		require.NoError(t, usersRepository.CreateDeviceSettings(ctx, ds1))
		require.NoError(t, usersRepository.CreateDeviceSettings(ctx, ds2))
		assert.Len(t, devicesettingsfixture.AllExistingDeviceSettings(ctx, t, dbConnector, usr.ID), 2)

		dm1 := devicemetadatafixture.CompletelyRandomizeDeviceMetadata(usr.ID)
		dm2 := devicemetadatafixture.CompletelyRandomizeDeviceMetadata(usr.ID)

		require.NoError(t, usersRepository.ReplaceDeviceMetadata(ctx, dm1, defaultClientIP))
		require.NoError(t, usersRepository.ReplaceDeviceMetadata(ctx, dm2, defaultClientIP))
		assert.Len(t, devicemetadatafixture.AllExistingDeviceMetadata(ctx, t, dbConnector, usr.ID), 2)

		deviceIDs = append(deviceIDs, ds1.ID, ds2.ID, dm1.ID, dm2.ID)
	})
	GIVEN("we have 1 user count for its country(default one)", func() {
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
	})
	var err error
	WHEN("deleting that user", func() {
		err = usr.mustDelete(ctx, t)
	})
	THEN(func() {
		IT("is successfully deleted", func() {
			require.NoError(t, err)
		})
		IT("generated a new deleted user snapshot message", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: nil, Before: usr})
		})
		IT("decremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 0)
		})
		IT("removed all device settings of that user", func() {
			assert.Empty(t, devicesettingsfixture.AllExistingDeviceSettings(ctx, t, dbConnector, usr.ID))
		})
		IT("removed all device metadata of that user", func() {
			assert.Empty(t, devicemetadatafixture.AllExistingDeviceMetadata(ctx, t, dbConnector, usr.ID))
		})
		IT("removed all phone number validations of that user", func() {
			assert.Empty(t, *(new(PhoneNumberValidation).bindExisting(ctx, t, usr.ID)))
		})
		IT("removed all email validations of that user", func() {
			assert.Empty(t, *(new(EmailValidation).bindExisting(ctx, t, usr.ID)))
		})
		IT("didnt generate any messages for device metadata entries", func() {
			devicemetadatafixture.VerifyNoDeviceMetadataSnapshotMessages(ctx, t, mbConnector, devicemetadatafixture.DELETE, deviceIDs...)
		})
		IT("didnt generate new messages for device settings entries", func() {
			devicesettingsfixture.VerifyNoDeviceSettingsSnapshotMessages(ctx, t, mbConnector, devicesettingsfixture.DELETE, deviceIDs...)
		})
	})
}

func TestRepository_DeleteUser_Failure_UserNotFound(t *testing.T) { //nolint:paralleltest // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	GIVEN("we have no users", func() {})
	var (
		nonExistingUser = new(User).completelyRandomizeForCreate()
		err             error
	)
	WHEN("deleting a non-existing user", func() {
		err = nonExistingUser.mustDelete(ctx, t)
	})
	THEN(func() {
		IT("is not deleted", func() {
			require.NotNil(t, err)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
		IT("didnt generate any user snapshot messages", func() {
			verifyNoUserSnapshotMessages(ctx, t, ANY, nonExistingUser.ID)
		})
		IT("didn't decrement `users_per_country` for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 0)
		})
	})
}

func TestRepository_DeleteUser_Success_Concurrently(t *testing.T) { //nolint:paralleltest,revive,gocognit,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*stdlibtime.Minute)
	defer cancel()
	for i := 0; i < 25; i++ {
		SETUP("we cleanup everything in the database", func() {
			mustDeleteEverything(ctx, t)
		})
		tenUsers := make([]User, 10, 10) //nolint:gosimple // Prefer to be descriptive.
		GIVEN("we have 10 users", func() {
			for ix := range tenUsers {
				require.NoError(t, (&tenUsers[ix]).completelyRandomizeForCreate().mustCreate(ctx, t))
			}
		})
		GIVEN("we have 10 user count for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 10)
		})
		var err error
		WHEN("deleting all users, concurrently, in an unordered manner", func() {
			errChan := make(chan error, 10)
			wg := new(sync.WaitGroup)
			wg.Add(10)
			for ii := range tenUsers {
				go func(ix int) {
					defer wg.Done()
					errChan <- (&tenUsers[ix]).mustDelete(ctx, t)
				}(ii)
			}
			wg.Wait()
			close(errChan)
			errs := make([]error, 10, 10) //nolint:gosimple // Prefer to be descriptive.
			for dErr := range errChan {
				errs = append(errs, dErr)
			}
			err = multierror.Append(nil, errs...).ErrorOrNil()
		})
		THEN(func() {
			IT("they are all successfully deleted", func() {
				require.NoError(t, err)
			})
			IT("generated a new updated user snapshot messages for users with updated referrals; except first"+
				"but some users was deleted in parallel thread, for them we have deleted message and no update one", func() {
				userIDs := make([]string, 0, 9)
				for ix := range tenUsers {
					if ix == 0 {
						continue
					}
					userIDs = append(userIDs, tenUsers[ix].ID)
				}
				verifyUserSnapshotUpdatedMessageOrDeletedWithNoUpdate(ctx, t, userIDs...)
				verifyNoUserSnapshotMessages(ctx, t, UPDATE, tenUsers[0].ID)
			})
			IT("generated a new deleted user snapshot message for each user", func() {
				for ix := range tenUsers {
					verifySomeUserSnapshotMessages(ctx, t, DELETE, tenUsers[ix].ID)
				}
			})
			IT("decremented `users_per_country` by 1 for their country(default one); ending up with 0", func() {
				assertUsersPerCountry(ctx, t, defaultClientIPCountry, 0)
			})
		})
	}
}

func (u *User) mustDelete(ctx context.Context, tb testing.TB) (err error) {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	before := new(User).bindExisting(ctx, tb, u.ID)

	err = usersRepository.DeleteUser(ctx, u.ID)

	after := new(User).bindExisting(ctx, tb, u.ID)

	if err == nil {
		assert.Empty(tb, *after)
		assert.Equal(tb, u.ID, before.ID)
	} else {
		assert.EqualValues(tb, before, after)
		if errors.Is(err, storage.ErrNotFound) {
			assert.Empty(tb, *before)
		} else {
			assert.Equal(tb, u.ID, before.ID)
		}
	}

	return err //nolint:wrapcheck // That's what we intend, to proxy it as-is.
}

func mustDeleteAllUsers(ctx context.Context, tb testing.TB) {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	err := storage.CheckSQLDMLErr(dbConnector.PrepareExecute("DELETE FROM users WHERE 1=1", map[string]any{}))
	require.True(tb, err == nil || errors.Is(err, storage.ErrNotFound))
}

func mustDeleteEverything(ctx context.Context, tb testing.TB) {
	tb.Helper()
	require.NoError(tb, ctx.Err())

	mustDeleteAllUsers(ctx, tb)

	noArgs := map[string]any{}
	err := storage.CheckSQLDMLErr(dbConnector.PrepareExecute("DELETE FROM users_per_country WHERE 1=1", noArgs))
	require.True(tb, err == nil || errors.Is(err, storage.ErrNotFound))
	require.ErrorIs(tb, storage.CheckSQLDMLErr(dbConnector.PrepareExecute("DELETE FROM phone_number_validations WHERE 1=1", noArgs)), storage.ErrNotFound)
	require.ErrorIs(tb, storage.CheckSQLDMLErr(dbConnector.PrepareExecute("DELETE FROM email_validations WHERE 1=1", noArgs)), storage.ErrNotFound)
	require.ErrorIs(tb, storage.CheckSQLDMLErr(dbConnector.PrepareExecute("DELETE FROM device_settings WHERE 1=1", noArgs)), storage.ErrNotFound)
	require.ErrorIs(tb, storage.CheckSQLDMLErr(dbConnector.PrepareExecute("DELETE FROM device_metadata WHERE 1=1", noArgs)), storage.ErrNotFound)
}

func verifyUserSnapshotUpdatedMessageOrDeletedWithNoUpdate(ctx context.Context, tb testing.TB, userID ...string) {
	tb.Helper()
	verifyAnyOfUserSnapshotMessages(ctx, tb, func() error {
		return verifySomeUserSnapshotMessagesWithError(ctx, tb, UPDATE, userID...)
	}, func() error {
		err := verifySomeUserSnapshotMessagesWithError(ctx, tb, DELETE, userID...)
		if err == nil {
			err = verifyNoUserSnapshotMessagesWithError(ctx, tb, UPDATE, userID...)
		}

		return err
	})
}
