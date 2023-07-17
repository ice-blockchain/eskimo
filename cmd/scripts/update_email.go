// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	_ "embed"

	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
)

const (
	applicationYamlEskimoKey = "users"
	applicationYamlAuthKey   = "auth/email-link"
)

var (
	//go:embed DDL.sql
	ddl string
)

type (
	record struct {
		PhoneNumber string `json:"phoneNumber" example:"+123456789"`
		Email       string `json:"email" example:"example@gmail.com"`
	}
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	usersProcessor := users.StartProcessor(ctx, cancel)
	authClient := auth.New(ctx, applicationYamlAuthKey)
	authEmailLinkClient := emaillink.NewClient(ctx, usersProcessor, authClient)
	db := storage.MustConnect(ctx, ddl, applicationYamlEskimoKey)
	defer db.Close()
	defer usersProcessor.Close()
	defer authEmailLinkClient.Close()

	records, err := getUsersForUpdate(ctx, db)
	log.Panic(err, "can't get users for update")
	if len(records) == 0 {
		log.Info("no records to update in the database")

		return
	}
	for _, value := range records {
		usr, err := getUserByPhoneNumber(ctx, db, value.PhoneNumber)
		if err != nil {
			log.Error(err, "can't get user by phone number: %v", value.PhoneNumber)

			continue
		}
		updUsr := users.User{
			PublicUserInformation: users.PublicUserInformation{
				ID: usr.ID,
			},
			PrivateUserInformation: users.PrivateUserInformation{
				SensitiveUserInformation: users.SensitiveUserInformation{
					Email: value.Email,
				},
			},
		}
		if err := usersProcessor.ModifyUser(ctx, &updUsr, nil); err != nil {
			log.Error(err, "can't modify eskimo user: %#v", updUsr)

			continue
		}
		md := users.JSON(map[string]any{
			auth.RegisteredWithProviderClaim: auth.ProviderFirebase,
			auth.FirebaseIDClaim:             usr.ID,
		})
		if _, err := authEmailLinkClient.UpdateMetadata(ctx, usr.ID, &md); err != nil {
			log.Error(err, "can't update metadata for userID:%v, phoneNumber:%v", usr.ID, value.PhoneNumber)

			continue
		}
		if err := authClient.UpdateEmail(ctx, usr.ID, value.Email); err != nil {
			log.Error(err, "can't update firebase email for userID:%v, email:%v", usr.ID, value.Email)
		}
	}
}

func getUserByPhoneNumber(ctx context.Context, db *storage.DB, phoneNumber string) (*users.User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result, err := storage.Get[users.User](ctx, db, `SELECT * FROM users WHERE phone_number = $1`, phoneNumber)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by phoneNumber %v", phoneNumber)
	}

	return result, nil
}

func getUsersForUpdate(ctx context.Context, db *storage.DB) (records []*record, err error) {
	sql := `SELECT 
				phone_number,
				email 
			FROM update_email_temporary`
	result, err := storage.Select[record](ctx, db, sql)
	if result == nil {
		return []*record{}, nil
	}

	return result, errors.Wrapf(err, "failed to select users for update")
}
