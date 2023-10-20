// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/auth/fixture"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
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
		PhoneNumber  string `json:"phoneNumber" example:"+123456789"`
		Email        string `json:"email" example:"example@gmail.com"`
		CurrentEmail string `json:"currentEmail" example:"example@gmail.com"`
		ID           string `json:"id" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
)

//nolint:funlen // Concurrency logic.
func main() {
	usersProcessor := users.StartProcessor(context.Background(), func() {})
	authClient := auth.New(context.Background(), applicationYamlAuthKey)
	authEmailLinkClient := emaillink.NewClient(context.Background(), usersProcessor, authClient)
	db := storage.MustConnect(context.Background(), ddl, applicationYamlEskimoKey)
	defer db.Close()
	defer usersProcessor.Close()
	defer authEmailLinkClient.Close()

	offset := uint64(0)
	concurrencyGuard := make(chan struct{}, concurrencyCount)
	wg := new(sync.WaitGroup)
	for {
		records := getUsersToMerge(db, defaultLimit, offset)
		if len(records) == 0 {
			break
		}
		for idx, record := range records {
			index := uint64(idx) + offset
			usr := record
			if usr.ID == "" {
				log.Error(errors.Errorf("no user with phone number `%v` found", usr.PhoneNumber))

				continue
			}
			if usr.CurrentEmail != usr.ID && usr.CurrentEmail != usr.Email {
				log.Error(errors.Errorf("user with phone number: `%v`, id: `%v` has a different email: `%v`", usr.PhoneNumber, usr.ID, usr.CurrentEmail))

				continue
			}
			wg.Add(1)
			concurrencyGuard <- struct{}{}
			go func() {
				defer wg.Done()
				updateDBEmail(usersProcessor, usr, index)
				updateMetadata(authEmailLinkClient, usr, index)
				updateFirebaseEmail(authClient, usr, index)
				<-concurrencyGuard
			}()
			log.Error(fmt.Errorf("rows processed %v/%v", index+1, len(records))) //nolint:goerr113 // .
		}
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
				Email: usr.Email,
			},
		},
	}
	err := usersProcessor.ModifyUser(context.Background(), &updUsr, nil)
	if errors.Is(err, users.ErrDuplicate) { //nolint:revive // Nope.
		log.Error(errors.Errorf("duplicate email(belongs to another user): id:%v, email:%v", usr.ID, usr.Email))
	} else {
		log.Panic(errors.Wrapf(err, "can't modify eskimo user: %#v, idx:%v", updUsr, idx))
	}
}

func updateMetadata(authEmailLinkClient emaillink.Client, usr *record, idx uint64) {
	md := users.JSON(map[string]any{
		auth.RegisteredWithProviderClaim: auth.ProviderFirebase,
		auth.FirebaseIDClaim:             usr.ID,
	})
	mdJSON, err := authEmailLinkClient.UpdateMetadata(context.Background(), usr.ID, &md)
	log.Panic(errors.Wrapf(err, "can't update metadata for userID:%v, idx:%v", usr.ID, idx)) //nolint:revive // Wrong.
	if mdJSON == nil || (*mdJSON)[auth.RegisteredWithProviderClaim] != auth.ProviderFirebase ||
		(*mdJSON)[auth.FirebaseIDClaim] != usr.ID {
		log.Panic(errors.Wrapf(errMetadataMismatch, "metadata mismatch, metadata:%#v, added:%#v, idx:%v", mdJSON, md, idx))
	}
}

func updateFirebaseEmail(authClient auth.Client, usr *record, idx uint64) {
	err := authClient.UpdateEmail(context.Background(), usr.ID, usr.Email)
	if errors.Is(err, auth.ErrConflict) { //nolint:revive // Nope.
		log.Error(errors.Errorf("duplicate email[firebase](belongs to another user): id:%v, email:%v", usr.ID, usr.Email))
	} else {
		log.Panic(errors.Wrapf(err, "can't update firebase email for userID:%v, email:%v, idx:%v", usr.ID, usr.Email, idx))
	}
	if true {
		return
	}
	firebaseUsr, err := fixture.GetUser(context.Background(), usr.ID)
	log.Panic(errors.Wrapf(err, "can't get user by id:%v from firebase, idx:%v", usr.ID, idx)) //nolint:revive // Intended.
	if firebaseUsr.Email != usr.Email {
		log.Panic(errors.Wrapf(errFirebaseEmailMismatch, "firebase emails mismatch, db:%v, firebase:%v, idx:%v", usr.Email, firebaseUsr.Email, idx))
	}
}

func getUsersToMerge(db *storage.DB, limit, offset uint64) []*record {
	params := []any{limit, offset}
	sql := `SELECT 
				coalesce(u.id,'') AS id,
				m.email,
				m.phone_number,
				coalesce(u.email,'') AS current_email
			FROM merge_firebase_phone_login_with_ice_email_login m
				LEFT JOIN users u
					ON m.phone_number = u.phone_number
			ORDER BY m.created_at ASC
			LIMIT $1 OFFSET $2`
	result, err := storage.Select[record](context.Background(), db, sql, params...)
	log.Panic(errors.Wrapf(err, "can't select records for limit:%v, offset:%v", limit, offset))

	return result
}
