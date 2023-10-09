// SPDX-License-Identifier: ice License 1.0

package devicemetadata

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/ip2location/ip2location-go/v9"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:gochecknoinits // We're just transforming the embedded files into useful data types.
func init() {
	var countryArray []*country
	//nolint:revive // That's the point.
	log.Panic(json.UnmarshalContext(context.Background(), []byte(countriesJSON), &countryArray))
	countries = make(map[string]*country)
	for _, c := range countryArray {
		countries[c.IsoCode] = c
	}

	if len(countries) != 249 { //nolint:gomnd // We have 249 countries in ip2location
		log.Panic(errors.Errorf("invalid number of countries %v. Expected 249", len(countries)))
	}
}

func New(db *storage.DB, mb messagebroker.Client) DeviceMetadataRepository {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	repo := &repository{db: db, mb: mb, cfg: &cfg}
	if mb != nil && !cfg.SkipIP2LocationBinary {
		var err error
		repo.ip2LocationDB, err = ip2location.OpenDB(cfg.IP2LocationBinaryPath)
		log.Panic(errors.Wrap(err, "unable to open ip2location database"))
	}

	return repo
}

func (r *repository) DeleteAllDeviceMetadata(ctx context.Context, userID string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline ")
	}
	sql := `SELECT * FROM device_metadata WHERE user_id = $1`
	res, err := storage.Select[DeviceMetadata](ctx, r.db, sql, userID)
	if err != nil {
		return errors.Wrapf(err, "failed to select all device metadata for userID:%v", userID)
	}
	if len(res) == 0 {
		return nil
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(res))
	errChan := make(chan error, len(res))
	for ix := range res {
		go func(iix int) {
			defer wg.Done()
			errChan <- errors.Wrapf(r.deleteDeviceMetadata(ctx, res[iix]), "failed to deleteDeviceMetadata for %#v", res[iix])
		}(ix)
	}
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(res))
	for err := range errChan {
		errs = append(errs, err)
	}

	return multierror.Append(nil, errs...).ErrorOrNil() //nolint:wrapcheck // Not needed.
}

func (r *repository) deleteDeviceMetadata(ctx context.Context, dm *DeviceMetadata) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	sql := `DELETE FROM device_metadata WHERE user_id = $1 AND device_unique_id = $2` //nolint:goconst // No need to make a constant SQL query.
	if _, err := storage.Exec(ctx, r.db, sql, dm.UserID, dm.DeviceUniqueID); err != nil {
		return errors.Wrapf(err, "failed to delete device_metadata for id:%#v", &dm.ID)
	}
	snapshot := deviceMetadataSnapshot(dm, nil)
	if err := r.sendDeviceMetadataSnapshotMessage(ctx, snapshot); err != nil {
		return errors.Wrapf(err, "failed to sendDeviceMetadataSnapshotMessage for %#v", snapshot)
	}
	if err := r.sendTombstonedDeviceMetadataMessage(ctx, &dm.ID); err != nil {
		return errors.Wrapf(err, "failed to sendTombstonedDeviceMetadataMessage for %#v", &dm.ID)
	}

	return nil
}

func (r *repository) Close() error {
	if r.ip2LocationDB != nil {
		r.ip2LocationDB.Close()
	}

	return nil
}

func (r *repository) GetDeviceMetadataLocation(ctx context.Context, deviceID *device.ID, clientIP net.IP) *DeviceLocation {
	if ctx.Err() != nil {
		log.Error(errors.Wrapf(ctx.Err(), "context error for GetDeviceMetadataLocation for %#v", deviceID))

		return new(DeviceLocation)
	}
	//nolint:godox // .
	// TODO: TBD if we need to use deviceID.DeviceUniqueID and/or deviceID.UserID to find some default/preferred value for the user.

	result, err := r.ip2LocationDB.Get_all(clientIP.String()) //nolint:nosnakecase // External library.
	if err != nil {
		log.Error(errors.Wrapf(err, "unable to get country&city for %#v, %v", deviceID, clientIP.String()))

		return new(DeviceLocation)
	}

	return &DeviceLocation{
		Country: strings.ToUpper(result.Country_short), //nolint:nosnakecase // External library.
		City:    result.City,
	}
}

