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
	countries = make(map[string]bool)

	for _, a := range v {
		countries[strings.ToLower(a)] = true
	}
}

func New(ctx context.Context) Repository {
	log.Panic(errors.Wrap(ctx.Err(), "context error"))

	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db, err := ip2location.OpenDB(cfg.BinaryLocation)
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
		log.Error(errors.Wrap(err, "unable to get country by ip"))

		return ""
	}

	if Validate(results.Country_short) != nil {
		return ""
	}

	return results.Country_short
}

func Validate(country string) error {
	if !countries[country] {
		return errors.New("country invalid")
	}

	return nil
}
