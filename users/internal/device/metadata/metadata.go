// SPDX-License-Identifier: BUSL-1.1

package devicemetadata

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/framey-io/go-tarantool"
	"github.com/ip2location/ip2location-go"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:gochecknoinits // We're just transforming the embedded files into useful data types.
func init() {
	var countryArray []*country
	log.Panic(json.Unmarshal([]byte(countriesJSON), &countryArray))
	countries = make(map[string]*country)
	for _, c := range countryArray {
		countries[c.IsoCode] = c
	}

	if len(countries) != 249 { //nolint:gomnd // We have 249 countries in ip2location
		log.Panic(errors.Errorf("invalid number of countries %v. Expected 249", len(countries)))
	}
}

func New(db tarantool.Connector, mb messagebroker.Client) (r DeviceMetadataRepository) {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	r = &repository{db: db, mb: mb}
	if mb != nil {
		var err error
		r.(*repository).ip2LocationDB, err = ip2location.OpenDB(cfg.IP2LocationBinaryPath)
		log.Panic(errors.Wrap(err, "unable to open ip2location database"))
	}

	return r
}

func (r *repository) Close() error {
	if r.ip2LocationDB != nil {
		r.ip2LocationDB.Close()
	}

	return nil
}

func (r *repository) GetDeviceCountry(ctx context.Context, ip net.IP) Country {
	if ctx.Err() != nil {
		log.Error(errors.Wrap(ctx.Err(), "context error"))

		return ""
	}

	result, err := r.ip2LocationDB.Get_country_short(ip.String())
	if err != nil {
		log.Error(errors.Wrapf(err, "unable to get country by ip: %v", ip.String()))

		return ""
	}

	return strings.ToUpper(result.Country_short)
}

func (r *repository) GetDeviceMetadata(ctx context.Context, id device.ID) (*DeviceMetadata, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	dm := new(DeviceMetadata)
	if err := r.db.GetTyped("DEVICE_METADATA", "pk_unnamed_DEVICE_METADATA_1", id, dm); err != nil {
		return nil, errors.Wrapf(err, "failed to get device metadata by id: %#v", id)
	}
	if dm.ID.UserID == "" {
		return nil, storage.ErrNotFound
	}

	return dm, nil
}

func (r *repository) ReplaceDeviceMetadata(ctx context.Context, arg *ReplaceDeviceMetadataArg) (err error) {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	ip := arg.ClientIP
	metadata := new(deviceMetadata)
	metadata.UpdatedAt = time.Now()
	metadata.DeviceMetadata = arg.DeviceMetadata
	if metadata.IP2Locationrecord, err = r.ip2LocationDB.Get_all(ip.String()); err != nil {
		return errors.Wrapf(err, "failed to get location information based on IP %v to replace device metadata", ip.String())
	}
	var result []*DeviceMetadata
	if err = r.db.ReplaceTyped("DEVICE_METADATA", metadata, &result); err != nil {
		return errors.Wrapf(err, "failed to replace device's %#v metadata", metadata.ID)
	}
	// Because we don't care about the other fields, for now.
	dm := &DeviceMetadata{ID: metadata.ID, PushNotificationToken: metadata.PushNotificationToken}

	return errors.Wrap(r.sendDeviceMetadataMessage(ctx, dm), "failed to send device metadata message")
}

func (r *repository) sendDeviceMetadataMessage(ctx context.Context, dm *DeviceMetadata) error {
	valueBytes, err := json.Marshal(dm)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal DeviceMetadata %#v", dm)
	}
	m := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     dm.UserID + "~" + dm.DeviceUniqueID,
		Topic:   cfg.MessageBroker.Topics[1].Name,
		Value:   valueBytes,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, m, responder)

	return errors.Wrapf(<-responder, "failed to send device metadata message to broker")
}

func (r *repository) LookupCountries(keyword Keyword) []Country {
	keyword = strings.ToUpper(keyword)
	matchingCountries := make([]Country, 0)
	for countryCode, c := range countries {
		if strings.Contains(countryCode, keyword) ||
			strings.Contains(strings.ToUpper(c.Name), keyword) {
			matchingCountries = append(matchingCountries, countryCode)
		}
	}

	return matchingCountries
}

func (r *repository) IsValid(c Country) bool {
	_, found := countries[strings.ToUpper(c)]

	return found
}

func (r *repository) GetDeviceMetadataLocation(ctx context.Context, arg *GetDeviceMetadataLocationArg) *DeviceLocation {
	if arg.UserID != "" {
		//nolint:godox // .
		//TODO: TBD if we need to implement this.
		return &DeviceLocation{Country: "UNKNOWN"}
	}
	//nolint:godox // .
	//TODO: TBD if we need to use arg.DeviceUniqueID to find some default/preferred value for the user.

	return &DeviceLocation{Country: r.GetDeviceCountry(ctx, arg.ClientIP)}
}