func (r *repository) GetDeviceMetadata(ctx context.Context, id *device.ID) (*DeviceMetadata, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	sql := `SELECT * FROM device_metadata WHERE user_id = $1 AND device_unique_id = $2`
	dm, err := storage.Get[DeviceMetadata](ctx, r.db, sql, id.UserID, id.DeviceUniqueID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get device metadata by id: %#v", id)
	}
	if dm.ID.UserID == "" {
		return nil, storage.ErrNotFound
	}

	return dm, nil
}

func (r *repository) ReplaceDeviceMetadata(ctx context.Context, input *DeviceMetadata, clientIP net.IP) (err error) { //nolint:funlen // Big rollback logic.
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	if vErr := r.verifyDeviceAppVersion(input); vErr != nil {
		return vErr
	}
	if input.UserID == "" || input.UserID == "-" {
		return nil
	}
	input.UpdatedAt = time.Now()
	var ip2locationRecord ip2location.IP2Locationrecord
	if ip2locationRecord, err = r.ip2LocationDB.Get_all(clientIP.String()); err != nil { //nolint:nosnakecase // External library.
		return errors.Wrapf(err, "failed to get location information based on IP %v to replace device metadata", clientIP.String())
	}
	(&input.ip2LocationRecord).convertIP2Location(&ip2locationRecord)
	before, err := r.GetDeviceMetadata(ctx, &input.ID)
	if err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to get current device metadata for %#v", input.ID)
	}

	sql, args := input.replaceSQL()
	if _, err = storage.Exec(ctx, r.db, sql, args...); err != nil {
		return errors.Wrapf(err, "failed to replace device's %#v metadata", input.ID)
	}
	dm := deviceMetadataSnapshot(before, input)
	if err = r.sendDeviceMetadataSnapshotMessage(ctx, dm); err != nil {
		var revertErr error
		if before == nil {
			sql = "DELETE FROM device_metadata WHERE user_id = $1 AND device_unique_id = $2"
			_, revertErr = storage.Exec(ctx, r.db, sql, input.UserID, input.DeviceUniqueID)
			revertErr = errors.Wrapf(revertErr, "failed to delete device metadata due to rollback for %#v", input)
		} else {
			sql, args = before.replaceSQL()
			_, revertErr = storage.Exec(ctx, r.db, sql, args...)
			revertErr = errors.Wrapf(revertErr, "failed to replace to before, due to a rollback for %#v", input)
		}

		return multierror.Append(errors.Wrapf(err, "failed to send device metadata snapshot message %#v", dm), revertErr).ErrorOrNil() //nolint:wrapcheck // .
	}

	return nil
}

func (r *repository) verifyDeviceAppVersion(metadata *DeviceMetadata) error {
	readableParts := strings.Split(metadata.ReadableVersion, ".")
	if len(readableParts) < 1+1+1 {
		return errors.Wrapf(ErrInvalidAppVersion, "invalid version %v", metadata.ReadableVersion)
	}
	version := strings.ReplaceAll(fmt.Sprintf("v%v.%v.%v", readableParts[0], readableParts[1], readableParts[2]), "vv", "v")
	if semver.Compare(version, r.cfg.RequiredAppVersion) < 0 {
		return errors.Wrapf(ErrOutdatedAppVersion,
			"mobile app version %v is older than the required one %v, please update", metadata.ReadableVersion, r.cfg.RequiredAppVersion)
	}

	return errors.Wrapf(r.verifyDeviceAppNanosVersion(readableParts),
		"mobile app version %v is older than the required one %v, please update", metadata.ReadableVersion, r.cfg.RequiredAppVersion)
}

func (r *repository) verifyDeviceAppNanosVersion(readableParts []string) error {
	requiredParts := strings.Split(r.cfg.RequiredAppVersion, ".")
	if len(requiredParts) > 1+1+1 && len(readableParts) == 1+1+1 {
		return errors.Wrapf(ErrOutdatedAppVersion,
			"mobile app version doesn't contain nanos that is required %v, please update", r.cfg.RequiredAppVersion)
	}
	if len(requiredParts) > 1+1+1 && len(readableParts) > 1+1+1 {
		readableNano, err := strconv.Atoi(readableParts[3])
		log.Panic(err) //nolint:revive // Wrong, it's reachable.
		requiredNano, err := strconv.Atoi(requiredParts[3])
		log.Panic(err)

		if readableNano != 0 && readableNano != 1 && readableNano < requiredNano {
			return errors.Wrapf(ErrOutdatedAppVersion,
				"mobile app nanos version %v is older than the required one %v, please update", readableNano, requiredNano)
		}
	}

	return nil
}

