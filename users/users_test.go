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
	// Example address in US, New York City.
	testClientIP = net.IPv4(72, 229, 28, 185)
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
func requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped(cctx context.Context) error {
	const apiMethodsCount = 16
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
			return usersProcessor.CreateUser(ctx, &User{PublicUserInformation: PublicUserInformation{Username: "bogus"}}, bogusIP)
		},
		func(ctx context.Context) error {
			return usersProcessor.ModifyUser(ctx, &User{PublicUserInformation: PublicUserInformation{Username: "bogus"}}, nil)
		},
		func(ctx context.Context) error {
			return usersProcessor.DeleteUser(ctx, "bogusUserID")
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
			return usersProcessor.ReplaceDeviceMetadata(ctx, &devicemetadata.DeviceMetadata{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}}, bogusIP)
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
			_, err := usersRepository.GetDeviceMetadata(ctx, device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetDeviceSettings(ctx, device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetUsers(ctx, "bogus", 1, 0)
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetUserByUsername(ctx, "bogususername")
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetUserByID(ctx, "bogusUserID")
			return err
		},
		func(ctx context.Context) error {
			_, err := usersProcessor.GetTopCountries(ctx, "us", 1, 0)
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
