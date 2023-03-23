// SPDX-License-Identifier: ice License 1.0

package users

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"
	"golang.org/x/net/http2"

	. "github.com/ice-blockchain/wintr/testing"
	"github.com/ice-blockchain/wintr/time"
)

func TestRepository_ModifyUser_Success_AllFields(t *testing.T) { //nolint:funlen,paralleltest,revive // .
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*testDeadline)
	defer cancel()
	var (
		toPhoneNumber     = smsfixture.TestingPhoneNumber(0)
		toPhoneNumberHash = "h11"
		toEmail           = emailfixture.TestingEmail(ctx, t)
	)
	SETUP("we clean number's message queue/inbox", func() {
		require.NoError(t, smsfixture.ClearSMSQueue(toPhoneNumber))
	})
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	var usr *User
	GIVEN("we have an user", func() {
		usr = new(User).completelyRandomizeForCreate()
		require.NoError(t, usr.mustCreate(ctx, t))
		assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
	})
	var (
		userMod  = *new(User).randomizeForModification(usr.ID, "JP", toEmail, toPhoneNumber, toPhoneNumberHash)
		original = *usr
		err      error
	)
	WHEN("modifying the user with all possible fields at once", func() {
		cpy := userMod
		err = (&cpy).mustModify(ctx, t, getProfilePic(t, "profilePic1.jpg"))
		*usr = cpy
	})
	pnv := new(PhoneNumberValidation)
	ev := new(EmailValidation)
	THEN(func() {
		IT("has no error", func() {
			require.NoError(t, err)
		})
		IT("has all the expected fields updated", func() {
			assert.EqualValues(t, userMod.Username, usr.Username)
			assert.EqualValues(t, userMod.FirstName, usr.FirstName)
			assert.EqualValues(t, userMod.LastName, usr.LastName)
			assert.EqualValues(t, userMod.Country, usr.Country)
			assert.EqualValues(t, userMod.City, usr.City)
			assert.EqualValues(t, userMod.AgendaPhoneNumberHashes, usr.AgendaPhoneNumberHashes)
			assert.EqualValues(t, original.PhoneNumber, usr.PhoneNumber)
			assert.EqualValues(t, original.PhoneNumberHash, usr.PhoneNumberHash)
			assert.EqualValues(t, original.Email, usr.Email)
		})
		IT("has phoneNumber validation saved in DB matching with validation code", func() {
			pnv.assertCreated(ctx, t, usr.ID, toPhoneNumber, toPhoneNumberHash)
			assert.EqualValues(t, usr.UpdatedAt, pnv.CreatedAt)
		})
		IT("has email validation saved in DB matching with validation code", func() {
			ev.assertCreated(ctx, t, usr.ID, toEmail)
			assert.EqualValues(t, usr.UpdatedAt, ev.CreatedAt)
		})
		IT("delivered SMS with validation code", func() {
			pnv.assertPhoneNumberConfirmationCodeDelivered(ctx, t)
		})
		IT("delivered email with validation code", func() {
			ev.assertEmailConfirmationCodeDelivered(ctx, t)
		})
		IT("saved the new profile picture onto the remote media storage", func() {
			assertProfilePictureUploaded(ctx, t, usr.ProfilePictureURL)
		})
		IT("sends UserSnapshot message to message broker with all those fields updated", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: &original})
		})
		IT("has shifted users per country stats from default country to JP by 1", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 0)
			assertUsersPerCountry(ctx, t, "JP", 1)
		})
	})
	original = *usr
	WHEN("validating the phoneNumber", func() {
		err = usr.mustValidatePhoneNumber(ctx, t, pnv)
	})
	THEN(func() {
		IT("is successful", func() {
			require.NoError(t, err)
		})
		IT("returns the updated user", func() {
			assert.NotEqualValues(t, original.UpdatedAt, usr.UpdatedAt)
			origCopy := original
			origCopy.PhoneNumber = userMod.PhoneNumber
			origCopy.PhoneNumberHash = userMod.PhoneNumberHash
			origCopy.UpdatedAt = usr.UpdatedAt
			assert.EqualValues(t, &origCopy, usr)
		})
		IT("sends UserSnapshot message to message broker with new phoneNumber", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: &original})
		})
	})
	original = *usr
	WHEN("validating the email", func() {
		err = usr.mustValidateEmail(ctx, t, ev)
	})
	THEN(func() {
		IT("is successful", func() {
			require.NoError(t, err)
		})
		IT("returns the updated user", func() {
			assert.NotEqualValues(t, original.UpdatedAt, usr.UpdatedAt)
			origCopy := original
			origCopy.Email = userMod.Email
			origCopy.UpdatedAt = usr.UpdatedAt
			assert.EqualValues(t, &origCopy, usr)
		})
		IT("sends UserSnapshot message to message broker with new phoneNumber", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: &original})
		})
	})
	original = *usr
	WHEN("modifying the profile picture again", func() {
		cpy := new(User)
		cpy.ID = usr.ID
		err = cpy.mustModify(ctx, t, getProfilePic(t, "profilePic2.png"))
		*usr = *cpy
	})
	THEN(func() {
		IT("has no error", func() {
			require.NoError(t, err)
		})
		IT("returns the updated user", func() {
			assert.NotEqualValues(t, original.UpdatedAt, usr.UpdatedAt)
			assert.NotEqualValues(t, original.ProfilePictureURL, usr.ProfilePictureURL)
			origCopy := original
			origCopy.ProfilePictureURL = usr.ProfilePictureURL
			origCopy.UpdatedAt = usr.UpdatedAt
			assert.EqualValues(t, &origCopy, usr)
		})
		IT("saved the new profile picture onto the remote media storage", func() {
			assertProfilePictureUploaded(ctx, t, usr.ProfilePictureURL)
		})
		IT("sends UserSnapshot message to message broker with all those fields updated", func() {
			verifyUserSnapshotMessages(ctx, t, &UserSnapshot{User: usr, Before: &original})
		})
	})
}

