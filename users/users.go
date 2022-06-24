// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	stdlibtime "time"

	"github.com/cenkalti/backoff/v4"
	"github.com/framey-io/go-tarantool"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	devicesettings "github.com/ice-blockchain/eskimo/users/internal/device/settings"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
)

func New(ctx context.Context, cancel context.CancelFunc) Repository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)

	return &repository{
		close:                    db.Close,
		db:                       db,
		DeviceMetadataRepository: devicemetadata.New(db, nil),
		DeviceSettingsRepository: devicesettings.New(db, nil),
	}
}

func StartProcessor(ctx context.Context, cancel context.CancelFunc) Processor {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)
	mbProducer := messagebroker.MustConnect(ctx, applicationYamlKey)

	p := &processor{}
	mbConsumer := messagebroker.MustConnectAndStartConsuming(context.Background(), cancel, applicationYamlKey, map[messagebroker.Topic]messagebroker.Processor{
		cfg.MessageBroker.ConsumingTopics[0]: &userSnapshotSource{processor: p},
		cfg.MessageBroker.ConsumingTopics[1]: &miningStartedSource{processor: p},
	})

	deviceMetadataRepository := devicemetadata.New(db, mbProducer)
	p.repository = &repository{
		close:                    closeAll(mbConsumer, mbProducer, db, deviceMetadataRepository.Close),
		db:                       db,
		mb:                       mbProducer,
		DeviceMetadataRepository: deviceMetadataRepository,
		DeviceSettingsRepository: devicesettings.New(db, mbProducer),
		twilioClient:             initTwilioClient(),
	}

	return p
}

func (r *repository) Close() error {
	return errors.Wrap(r.close(), "closing users repository failed")
}

func closeAll(mbConsumer, mbProducer messagebroker.Client, db tarantool.Connector, otherClosers ...func() error) func() error {
	return func() error {
		err1 := errors.Wrap(mbConsumer.Close(), "closing message broker consumer connection failed")
		err2 := errors.Wrap(mbProducer.Close(), "closing message broker producer connection failed")
		err3 := errors.Wrap(db.Close(), "closing db connection failed")
		errs := make([]error, 0, 1+1+1+len(otherClosers))
		errs = append(errs, err1, err2, err3)
		for _, closeOther := range otherClosers {
			if err := closeOther(); err != nil {
				errs = append(errs, err)
			}
		}

		return errors.Wrap(multierror.Append(nil, errs...).ErrorOrNil(), "failed to close resources")
	}
}

func (p *processor) CheckHealth(_ context.Context) error {
	//nolint:nolintlint    // TODO implement me.
	return nil
}

func retry(ctx context.Context, op func() error) error {
	//nolint:wrapcheck // No need, its just a proxy.
	return backoff.RetryNotify(
		op,
		//nolint:gomnd // Because those are static configs.
		backoff.WithContext(&backoff.ExponentialBackOff{
			InitialInterval:     100 * stdlibtime.Millisecond,
			RandomizationFactor: 0.5,
			Multiplier:          2.5,
			MaxInterval:         stdlibtime.Second,
			MaxElapsedTime:      25 * stdlibtime.Second,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		}, ctx),
		func(e error, next stdlibtime.Duration) {
			log.Error(errors.Wrapf(e, "call failed. retrying in %v... ", next))
		})
}
