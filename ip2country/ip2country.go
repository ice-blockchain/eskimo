package ip2country

import (
	"context"

	"github.com/ip2location/ip2location-go"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/log"
)

func New(_ context.Context) *IPDatabase {
	db, err := ip2location.OpenDB("./ipdb.bin")
	if err != nil {
		return &IPDatabase{}
	}

	return &IPDatabase{db: db}
}

func (i *IPDatabase) Close() error {
	return errors.Wrap(i.Close(), "error closing IP database")
}

func (i *IPDatabase) GetCountry(_ context.Context, ip string) string {
	results, err := i.db.Get_all(ip)
	if err != nil {
		log.Error(errors.Wrap(err, "unable to get country by ip"))

		return ""
	}

	return results.Country_short
}