func TestRepository_ModifyUser_Failure_InvalidCountryOrCity(t *testing.T) { //nolint:funlen // .
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	var usr *User
	GIVEN("we have an user", func() {
		usr = new(User).completelyRandomizeForCreate()
		require.NoError(t, usr.mustCreate(ctx, t))
	})
	var err error
	WHEN("modifying the country with an invalid one", func() {
		usr.Country = "bogusCountry"
		err = usr.mustModify(ctx, t)
	})
	THEN(func() {
		IT("returns specific error", func() {
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidCountry)
		})
	})
	WHEN("modifying the country, but omitting to modify the city as well", func() {
		usr.Country = "RU"
		err = usr.mustModify(ctx, t)
	})
	THEN(func() {
		IT("returns specific error", func() {
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidCity)
		})
	})
}

func TestRepository_ModifyUser_Failure_DuplicateUserName(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	var usr1, usr2 *User
	GIVEN("we have 2 users", func() {
		usr1 = new(User).completelyRandomizeForCreate()
		require.NoError(t, usr1.mustCreate(ctx, t))
		usr2 = new(User).completelyRandomizeForCreate()
		require.NoError(t, usr2.mustCreate(ctx, t))
	})
	var err error
	WHEN("modifying the username of 1 with the username of 2", func() {
		usrMod := new(User)
		usrMod.ID = usr1.ID
		usrMod.Username = usr2.Username
		err = usrMod.mustModify(ctx, t)
	})
	THEN(func() {
		IT("returns specific error", func() {
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrDuplicate)
		})
	})
}

func TestRepository_ModifyUser_Failure_NonExistingUser(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	GIVEN("we have no specific users", func() {})
	var err error
	WHEN("modifying some non existing user", func() {
		err = new(User).randomizeForModification(uuid.NewString(), "FR", uuid.NewString(), uuid.NewString(), uuid.NewString()).mustModify(ctx, t)
	})
	THEN(func() {
		IT("returns specific error", func() {
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrNotFound)
		})
	})
}

