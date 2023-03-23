// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"embed"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	stdlibtime "time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/eskimo/users/fixture"
	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	tmulti "github.com/ice-blockchain/go-tarantool-client/multi"
	connectorsfixture "github.com/ice-blockchain/wintr/connectors/fixture"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	messagebrokerfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
	storagefixture "github.com/ice-blockchain/wintr/connectors/storage/fixture"
	. "github.com/ice-blockchain/wintr/testing"
	"github.com/ice-blockchain/wintr/time"
)

const (
	testDeadline = 30 * stdlibtime.Second
)

//nolint:gochecknoglobals // Because those are global, set only once for the whole package test runtime and execution.
var (
	dbConnector            storagefixture.TestConnector
	mbConnector            messagebrokerfixture.TestConnector
	usersRepository        Repository
	usersProcessor         Processor
	userSnapshotProcessor  messagebroker.Processor
	defaultClientIP        = net.IPv4(1, 1, 1, 1)
	defaultClientIPCountry = "US"
	defaultClientIPCity    = "Los Angeles"
	//go:embed .testdata/*.jpg .testdata/*.png
	profilePictures embed.FS
)

func TestMain(m *testing.M) {
	fixture.RunTests(m, &dbConnector, &mbConnector, &connectorsfixture.ConnectorLifecycleHooks{AfterConnectorsStarted: afterConnectorsStarted})
}

func afterConnectorsStarted(ctx context.Context) connectorsfixture.ContextErrClose {
	usersProcessor = StartProcessor(ctx, func() {})
	usersRepository = usersProcessor
	userSnapshotProcessor = &userSnapshotSource{processor: usersProcessor.(*processor)} //nolint:forcetypeassert // We know for sure.

	return func(context.Context) error {
		if err := usersRepository.Close(); err != nil {
			return errors.Wrap(err, "can't close test users repository")
		}
		if err := usersProcessor.Close(); err != nil {
			return errors.Wrap(err, "can't close test users processors")
		}

		return errors.Wrapf(requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped(ctx),
			"requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped failed")
	}
}

//nolint:funlen // A lot of APIs to check.
func requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped(cctx context.Context) error {
	const apiMethodsCount = 17
	errsChan := make(chan error, apiMethodsCount)
	wg := new(sync.WaitGroup)
	wg.Add(apiMethodsCount)
	bogusIP := net.ParseIP("1.1.1.1")
	//nolint:nolintlint // Its gonna come back.
	pCtx := context.WithValue(cctx, requestingUserIDCtxValueKey, "bogusRequestingUserID") //nolint:revive,staticcheck // Nope.
	//nolint:nlreturn,wrapcheck // Not needed.
	apiMethods := []func(context.Context) error{
		func(ctx context.Context) error {
			return usersProcessor.CheckHealth(ctx)
		},
		func(ctx context.Context) error {
			return usersRepository.CreateUser(ctx, &User{PublicUserInformation: PublicUserInformation{Username: "bogus"}}, bogusIP)
		},
		func(ctx context.Context) error {
			return usersRepository.ModifyUser(ctx, &User{PublicUserInformation: PublicUserInformation{Username: "bogus"}}, nil)
		},
		func(ctx context.Context) error {
			return usersRepository.DeleteUser(ctx, "bogusUserID")
		},
		func(ctx context.Context) error {
			return usersRepository.CreateDeviceSettings(ctx, &devicesettings.DeviceSettings{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}})
		},
		func(ctx context.Context) error {
			return usersRepository.ModifyDeviceSettings(ctx, &devicesettings.DeviceSettings{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}})
		},
		func(ctx context.Context) error {
			_, err := usersRepository.ValidatePhoneNumber(ctx, &PhoneNumberValidation{UserID: "bogus", PhoneNumber: "bogus"})

			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.ValidateEmail(ctx, &EmailValidation{UserID: "bogus", Email: "bogus"})

			return err
		},
		func(ctx context.Context) error {
			metadata := &devicemetadata.DeviceMetadata{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}}
			metadata.ReadableVersion = "0.0.3"

			return usersRepository.ReplaceDeviceMetadata(ctx, metadata, bogusIP)
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetReferrals(ctx, "bogusUserID", Tier1Referrals, 1, 0)
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetReferralAcquisitionHistory(ctx, "bogusUserID", 5)
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetDeviceMetadata(ctx, &device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetDeviceSettings(ctx, &device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetUsers(ctx, "bogus", 1, 0)
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetUserByUsername(ctx, "bogususername")
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetUserByID(ctx, "bogusUserID")
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetTopCountries(ctx, "us", 1, 0)
			return err
		},
	}
	for _, fn := range apiMethods {
		go func(apiMethod func(context.Context) error) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(pCtx, 100*stdlibtime.Millisecond)
			defer cancel()
			errsChan <- apiMethod(ctx)
		}(fn)
	}
	wg.Wait()
	close(errsChan)
	var unexpectedErrors []error
	for e := range errsChan {
		if e == nil || (!errors.Is(e, tmulti.ErrNoConnection) && !errors.Is(e, os.ErrClosed)) {
			unexpectedErrors = append(unexpectedErrors, e)
		}
	}

	return errors.Wrapf(multierror.Append(nil, unexpectedErrors...).ErrorOrNil(), "atleast one API did not error or had an unexpected error")
}

