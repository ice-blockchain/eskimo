// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	tmulti "github.com/framey-io/go-tarantool/multi"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users/fixture"
	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	devicesettings "github.com/ice-blockchain/eskimo/users/internal/device/settings"
	connectorsfixture "github.com/ice-blockchain/wintr/connectors/fixture"
	messagebrokerfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
	storagefixture "github.com/ice-blockchain/wintr/connectors/storage/fixture"
)

const (
	testDeadline = 30 * time.Second
)

//nolint:gochecknoglobals // Because those are global, set only once for the whole package test runtime and execution.
var (
	dbConnector     storagefixture.TestConnector
	mbConnector     messagebrokerfixture.TestConnector
	usersRepository Repository
	usersProcessor  Processor
)

func TestMain(m *testing.M) {
	fixture.RunTests(m, &dbConnector, &mbConnector, &connectorsfixture.ConnectorLifecycleHooks{AfterConnectorsStarted: afterConnectorsStarted})
}

func afterConnectorsStarted(ctx context.Context) connectorsfixture.ContextErrClose {
	usersRepository = New(ctx, func() {})
	usersProcessor = StartProcessor(ctx, func() {})

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
func requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped(pCtx context.Context) error {
	const apiMethodsCount = 16
	errsChan := make(chan error, apiMethodsCount)
	wg := new(sync.WaitGroup)
	wg.Add(apiMethodsCount)
	//nolint:nlreturn,wrapcheck // Not needed.
	apiMethods := []func(context.Context) error{
		func(ctx context.Context) error {
			return usersProcessor.CheckHealth(ctx)
		},
		func(ctx context.Context) error {
			return usersProcessor.CreateUser(ctx, &CreateUserArg{Username: "bogus"})
		},
		func(ctx context.Context) error {
			return usersProcessor.ModifyUser(ctx, &ModifyUserArg{Username: "bogus-modified"})
		},
		func(ctx context.Context) error {
			return usersProcessor.DeleteUser(ctx, "bogus")
		},
		func(ctx context.Context) error {
			return usersProcessor.CreateDeviceSettings(ctx, &devicesettings.DeviceSettings{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}})
		},
		func(ctx context.Context) error {
			return usersProcessor.ModifyDeviceSettings(ctx, &devicesettings.DeviceSettings{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}})
		},
		func(ctx context.Context) error {
			return usersProcessor.ValidatePhoneNumber(ctx, &PhoneNumberValidation{UserID: "bogus", PhoneNumber: "bogus"})
		},
		func(ctx context.Context) error {
			return usersProcessor.ReplaceDeviceMetadata(ctx, &ReplaceDeviceMetadataArg{
				ClientIP:       net.ParseIP("1.1.1.1"),
				DeviceMetadata: devicemetadata.DeviceMetadata{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}},
			})
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetReferrals(ctx, &GetReferralsArg{UserID: "bogus", Type: Tier1Referrals})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetReferralAcquisitionHistory(ctx, &GetReferralAcquisitionHistoryArg{UserID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersRepository.GetDeviceMetadata(ctx, device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetDeviceSettings(ctx, device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetUsers(ctx, &GetUsersArg{UserID: "bogus", Keyword: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetUserByUsername(ctx, "bogususername")
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetUserProfileByID(ctx, "bogusid")
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetTopCountries(ctx, &GetTopCountriesArg{Keyword: "us"})
			return err
		},
	}
	for _, fn := range apiMethods {
		go func(apiMethod func(context.Context) error) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(pCtx, 100*time.Millisecond)
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