//nolint:funlen // A lot of fields here.
func (dm *DeviceMetadata) replaceSQL() (string, []any) {
	var firstInstallTime, lastUpdateTime *stdlibtime.Time
	if dm.FirstInstallTime != nil {
		firstInstallTime = dm.FirstInstallTime.Time
	}
	if dm.LastUpdateTime != nil {
		lastUpdateTime = dm.LastUpdateTime.Time
	}

	sql := `INSERT INTO DEVICE_METADATA (
				country_short,
				country_long,
				region,
				city,
				isp,
				latitude,
				longitude,
				domain,
				zipcode,
				timezone,
				net_speed,
				idd_code,
				area_code,
				weather_station_code,
				weather_station_name,
				mcc,
				mnc,
				mobile_brand,
				elevation,
				usage_type,
				updated_at,
				first_install_time,
				last_update_time,
				user_id,
				device_unique_id,
				readable_version,
				fingerprint,
				instance_id,
				hardware,
				product,
				device,
				type,
				tags,
				device_id,
				device_type,
				device_name,
				brand,
				carrier,
				manufacturer,
				user_agent,
				system_name,
				system_version,
				base_os,
				build_id,
				bootloader,
				codename,
				installer_package_name,
				push_notification_token,
				device_timezone,
				api_level,
				tablet,
				pin_or_fingerprint_set,
				emulator
			) VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12,
				$13,
				$14,
				$15,
				$16,
				$17,
				$18,
				$19,
				$20,
				$21,
				$22,
				$23,
				$24,
				$25,
				$26,
				$27,
				$28,
				$29,
				$30,
				$31,
				$32,
				$33,
				$34,
				$35,
				$36,
				$37,
				$38,
				$39,
				$40,
				$41,
				$42,
				$43,
				$44,
				$45,
				$46,
				$47,
				$48,
				$49,
				$50,
				$51,
				$52,
				$53
			)
			ON CONFLICT(user_id, device_unique_id)
				DO UPDATE
					SET 
						country_short 			= EXCLUDED.country_short,
						country_long 			= EXCLUDED.country_long,
						region 					= EXCLUDED.region,
						city 					= EXCLUDED.city,
						isp 					= EXCLUDED.isp,
						latitude 				= EXCLUDED.latitude,
						longitude 				= EXCLUDED.longitude,
						domain 					= EXCLUDED.domain,
						zipcode 				= EXCLUDED.zipcode,
						timezone 				= EXCLUDED.timezone,
						net_speed 				= EXCLUDED.net_speed,
						idd_code 				= EXCLUDED.idd_code,
						area_code 				= EXCLUDED.area_code,
						weather_station_code 	= EXCLUDED.weather_station_code,
						weather_station_name 	= EXCLUDED.weather_station_name,
						mcc 					= EXCLUDED.mcc,
						mnc 					= EXCLUDED.mnc,
						mobile_brand 			= EXCLUDED.mobile_brand,
						elevation 				= EXCLUDED.elevation,
						usage_type 				= EXCLUDED.usage_type,
						updated_at 				= EXCLUDED.updated_at,
						first_install_time 		= EXCLUDED.first_install_time,
						last_update_time 		= EXCLUDED.last_update_time,
						readable_version 		= EXCLUDED.readable_version,
						fingerprint 			= EXCLUDED.fingerprint,
						instance_id 			= EXCLUDED.instance_id,
						hardware 				= EXCLUDED.hardware,
						product 				= EXCLUDED.product,
						device 					= EXCLUDED.device,
						type 					= EXCLUDED.type,
						tags 					= EXCLUDED.tags,
						device_id 				= EXCLUDED.device_id,
						device_type 			= EXCLUDED.device_type,
						device_name 			= EXCLUDED.device_name,
						brand 					= EXCLUDED.brand,
						carrier 				= EXCLUDED.carrier,
						manufacturer 			= EXCLUDED.manufacturer,
						user_agent 				= EXCLUDED.user_agent,
						system_name 			= EXCLUDED.system_name,
						system_version 			= EXCLUDED.system_version,
						base_os 				= EXCLUDED.base_os,
						build_id 				= EXCLUDED.build_id,
						bootloader 				= EXCLUDED.bootloader,
						codename 				= EXCLUDED.codename,
						installer_package_name 	= EXCLUDED.installer_package_name,
						push_notification_token = EXCLUDED.push_notification_token,
						device_timezone 		= EXCLUDED.device_timezone,
						api_level 				= EXCLUDED.api_level,
						tablet 					= EXCLUDED.tablet,
						pin_or_fingerprint_set 	= EXCLUDED.pin_or_fingerprint_set,
						emulator 				= EXCLUDED.emulator
				WHERE 	 COALESCE(DEVICE_METADATA.country_short, '') 				   != coalesce(EXCLUDED.country_short, '')
					  OR COALESCE(DEVICE_METADATA.country_long, '') 			 	   != coalesce(EXCLUDED.country_long, '')
					  OR COALESCE(DEVICE_METADATA.region, '') 					 	   != coalesce(EXCLUDED.region, '')
					  OR COALESCE(DEVICE_METADATA.city, '') 					 	   != coalesce(EXCLUDED.city, '')
					  OR COALESCE(DEVICE_METADATA.isp, '') 						 	   != coalesce(EXCLUDED.isp, '')
					  OR COALESCE(DEVICE_METADATA.latitude, 0)	 				 	   != coalesce(EXCLUDED.latitude, 0)
					  OR COALESCE(DEVICE_METADATA.longitude, 0) 				 	   != coalesce(EXCLUDED.longitude, 0)
					  OR COALESCE(DEVICE_METADATA.domain, '') 					 	   != coalesce(EXCLUDED.domain, '')
					  OR COALESCE(DEVICE_METADATA.zipcode, '') 					 	   != coalesce(EXCLUDED.zipcode, '')
					  OR COALESCE(DEVICE_METADATA.timezone, '') 				 	   != coalesce(EXCLUDED.timezone, '')
					  OR COALESCE(DEVICE_METADATA.net_speed, '') 				 	   != coalesce(EXCLUDED.net_speed, '')
					  OR COALESCE(DEVICE_METADATA.idd_code, '') 				 	   != coalesce(EXCLUDED.idd_code, '')
					  OR COALESCE(DEVICE_METADATA.area_code, '') 				 	   != coalesce(EXCLUDED.area_code, '')
					  OR COALESCE(DEVICE_METADATA.weather_station_code, '') 	 	   != coalesce(EXCLUDED.weather_station_code, '')
					  OR COALESCE(DEVICE_METADATA.weather_station_name, '') 	 	   != coalesce(EXCLUDED.weather_station_name, '')
					  OR COALESCE(DEVICE_METADATA.mcc, '') 						 	   != coalesce(EXCLUDED.mcc, '')
					  OR COALESCE(DEVICE_METADATA.mnc, '') 						 	   != coalesce(EXCLUDED.mnc, '')
					  OR COALESCE(DEVICE_METADATA.mobile_brand, '') 			 	   != coalesce(EXCLUDED.mobile_brand, '')
					  OR COALESCE(DEVICE_METADATA.elevation, 0) 				  	   != coalesce(EXCLUDED.elevation, 0)
					  OR COALESCE(DEVICE_METADATA.usage_type, '') 				 	   != coalesce(EXCLUDED.usage_type, '')
					  OR COALESCE(DEVICE_METADATA.updated_at, to_timestamp(0)) 	 	   != coalesce(EXCLUDED.updated_at, to_timestamp(0))
					  OR COALESCE(DEVICE_METADATA.first_install_time, to_timestamp(0)) != coalesce(EXCLUDED.first_install_time, to_timestamp(0))
					  OR COALESCE(DEVICE_METADATA.last_update_time, to_timestamp(0))   != coalesce(EXCLUDED.last_update_time, to_timestamp(0))
					  OR COALESCE(DEVICE_METADATA.readable_version, '') 			   != coalesce(EXCLUDED.readable_version, '')
					  OR COALESCE(DEVICE_METADATA.fingerprint, '') 				 	   != coalesce(EXCLUDED.fingerprint, '')
					  OR COALESCE(DEVICE_METADATA.instance_id, '') 				 	   != coalesce(EXCLUDED.instance_id, '')
					  OR COALESCE(DEVICE_METADATA.hardware, '') 				 	   != coalesce(EXCLUDED.hardware, '')
					  OR COALESCE(DEVICE_METADATA.product, '') 					 	   != coalesce(EXCLUDED.product, '')
					  OR COALESCE(DEVICE_METADATA.device, '') 					 	   != coalesce(EXCLUDED.device, '')
					  OR COALESCE(DEVICE_METADATA.type, '') 					 	   != coalesce(EXCLUDED.type, '')
					  OR COALESCE(DEVICE_METADATA.tags, '') 					 	   != coalesce(EXCLUDED.tags, '')
					  OR COALESCE(DEVICE_METADATA.device_id, '') 				 	   != coalesce(EXCLUDED.device_id, '')
					  OR COALESCE(DEVICE_METADATA.device_type, '') 				 	   != coalesce(EXCLUDED.device_type, '')
					  OR COALESCE(DEVICE_METADATA.device_name, '') 				 	   != coalesce(EXCLUDED.device_name, '')
					  OR COALESCE(DEVICE_METADATA.brand, '') 					 	   != coalesce(EXCLUDED.brand, '')
					  OR COALESCE(DEVICE_METADATA.carrier, '') 					 	   != coalesce(EXCLUDED.carrier, '')
					  OR COALESCE(DEVICE_METADATA.manufacturer, '') 			 	   != coalesce(EXCLUDED.manufacturer, '')
					  OR COALESCE(DEVICE_METADATA.user_agent, '') 				 	   != coalesce(EXCLUDED.user_agent, '')
					  OR COALESCE(DEVICE_METADATA.system_name, '') 				 	   != coalesce(EXCLUDED.system_name, '')
					  OR COALESCE(DEVICE_METADATA.system_version, '') 			 	   != coalesce(EXCLUDED.system_version, '')
					  OR COALESCE(DEVICE_METADATA.base_os, '') 					 	   != coalesce(EXCLUDED.base_os, '')
					  OR COALESCE(DEVICE_METADATA.build_id, '') 				 	   != coalesce(EXCLUDED.build_id, '')
					  OR COALESCE(DEVICE_METADATA.bootloader, '') 				 	   != coalesce(EXCLUDED.bootloader, '')
					  OR COALESCE(DEVICE_METADATA.codename, '') 				 	   != coalesce(EXCLUDED.codename, '')
					  OR COALESCE(DEVICE_METADATA.installer_package_name, '') 	 	   != coalesce(EXCLUDED.installer_package_name, '')
					  OR COALESCE(DEVICE_METADATA.push_notification_token, '') 	 	   != coalesce(EXCLUDED.push_notification_token, '')
					  OR COALESCE(DEVICE_METADATA.device_timezone, '') 			 	   != coalesce(EXCLUDED.device_timezone, '')
					  OR COALESCE(DEVICE_METADATA.api_level, 0) 				 	   != coalesce(EXCLUDED.api_level, 0)
					  OR COALESCE(DEVICE_METADATA.tablet, FALSE) 				 	   != coalesce(EXCLUDED.tablet, FALSE)
					  OR COALESCE(DEVICE_METADATA.pin_or_fingerprint_set, FALSE) 	   != coalesce(EXCLUDED.pin_or_fingerprint_set, FALSE)
					  OR COALESCE(DEVICE_METADATA.emulator, FALSE) 				 	   != coalesce(EXCLUDED.emulator, FALSE)`
	args := []any{
		dm.CountryShort,
		dm.CountryLong,
		dm.Region,
		dm.City,
		dm.Isp,
		dm.Latitude,
		dm.Longitude,
		dm.Domain,
		dm.Zipcode,
		dm.Timezone,
		dm.Netspeed,
		dm.Iddcode,
		dm.Areacode,
		dm.Weatherstationcode,
		dm.Weatherstationname,
		dm.Mcc,
		dm.Mnc,
		dm.Mobilebrand,
		dm.Elevation,
		dm.Usagetype,
		dm.UpdatedAt.Time,
		firstInstallTime,
		lastUpdateTime,
		dm.UserID,
		dm.DeviceUniqueID,
		dm.ReadableVersion,
		dm.Fingerprint,
		dm.InstanceID,
		dm.Hardware,
		dm.Product,
		dm.Device,
		dm.Type,
		dm.Tags,
		dm.DeviceID,
		dm.DeviceType,
		dm.DeviceName,
		dm.Brand,
		dm.Carrier,
		dm.Manufacturer,
		dm.UserAgent,
		dm.SystemName,
		dm.SystemVersion,
		dm.BaseOS,
		dm.BuildID,
		dm.Bootloader,
		dm.Codename,
		dm.InstallerPackageName,
		dm.PushNotificationToken,
		dm.TZ,
		dm.APILevel,
		dm.Tablet,
		dm.PinOrFingerprintSet,
		dm.Emulator,
	}

	return sql, args
}

