// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"net"
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

const testDeadline = 30 * time.Second

//nolint:gochecknoglobals // Because those are global, set only once for the whole package test runtime and execution.
var (
	dbConnector storagefixture.TestConnector
	mbConnector messagebrokerfixture.TestConnector
	u           Repository
	p           Processor
)

func TestMain(m *testing.M) {
	fixture.RunTests(m, &dbConnector, &mbConnector, &connectorsfixture.ConnectorLifecycleHooks{AfterConnectorsStarted: afterConnectorsStarted})
}

func afterConnectorsStarted(ctx context.Context) connectorsfixture.ContextErrClose {
	u = New(ctx, func() {})
	p = StartProcessor(ctx, func() {})

	return func(context.Context) error {
		if err := u.Close(); err != nil {
			return errors.Wrap(err, "can't close test users repository")
		}
		if err := p.Close(); err != nil {
			return errors.Wrap(err, "can't close test users processors")
		}

		return errors.Wrapf(requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped(ctx),
			"requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped failed")
	}
}

//nolint:funlen // A lot of APIs to check.
func requireAllAPIMethodsFailIfRepositoryOrProcessorAreStopped(ctx context.Context) error {
	var errs []error
	errs = append(errs,
		p.CheckHealth(ctx),
		p.CreateUser(ctx, &CreateUserArg{Username: "bogus"}),
		p.ModifyUser(ctx, &ModifyUserArg{Username: "bogus-modified"}),
		p.DeleteUser(ctx, "bogus"),
		p.CreateDeviceSettings(ctx, &devicesettings.DeviceSettings{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}}),
		p.ModifyDeviceSettings(ctx, &devicesettings.DeviceSettings{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}}),
		p.ValidatePhoneNumber(ctx, &PhoneNumberValidation{UserID: "bogus", PhoneNumber: "bogus"}),
		p.ReplaceDeviceMetadata(ctx, &ReplaceDeviceMetadataArg{
			ClientIP:       net.ParseIP("1.1.1.1"),
			DeviceMetadata: devicemetadata.DeviceMetadata{ID: device.ID{UserID: "bogus", DeviceUniqueID: "bogus"}},
		}))
	_, err := u.GetReferrals(ctx, &GetReferralsArg{UserID: "bogus", Type: Tier1Referrals})
	errs = append(errs, err)
	_, err = u.GetReferralAcquisitionHistory(ctx, &GetReferralAcquisitionHistoryArg{UserID: "bogus"})
	errs = append(errs, err)
	_, err = u.GetDeviceMetadata(ctx, device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
	errs = append(errs, err)
	_, err = p.GetDeviceSettings(ctx, device.ID{UserID: "bogus", DeviceUniqueID: "bogus"})
	errs = append(errs, err)
	_, err = p.GetUsers(ctx, &GetUsersArg{UserID: "bogus", Keyword: "bogus"})
	errs = append(errs, err)
	_, err = p.GetUserByUsername(ctx, "bogususername")
	errs = append(errs, err)
	_, err = p.GetUserByID(ctx, "bogusid")
	errs = append(errs, err)
	_, err = p.GetTopCountries(ctx, &GetTopCountriesArg{Keyword: "us"})
	errs = append(errs, err)

	var unexpectedErrors []error
	for _, e := range errs {
		if e == nil || !errors.Is(e, tmulti.ErrNoConnection) {
			unexpectedErrors = append(unexpectedErrors, e)
		}
	}
	err = multierror.Append(nil, unexpectedErrors...).ErrorOrNil()
	if err == nil {
		err = errors.New("none of the APIs failed")
	}

	return errors.Wrapf(err, "atleast one API did not error or had an unexpected error")
}

func TestCheckHealth(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()

	require.NoError(t, p.CheckHealth(ctx))
}
