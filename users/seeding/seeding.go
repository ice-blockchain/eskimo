// SPDX-License-Identifier: ice License 1.0

//go:build !test

package seeding

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	stdlibtime "time"

	"github.com/google/uuid"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/eskimo/users"
	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	"github.com/ice-blockchain/go-tarantool-client"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func StartSeeding() {
	before := stdlibtime.Now()
	db := dbConnector()
	defer func() {
		log.Panic(db.Close()) //nolint:revive // It doesnt really matter.
		log.Info(fmt.Sprintf("seeding finalized in %v", stdlibtime.Since(before).String()))
	}()

	for _, usr := range findRealUsers(db) {
		for i := 0; i < 100; i++ {
			t1 := createReferral(db, usr.UserID)
			makeReferralAPhoneAgendaContact(db, t1, usr.UserID)
			for j := 0; j < 100; j++ {
				makeReferralAPhoneAgendaContact(db, createReferral(db, t1), usr.UserID)
			}
		}
	}
	createGlobalStats(db)

	ctx, cancel := context.WithTimeout(context.Background(), 10*stdlibtime.Second) //nolint:gomnd // It doesnt matter here.
	defer cancel()
	for ctx.Err() == nil {
		createUsersPerCountry(db)
	}
}

//nolint:gosec,funlen // Not an issue.
func createGlobalStats(db tarantool.Connector) {
	const maxCount = 1_000_000_000
	const totalHours = 36000                                           // That's 2 years in the past + 2 years in the future.
	globalKeys := make(map[string]int, 2*totalHours+(totalHours/11)+1) //nolint:gomnd // .
	globalKeys["TOTAL_USERS"] = rand.Intn(maxCount)
	nowNanos := time.Now().UnixNano()
	for i := totalHours / 2; i > 0; i-- { //nolint:gomnd // .
		pastHour := stdlibtime.Unix(0, nowNanos).Add(stdlibtime.Duration(-i) * stdlibtime.Hour)
		globalKeys[fmt.Sprintf("TOTAL_ACTIVE_USERS_%v", pastHour.Format("2006-01-02:15"))] = rand.Intn(maxCount)
		globalKeys[fmt.Sprintf("TOTAL_USERS_%v", pastHour.Format("2006-01-02:15"))] = rand.Intn(maxCount)
		globalKeys[fmt.Sprintf("TOTAL_ACTIVE_USERS_%v", pastHour.Format("2006-01-02"))] = rand.Intn(maxCount)
		globalKeys[fmt.Sprintf("TOTAL_USERS_%v", pastHour.Format("2006-01-02"))] = rand.Intn(maxCount)
	}
	for i := 0; i < totalHours/2; i++ {
		futureHour := stdlibtime.Unix(0, nowNanos).Add(stdlibtime.Duration(i) * stdlibtime.Hour)
		globalKeys[fmt.Sprintf("TOTAL_ACTIVE_USERS_%v", futureHour.Format("2006-01-02:15"))] = rand.Intn(maxCount)
		globalKeys[fmt.Sprintf("TOTAL_USERS_%v", futureHour.Format("2006-01-02:15"))] = rand.Intn(maxCount)
		globalKeys[fmt.Sprintf("TOTAL_ACTIVE_USERS_%v", futureHour.Format("2006-01-02"))] = rand.Intn(maxCount)
		globalKeys[fmt.Sprintf("TOTAL_USERS_%v", futureHour.Format("2006-01-02"))] = rand.Intn(maxCount)
	}

	for key, val := range globalKeys {
		log.Panic(db.UpsertTyped("GLOBAL", &struct {
			_msgpack struct{} `msgpack:",asArray"` //nolint:revive,tagliatelle,nosnakecase // To insert we need asArray
			Key      string
			Value    uint64
		}{
			Key:   key,
			Value: uint64(val),
		}, []tarantool.Op{{
			Op:    "=",
			Field: 1,
			Arg:   uint64(val),
		}}, &[]*struct {
			_msgpack struct{} `msgpack:",asArray"` //nolint:revive,tagliatelle,nosnakecase // To insert we need asArray
			Key      string
			Value    uint64
		}{}))
	}
}

func dbConnector() tarantool.Connector {
	parts := strings.Split(os.Getenv("MASTER_DB_INSTANCE_ADDRESS"), "@")
	userAndPass := strings.Split(parts[0], ":")
	opts := tarantool.Opts{
		Timeout: 20 * stdlibtime.Second, //nolint:gomnd // It doesnt matter here.
		User:    userAndPass[0],
		Pass:    userAndPass[1],
	}
	db, err := tarantool.Connect(parts[1], opts)
	log.Panic(err)

	return db
}

func findRealUsers(db tarantool.Connector) (res []*struct{ UserID string }) {
	log.Panic(db.PrepareExecuteTyped(`SELECT id 
										FROM users
										WHERE email != id OR phone_number != id`, map[string]any{}, &res))

	return
}

func createReferral(db tarantool.Connector, referredBy string) string {
	//nolint:lll // .
	sql := `
	INSERT INTO users 
		(ID, HASH_CODE, EMAIL, PHONE_NUMBER, PHONE_NUMBER_HASH, USERNAME, REFERRED_BY, PROFILE_PICTURE_NAME, BLOCKCHAIN_ACCOUNT_ADDRESS, MINING_BLOCKCHAIN_ACCOUNT_ADDRESS, HIDDEN_PROFILE_ELEMENTS, COUNTRY, CITY, CREATED_AT, UPDATED_AT)
	VALUES
		(:id, :hashCode, :id, :id, :id, :username, :referredBy, :profilePictureName, :id, :id, :hiddenProfileElements, :country, :city, :createdAt, :createdAt)`
	now := time.New(time.Now().AddDate(0, 0, -rand.Intn(6))) //nolint:gomnd,gosec // Not an issue here.
	id := uuid.NewString()
	params := map[string]any{
		"id":                    id,
		"hashCode":              xxh3.HashStringSeed(id, uint64(now.UnixNano())),
		"username":              fmt.Sprintf("u%v", now.UnixNano()),
		"profilePictureName":    users.RandomDefaultProfilePictureName(),
		"hiddenProfileElements": users.RandomizeHiddenProfileElements(),
		"country":               devicemetadata.RandomCountry(),
		"city":                  fmt.Sprintf("bogusCity%v", uuid.NewString()),
		"createdAt":             now,
		"referredBy":            referredBy,
	}
	log.Panic(storage.CheckSQLDMLErr(db.PrepareExecute(sql, params)))

	return id
}

func makeReferralAPhoneAgendaContact(db tarantool.Connector, userID, referredBy string) {
	sql := `UPDATE users
			SET agenda_phone_number_hashes  = COALESCE(agenda_phone_number_hashes,'') || ',' || :phoneNumberHash
			WHERE id = :id`
	params := map[string]any{
		"id":              referredBy,
		"phoneNumberHash": userID,
	}
	log.Panic(storage.CheckSQLDMLErr(db.PrepareExecute(sql, params)))
}

func createUsersPerCountry(db tarantool.Connector) {
	sql := `REPLACE INTO users_per_country 
				(COUNTRY, USER_COUNT)
			VALUES
				(:country, :user_count)`
	params := map[string]any{
		"country":    devicemetadata.RandomCountry(),
		"user_count": uint64(rand.Intn(1000)), //nolint:gosec,gomnd // It doesn't matter here.
	}
	log.Panic(storage.CheckSQLDMLErr(db.PrepareExecute(sql, params)))
}