func (rec *ip2LocationRecord) convertIP2Location(ip *ip2location.IP2Locationrecord) {
	*rec = ip2LocationRecord{
		CountryShort:       ip.Country_short, //nolint:nosnakecase // 3rd party library.
		CountryLong:        ip.Country_long,  //nolint:nosnakecase // 3rd party library.
		Region:             ip.Region,
		City:               ip.City,
		Isp:                ip.Isp,
		Domain:             ip.Domain,
		Zipcode:            ip.Zipcode,
		Timezone:           ip.Timezone,
		Netspeed:           ip.Netspeed,
		Iddcode:            ip.Iddcode,
		Areacode:           ip.Areacode,
		Weatherstationcode: ip.Weatherstationcode,
		Weatherstationname: ip.Weatherstationname,
		Mcc:                ip.Mcc,
		Mnc:                ip.Mnc,
		Mobilebrand:        ip.Mobilebrand,
		Usagetype:          ip.Usagetype,
		Elevation:          float64(ip.Elevation),
		Latitude:           float64(ip.Latitude),
		Longitude:          float64(ip.Longitude),
	}
}

func deviceMetadataSnapshot(before, after *DeviceMetadata) *DeviceMetadataSnapshot {
	var before2, after2 *DeviceMetadata
	if before != nil {
		before2 = &DeviceMetadata{
			ID:                    before.ID,
			PushNotificationToken: before.PushNotificationToken,
			TZ:                    before.ip2LocationRecord.Timezone,
		}
		if before.TZ != "" {
			before2.TZ = before.TZ
		}
	}
	if after != nil {
		after2 = &DeviceMetadata{
			ID:                    after.ID,
			PushNotificationToken: after.PushNotificationToken,
			TZ:                    after.ip2LocationRecord.Timezone,
		}
		if after.TZ != "" {
			after2.TZ = after.TZ
		}
	}

	return &DeviceMetadataSnapshot{DeviceMetadata: after2, Before: before2}
}