func TestCheckHealth(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	require.NoError(t, usersProcessor.CheckHealth(ctx))
}

func TestReadOnlyRepositoryDoesNotAllowWriting(t *testing.T) { //nolint:paralleltest,funlen // We need a clean database.
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	readOnlyRepository := New(ctx, cancel)
	mustDeleteEverything(ctx, t)
	//nolint:errcheck // Not needed.
	writeAPIMethods := []func(context.Context){
		func(ctx context.Context) {
			_ = readOnlyRepository.CreateUser(ctx, new(User).completelyRandomizeForCreate(), defaultClientIP)
		},
		func(ctx context.Context) {
			usr := new(User).completelyRandomizeForCreate()
			usr.CreatedAt = time.Now()
			usr.UpdatedAt = usr.CreatedAt
			usr.HashCode = xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano()))
			usr.ProfilePictureURL = defaultUserImage
			require.NoError(t, dbConnector.InsertTyped("USERS", usr, &[]*User{}))
			mUsr := new(User)
			mUsr.ID = usr.ID
			mUsr.FirstName = "bogus"
			_ = readOnlyRepository.ModifyUser(ctx, mUsr, nil)
		},
		func(ctx context.Context) {
			usr := new(User).completelyRandomizeForCreate()
			usr.CreatedAt = time.Now()
			usr.UpdatedAt = usr.CreatedAt
			usr.HashCode = xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano()))
			usr.ProfilePictureURL = defaultUserImage
			require.NoError(t, dbConnector.InsertTyped("USERS", usr, &[]*User{}))
			_ = readOnlyRepository.DeleteUser(ctx, usr.ID)
		},
		func(ctx context.Context) {
			_ = readOnlyRepository.CreateDeviceSettings(ctx, devicesettingsfixture.CompletelyRandomizeDeviceSettings(uuid.NewString()))
		},
		func(ctx context.Context) {
			settings := devicesettingsfixture.CompletelyRandomizeDeviceSettings(uuid.NewString())
			*settings.Language = "ru"
			require.NoError(t, dbConnector.InsertTyped("DEVICE_SETTINGS", settings, &[]*devicesettings.DeviceSettings{}))
			_ = readOnlyRepository.ModifyDeviceSettings(ctx, settings)
		},
		func(ctx context.Context) {
			usr := new(User).completelyRandomizeForCreate()
			usr.CreatedAt = time.Now()
			usr.UpdatedAt = usr.CreatedAt
			usr.HashCode = xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano()))
			usr.ProfilePictureURL = defaultUserImage
			require.NoError(t, dbConnector.InsertTyped("USERS", usr, &[]*User{}))
			phoneNumberValidation := new(PhoneNumberValidation).randomize(usr.ID)
			phoneNumberValidation.mustCreate(ctx, t)
			_, _ = readOnlyRepository.ValidatePhoneNumber(ctx, phoneNumberValidation)
		},
		func(ctx context.Context) {
			usr := new(User).completelyRandomizeForCreate()
			usr.CreatedAt = time.Now()
			usr.UpdatedAt = usr.CreatedAt
			usr.HashCode = xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano()))
			usr.ProfilePictureURL = defaultUserImage
			require.NoError(t, dbConnector.InsertTyped("USERS", usr, &[]*User{}))
			emailValidation := new(EmailValidation).randomize(usr.ID)
			emailValidation.mustCreate(ctx, t)
			_, _ = readOnlyRepository.ValidateEmail(ctx, emailValidation)
		},
		func(ctx context.Context) {
			metadata := &devicemetadata.DeviceMetadata{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}}
			metadata.ReadableVersion = "0.0.1"

			_ = readOnlyRepository.ReplaceDeviceMetadata(ctx, metadata, defaultClientIP)
		},
		func(ctx context.Context) {
			_ = readOnlyRepository.GetDeviceMetadataLocation(ctx, &device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}, defaultClientIP)
		},
	}

	for i, writeAPIMethod := range writeAPIMethods {
		assert.Panics(t, func() {
			writeAPIMethod(ctx)
		}, "func %v didn't panic", fmt.Sprint(i))
	}
}

