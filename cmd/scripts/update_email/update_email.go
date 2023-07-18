// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/auth/fixture"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

const (
	applicationYamlEskimoKey = "users"
	applicationYamlAuthKey   = "auth/email-link"

	defaultLimit     = 10000
	concurrencyCount = 1000
)

var (
	//go:embed DDL.sql
	ddl string

	errMetadataMismatch      = errors.New("metadata mismatch")
	errFirebaseEmailMismatch = errors.New("firebase emails mismatch")
)

type (
	record struct {
		HandledAt     *time.Time `json:"handledAt" example:"2022-01-03T16:20:52.156534Z"`
		ID            string     `json:"id" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber   string     `json:"phoneNumber" example:"+123456789"`
		UserEmail     string     `json:"userEmail" example:"example@gmail.com"`
		ToUpdateEmail string     `json:"toUpdateEmail" example:"example@gmail.com"`
	}
)

//nolint:funlen // Concurrency logic.
func main() {
	var argOffset uint64
	flag.Uint64Var(&argOffset, "offset", 0, "starting offset")
	flag.Parse()

	usersProcessor := users.StartProcessor(context.Background(), func() {})
	authClient := auth.New(context.Background(), applicationYamlAuthKey)
	authEmailLinkClient := emaillink.NewClient(context.Background(), usersProcessor, authClient)
	db := storage.MustConnect(context.Background(), ddl, applicationYamlEskimoKey)
	defer db.Close()
	defer usersProcessor.Close()
	defer authEmailLinkClient.Close()

	totalCount := getUsersForUpdateCount(db)
	offset := uint64(argOffset)
	concurrencyGuard := make(chan struct{}, concurrencyCount)

	wg := new(sync.WaitGroup)
	for context.Background().Err() == nil {
		records := getUsersForUpdate(db, defaultLimit, offset)
		if len(records) == 0 {
			log.Info("nothing to handle")

			break
		}
		wg.Add(len(records))
		for idx, record := range records {
			index := uint64(idx) + offset
			if record.HandledAt != nil || record.UserEmail == record.ToUpdateEmail {
				continue
			}
			concurrencyGuard <- struct{}{}
			usr := record
			go func() {
				defer wg.Done()
				updateDBEmail(usersProcessor, usr, index)
				updateMetadata(authEmailLinkClient, usr, index)
				updateFirebaseEmail(authClient, usr, index)
				markAsHandled(db, usr.ToUpdateEmail, index)
				<-concurrencyGuard
			}()
		}
		log.Info(fmt.Sprintf("rows processed %v/%v", len(records), totalCount))
		offset += defaultLimit
	}
	wg.Wait()
}

func updateDBEmail(usersProcessor users.Processor, usr *record, idx uint64) {
	updUsr := users.User{
		PublicUserInformation: users.PublicUserInformation{
			ID: usr.ID,
		},
		PrivateUserInformation: users.PrivateUserInformation{
			SensitiveUserInformation: users.SensitiveUserInformation{
				Email: usr.ToUpdateEmail,
			},
		},
	}
	log.Panic(errors.Wrapf(usersProcessor.ModifyUser(context.Background(), &updUsr, nil), "can't modify eskimo user: %#v, idx:%v", updUsr, idx))
}

func updateMetadata(authEmailLinkClient emaillink.Client, usr *record, idx uint64) {
	md := users.JSON(map[string]any{
		auth.RegisteredWithProviderClaim: auth.ProviderFirebase,
		auth.FirebaseIDClaim:             usr.ID,
	})
	_, err := authEmailLinkClient.UpdateMetadata(context.Background(), usr.ID, &md)
	log.Panic(errors.Wrapf(err, "can't update metadata for userID:%v, phoneNumber:%v, idx:%v", usr.ID, usr.PhoneNumber, idx)) //nolint:revive // Wrong.
	_, mdJSON, err := authEmailLinkClient.Metadata(context.Background(), usr.ID, usr.ToUpdateEmail)
	log.Panic(errors.Wrapf(err, "can't get user's:%v metadata, idx:%v", usr.ID, idx))
	if mdJSON == nil || (*mdJSON)[auth.RegisteredWithProviderClaim] != auth.ProviderFirebase ||
		(*mdJSON)[auth.FirebaseIDClaim] != usr.ID {
		log.Panic(errors.Wrapf(errMetadataMismatch, "metadata mismatch, metadata:%#v, added:%#v, idx:%v", mdJSON, md, idx))
	}
}

func updateFirebaseEmail(authClient auth.Client, usr *record, idx uint64) {
	log.Panic(authClient.UpdateEmail(context.Background(), usr.ID, usr.ToUpdateEmail), "can't update firebase email for userID:%v, email:%v, idx:%v", usr.ID, usr.ToUpdateEmail, idx) //nolint:revive // Wrong.
	firebaseUsr, err := fixture.GetUser(context.Background(), usr.ID)
	log.Panic(errors.Wrapf(err, "can't get user by id:%v from firebase, idx:%v", usr.ID, idx))
	if firebaseUsr.Email != usr.ToUpdateEmail {
		log.Panic(errors.Wrapf(errFirebaseEmailMismatch, "firebase emails mismatch, db:%v, firebase:%v, idx:%v", usr.ToUpdateEmail, firebaseUsr.Email, idx))
	}
}

func markAsHandled(db *storage.DB, email string, idx uint64) {
	sql := `UPDATE update_email_temporary 
				SET handled_at = $1
			WHERE email = $2`
	rowsUpdated, err := storage.Exec(context.Background(), db, sql, time.Now().Time, email)
	if rowsUpdated == 0 || err != nil {
		log.Panic(errors.Wrapf(err, "can't mark record as handled for email: %v, idx:%v", email, idx))
	}
}

func getUsersForUpdate(db *storage.DB, limit, offset uint64) []*record {
	params := []any{limit, offset}
	sql := `SELECT 
				u.id,
				u.phone_number,
				u.email   AS user_email,
				upd.email AS to_update_email,
				upd.handled_at
			FROM users u
				JOIN update_email_temporary upd
					ON upd.phone_number = u.phone_number
			WHERE handled_at IS NULL
			ORDER BY u.created_at ASC
			LIMIT $1 OFFSET $2`
	result, err := storage.Select[record](context.Background(), db, sql, params...)
	log.Panic(errors.Wrapf(err, "can't select records for limit:%v, offset:%v", limit, offset)) //nolint:revive // Wrong.
	if result == nil {
		return []*record{}
	}

	return result
}

func getUsersForUpdateCount(db *storage.DB) uint64 {
	type count struct {
		Count uint64
	}
	sql := `SELECT 
				COUNT(*)
			FROM users u
				JOIN update_email_temporary upd
					ON upd.phone_number = u.phone_number`
	result, err := storage.Select[count](context.Background(), db, sql)
	if err != nil || len(result) == 0 {
		log.Panic(errors.Wrapf(err, "can't select records count"))
	}

	return result[0].Count
}
