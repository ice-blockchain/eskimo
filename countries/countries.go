// SPDX-License-Identifier: BUSL-1.1

package countries

import (
	"context"
	"strings"

	"github.com/ip2location/ip2location-go"
	"github.com/pkg/errors"

	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
)

//nolint:gochecknoinits // init
func init() {
	v := strings.Split(countriesList, "\n")
	if len(v) != 240 { //nolint:gomnd // We have 240 countries in 2022
		log.Panic("Empty or corrupted country list database")
	}

	countries = make(map[string]bool)

	for _, a := range v {
		countries[strings.ToLower(a)] = true
	}
}

func New(ctx context.Context) Repository {
	log.Panic(errors.Wrap(ctx.Err(), "context error"))

	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db, err := ip2location.OpenDB(cfg.IP2LocationBinaryPath)
	log.Panic(errors.Wrap(err, "unable to open IP database"))

	return &countriesRepository{db: db}
}

func (i *countriesRepository) Close() error {
	i.db.Close()

	return nil
}

func (i *countriesRepository) Get(ctx context.Context, ip IP) string {
	if ctx.Err() != nil {
		log.Error(errors.Wrap(ctx.Err(), "context error"))

		return ""
	}

	results, err := i.db.Get_all(ip)
	if err != nil {
		log.Error(errors.Wrapf(err, "unable to get country by ip: %v", ip))

		return ""
	}

	if Validate(results.Country_short) != nil {
		return ""
	}

	return results.Country_short
}

func Validate(country string) error {
	if !countries[country] {
		return errors.Errorf("country invalid: %v", country)
	}

	return nil
}