func (r *repository) sendTombstonedDeviceMetadataMessage(ctx context.Context, did *device.ID) error {
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     did.UserID + "~~~" + did.DeviceUniqueID,
		Topic:   r.cfg.MessageBroker.Topics[2].Name,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send tombstoned device metadata message to broker")
}

func (r *repository) sendDeviceMetadataSnapshotMessage(ctx context.Context, dm *DeviceMetadataSnapshot) error {
	valueBytes, err := json.MarshalContext(ctx, dm)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal DeviceMetadata %#v", dm)
	}
	var did *device.ID
	if dm.DeviceMetadata != nil {
		did = &dm.DeviceMetadata.ID
	} else {
		did = &dm.Before.ID
	}
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     did.UserID + "~~~" + did.DeviceUniqueID,
		Topic:   r.cfg.MessageBroker.Topics[2].Name,
		Value:   valueBytes,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send device metadata snapshot message to broker")
}

func (*repository) LookupCountries(keyword Keyword) []Country {
	kw := strings.ToUpper(keyword)
	matchingCountries := make([]Country, 0)
	for countryCode, c := range countries {
		if strings.Contains(countryCode, kw) ||
			strings.Contains(strings.ToUpper(c.Name), kw) {
			matchingCountries = append(matchingCountries, countryCode)
		}
	}

	return matchingCountries
}

func (*repository) IsValid(c Country) bool {
	_, found := countries[strings.ToUpper(c)]

	return found
}

func RandomCountry() Country {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(countries))))
	log.Panic(errors.Wrap(err, "crypto random generator failed")) //nolint:revive // Should never happen.

	currentIdx := uint64(0)
	desiredIdx := n.Uint64()
	for k := range countries {
		if currentIdx == desiredIdx {
			return k
		}
		currentIdx++
	}

	return ""
}
