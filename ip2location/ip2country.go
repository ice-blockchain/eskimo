// SPDX-License-Identifier: BUSL-1.1

package ip2location

import (
	"context"

	"github.com/ip2location/ip2location-go"
	"github.com/pkg/errors"

	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
)

func New(ctx context.Context) Repository {
	if ctx.Err() != nil {
		log.Panic(errors.Wrap(ctx.Err(), "context error"))
	}

	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db, err := ip2location.OpenDB(cfg.BinaryLocation)
	if err != nil {
		log.Panic(errors.Wrap(err, "unable to open IP database"))

		return &ip2locationRepository{}
	}

	return &ip2locationRepository{db: db}
}

func (i *ip2locationRepository) Close() error {
	return errors.Wrap(i.Close(), "error closing IP database")
}

func (i *ip2locationRepository) GetCountry(ctx context.Context, ip IP) string {
	if ctx.Err() != nil {
		log.Error(errors.Wrap(ctx.Err(), "context error"))

		return ""
	}

	results, err := i.db.Get_all(ip)
	if err != nil {
		log.Error(errors.Wrap(err, "unable to get country by ip"))

		return ""
	}

	return results.Country_short
}