func TestAPIContract(t *testing.T) { //nolint:funlen // .
	t.Parallel()
	if testing.Short() {
		return
	}
	var (
		notExpiredTrueVal     = NotExpired(true)
		datetime              = time.New(stdlibtime.Unix(0, 1))
		publicUserInformation = PublicUserInformation{
			ID:                "a",
			Username:          "b",
			FirstName:         "c",
			LastName:          "d",
			PhoneNumber:       "e",
			ProfilePictureURL: "f",
			DeviceLocation:    DeviceLocation{Country: "g", City: "h"},
		}
		usr = &User{
			CreatedAt:               datetime,
			UpdatedAt:               datetime,
			LastMiningStartedAt:     datetime,
			LastMiningEndedAt:       datetime,
			LastPingCooldownEndedAt: datetime,
			PublicUserInformation:   publicUserInformation,
			Email:                   "i",
			ReferredBy:              "j",
			PhoneNumberHash:         "k",
			AgendaPhoneNumberHashes: "l",
			HashCode:                11,
		}
		usrShallowClone    = *usr
		userProfile        = &UserProfile{User: *usr, ReferralCount: 1}
		usrSnapshot        = &UserSnapshot{User: &usrShallowClone, Before: &usrShallowClone}
		countryStats       = &CountryStatistics{Country: "US", UserCount: 1}
		refAcq             = &ReferralAcquisition{Date: datetime, T1: 1, T2: 1}
		minimalUserProfile = MinimalUserProfile{Active: &notExpiredTrueVal, Pinged: &notExpiredTrueVal, PublicUserInformation: usr.PublicUserInformation}
		relUserProfile     = &RelatableUserProfile{ReferralType: "T1", MinimalUserProfile: minimalUserProfile}
		refs               = &Referrals{Referrals: []*Referral{{MinimalUserProfile: relUserProfile.MinimalUserProfile}}, Active: 1, Total: 1}
		pnv                = &PhoneNumberValidation{CreatedAt: datetime, UserID: usr.ID, PhoneNumber: usr.PhoneNumber, PhoneNumberHash: usr.PhoneNumberHash, ValidationCode: "123456"} //nolint:lll // .
		ev                 = &EmailValidation{CreatedAt: datetime, UserID: usr.ID, Email: usr.Email, ValidationCode: "123456"}                                                         //nolint:lll // .
	)
	AssertSymmetricMarshallingUnmarshalling(t, relUserProfile, `{
																  "active": true,
																  "pinged": true,
																  "id": "a",
																  "username": "b",
																  "firstName": "c",
																  "lastName": "d",
																  "phoneNumber": "e",
																  "profilePictureUrl": "f",
																  "country": "g",
																  "city": "h",
																  "referralType": "T1"
																}`)
	AssertSymmetricMarshallingUnmarshalling(t, userProfile, `{
															  "createdAt": "1970-01-01T00:00:00.000000001Z",
															  "updatedAt": "1970-01-01T00:00:00.000000001Z",
															  "lastMiningEndedAt": "1970-01-01T00:00:00.000000001Z",
															  "lastPingCooldownEndedAt": "1970-01-01T00:00:00.000000001Z",
															  "id": "a",
															  "username": "b",
															  "firstName": "c",
															  "lastName": "d",
															  "phoneNumber": "e",
															  "profilePictureUrl": "f",
															  "country": "g",
															  "city": "h",
															  "email": "i",
															  "referredBy": "j",
															  "agendaPhoneNumberHashes": "l",
															  "referralCount": 1
															 }`, `{
																  "referralCount": 0
																}`)
	AssertSymmetricMarshallingUnmarshalling(t, usr, `{
													  "createdAt": "1970-01-01T00:00:00.000000001Z",
													  "updatedAt": "1970-01-01T00:00:00.000000001Z",
													  "lastMiningEndedAt": "1970-01-01T00:00:00.000000001Z",
													  "lastPingCooldownEndedAt": "1970-01-01T00:00:00.000000001Z",
													  "id": "a",
													  "username": "b",
													  "firstName": "c",
													  "lastName": "d",
													  "phoneNumber": "e",
													  "profilePictureUrl": "f",
													  "country": "g",
													  "city": "h",
													  "email": "i",
													  "referredBy": "j",
													  "agendaPhoneNumberHashes": "l"
													 }`)
	AssertSymmetricMarshallingUnmarshalling(t, usrSnapshot, `{
																  "createdAt": "1970-01-01T00:00:00.000000001Z",
																  "updatedAt": "1970-01-01T00:00:00.000000001Z",
																  "lastMiningEndedAt": "1970-01-01T00:00:00.000000001Z",
																  "lastPingCooldownEndedAt": "1970-01-01T00:00:00.000000001Z",
																  "id": "a",
																  "username": "b",
																  "firstName": "c",
																  "lastName": "d",
																  "phoneNumber": "e",
																  "profilePictureUrl": "f",
																  "country": "g",
																  "city": "h",
																  "email": "i",
																  "referredBy": "j",
																  "agendaPhoneNumberHashes": "l",
																  "before": {
																	"createdAt": "1970-01-01T00:00:00.000000001Z",
																	"updatedAt": "1970-01-01T00:00:00.000000001Z",
																	"lastMiningEndedAt": "1970-01-01T00:00:00.000000001Z",
																	"lastPingCooldownEndedAt": "1970-01-01T00:00:00.000000001Z",
																	"id": "a",
																	"username": "b",
																	"firstName": "c",
																	"lastName": "d",
																	"phoneNumber": "e",
																	"profilePictureUrl": "f",
																	"country": "g",
																	"city": "h",
																	"email": "i",
																	"referredBy": "j",
																	"agendaPhoneNumberHashes": "l"
																  }
															 }`)
	AssertSymmetricMarshallingUnmarshalling(t, pnv, `{
													  "createdAt": "1970-01-01T00:00:00.000000001Z",
													  "userId": "a",
													  "phoneNumber": "e",
													  "phoneNumberHash": "k",
													  "validationCode": "123456"
													 }`)
	AssertSymmetricMarshallingUnmarshalling(t, ev, `{
													  "createdAt": "1970-01-01T00:00:00.000000001Z",
													  "userId": "a",
													  "email": "i",
													  "validationCode": "123456"
													 }`)
	AssertSymmetricMarshallingUnmarshalling(t, countryStats, `{
																  "country": "US",
																  "userCount": 1
															  }`, `{
																	  "country": "",
																	  "userCount": 0
																	}`)
	AssertSymmetricMarshallingUnmarshalling(t, refs, `{
														  "referrals": [
															{
															  "active": true,
															  "pinged": true,
															  "id": "a",
															  "username": "b",
															  "firstName": "c",
															  "lastName": "d",
															  "phoneNumber": "e",
															  "profilePictureUrl": "f",
															  "country": "g",
															  "city": "h"
															}
														  ],
														  "active": 1,
														  "total": 1
													  }`, `{
															  "referrals": null,
															  "active": 0,
															  "total": 0
															}`)
	AssertSymmetricMarshallingUnmarshalling(t, refs.Referrals[0], `{
																	  "active": true,
																	  "pinged": true,
																	  "id": "a",
																	  "username": "b",
																	  "firstName": "c",
																	  "lastName": "d",
																	  "phoneNumber": "e",
																	  "profilePictureUrl": "f",
																	  "country": "g",
																	  "city": "h"
																   }`)
	AssertSymmetricMarshallingUnmarshalling(t, refAcq, `{
														  "date": "1970-01-01T00:00:00.000000001Z",
														  "t1": 1,
														  "t2": 1
														}`, `{
															  "date": null,
															  "t1": 0,
															  "t2": 0
															 }`)
}