func (u *User) mustModify(ctx context.Context, tb testing.TB, profilePicture ...*multipart.FileHeader) (err error) { //nolint:funlen // .
	tb.Helper()
	require.NoError(tb, ctx.Err())

	before := new(User).bindExisting(ctx, tb, u.ID)

	var pic *multipart.FileHeader
	if len(profilePicture) == 1 {
		pic = profilePicture[0]
	}
	valuesToChange := *u
	err = usersRepository.ModifyUser(ctx, u, pic)

	after := new(User).bindExisting(ctx, tb, u.ID)

	if err == nil {
		assert.NotEmpty(tb, *before)
		expected := *u
		expected.ProfilePictureURL = after.ProfilePictureURL
		assert.NotNil(tb, after.CreatedAt)
		assert.NotNil(tb, after.UpdatedAt)
		// In case of email/phone change trigger user is not updated at all, so skip this verification.
		if before.PhoneNumber == valuesToChange.PhoneNumber || before.Email == valuesToChange.Email {
			assert.NotEqualValues(tb, after.CreatedAt, after.UpdatedAt)
			assert.InDelta(tb, after.UpdatedAt.Unix(), time.Now().Unix(), 2)
		}
		assert.NotEmpty(tb, after.ReferredBy)
		assert.NotEmpty(tb, after.City)
		assert.NotEmpty(tb, after.Country)
		assert.NotEmpty(tb, after.HashCode)
		if len(profilePicture) == 1 {
			splittedFilename := strings.Split(profilePicture[0].Filename, ".")
			assert.Greater(tb, len(splittedFilename), 1, "file without extension")
			ext := splittedFilename[len(splittedFilename)-1]
			expectedPicName := fmt.Sprintf("%v_%v.%v", xxh3.HashStringSeed(after.ID, uint64(after.CreatedAt.UnixNano())), after.UpdatedAt.UnixNano(), ext)
			assert.Equal(tb, expectedPicName, after.ProfilePictureURL)
			assert.NotEqual(tb, expectedPicName, before.ProfilePictureURL)
		} else {
			assert.Equal(tb, defaultUserImage, after.ProfilePictureURL)
		}
		assert.Equal(tb, fmt.Sprintf("%v/%v", cfg.PictureStorage.URLDownload, after.ProfilePictureURL), u.ProfilePictureURL)
		assert.EqualValues(tb, &expected, after)
	} else {
		assert.EqualValues(tb, before, after)
	}
	if before.ID == "" || after.ID == "" {
		assert.NotNil(tb, err)
		assert.EqualValues(tb, before, after)
	}

	return err //nolint:wrapcheck // That's what we intend, to proxy it as-is.
}

func (u *User) randomizeForModification(userID, country, email, phoneNumber, phoneNumberHash string) *User {
	*u = User{
		PublicUserInformation: PublicUserInformation{
			ID:        userID,
			Username:  uuid.NewString(),
			FirstName: fmt.Sprintf("FirstName-%v", uuid.NewString()),
			LastName:  fmt.Sprintf("LastName-%v", uuid.NewString()),
			DeviceLocation: DeviceLocation{
				Country: country,
				City:    uuid.NewString(),
			},
			PhoneNumber: phoneNumber,
		},
		Email:                   email,
		AgendaPhoneNumberHashes: uuid.NewString(),
		PhoneNumberHash:         phoneNumberHash,
	}

	return u
}

func getProfilePic(tb testing.TB, fileName string) *multipart.FileHeader {
	tb.Helper()
	pic, fErr := profilePictures.Open(fmt.Sprintf(".testdata/%v", fileName))
	require.NoError(tb, fErr)
	defer require.NoError(tb, pic.Close())
	stat, err := pic.Stat()
	require.NoError(tb, err)
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	formFile, err := writer.CreateFormFile("profilePic", fileName)
	require.NoError(tb, err)
	_, err = io.Copy(formFile, pic)
	require.NoError(tb, err)
	require.NoError(tb, writer.Close())
	form, err := multipart.NewReader(body, writer.Boundary()).ReadForm(stat.Size())
	require.NoError(tb, err)
	assert.Greater(tb, len(form.File["profilePic"]), 0)

	return form.File["profilePic"][0]
}

func assertProfilePictureUploaded(ctx context.Context, tb testing.TB, url string) {
	tb.Helper()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	require.NoError(tb, err)
	//nolint:gosec // Skip checking cert chain from CDN
	client := &http.Client{Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := client.Do(r)
	defer func() {
		require.NoError(tb, resp.Body.Close())
	}()
	require.NoError(tb, err)
	assert.Equal(tb, 200, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(tb, err)
	assert.Greater(tb, len(b), 0)
}
