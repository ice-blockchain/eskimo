// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
)

//nolint:funlen // It needs a better breakdown.
func (r *repository) upsertContacts(ctx context.Context, usr *User) error {
	before, err := r.getContacts(ctx, usr.ID)
	if err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "can't get contacts for user id: %v", usr.ID)
	}
	sql := `SELECT id FROM users WHERE phone_number_hash = ANY($1)`
	contactIDs, err := storage.Select[UserID](ctx, r.db, sql, strings.Split(usr.AgendaPhoneNumberHashes, ","))
	if err != nil {
		return errors.Wrapf(err, "can't get user ids by agenda hashes:%#v for userID:%v", usr.AgendaPhoneNumberHashes, usr.ID)
	}
	if len(contactIDs) == 0 {
		return nil
	}
	var toUpsert, unique []UserID
	if before != nil {
		ids := strings.Split(before.ContactUserIDs, ",")
		unique = contactDiff(ids, contactIDs)
		toUpsert = append(toUpsert, ids...)
	} else {
		for _, val := range contactIDs {
			unique = append(unique, *val)
		}
	}
	toUpsert = append(toUpsert, unique...)
	if len(unique) == 0 {
		return nil
	}
	sql = `INSERT INTO contacts(user_id, contact_user_ids) VALUES ($1, $2)
				ON CONFLICT(user_id)
				DO UPDATE
					SET contact_user_ids = EXCLUDED.contact_user_ids
				WHERE contacts.contact_user_ids != EXCLUDED.contact_user_ids`
	if _, iErr := storage.Exec(ctx, r.db, sql, usr.ID, strings.Join(toUpsert, ",")); iErr != nil {
		return errors.Wrapf(iErr, "can't insert/update contact user ids:%#v for userID:%v", toUpsert, usr.ID)
	}
	contacts := make([]*Contact, 0, len(unique))
	for _, contactUserID := range unique {
		contacts = append(contacts, &Contact{
			UserID:        usr.ID,
			ContactUserID: contactUserID,
		})
	}
	if sErr := runConcurrently(ctx, r.sendContactMessage, contacts); sErr != nil {
		var rErr error
		if before != nil {
			sql = `UPDATE contacts SET contact_user_ids = $1 WHERE user_id = $2 AND contact_user_ids = $3`
			_, rErr = storage.Exec(ctx, r.db, sql, strings.Join(toUpsert, ","), usr.ID, before.ContactUserIDs)
		} else {
			sql = `DELETE FROM contacts WHERE user_id = $1`
			_, rErr = storage.Exec(ctx, r.db, sql, usr.ID)
		}

		return errors.Wrapf(multierror.Append(rErr, sErr).ErrorOrNil(), "can't send contacts message for userID:%v", usr.ID)
	}

	return nil
}

func contactDiff(fromTable []UserID, fromRequest []*UserID) []UserID {
	var unique []UserID
	for _, valRequest := range fromRequest {
		found := false
		for _, valTable := range fromTable {
			if valTable == *valRequest {
				found = true

				break
			}
		}
		if !found {
			unique = append(unique, *valRequest)
		}
	}

	return unique
}

func (r *repository) getContacts(ctx context.Context, userID UserID) (*contacts, error) {
	sql := `SELECT * FROM contacts WHERE user_id = $1`
	res, err := storage.Get[contacts](ctx, r.db, sql, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get contact user ids for userID:%v", userID)
	}

	return res, nil
}

func (r *repository) sendContactMessage(ctx context.Context, contact *Contact) error {
	valueBytes, err := json.MarshalContext(ctx, contact)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal %#v", contact)
	}
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     contact.UserID,
		Topic:   r.cfg.MessageBroker.Topics[4].Name,
		Value:   valueBytes,
	}

	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send contacts message to broker")
}

func runConcurrently[ARG any](ctx context.Context, run func(context.Context, ARG) error, args []ARG) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	if len(args) == 0 {
		return nil
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(args))
	errChan := make(chan error, len(args))
	for i := range args {
		go func(ix int) {
			defer wg.Done()
			errChan <- errors.Wrapf(run(ctx, args[ix]), "failed to run:%#v", args[ix])
		}(i)
	}
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(args))
	for err := range errChan {
		errs = append(errs, err)
	}

	return errors.Wrap(multierror.Append(nil, errs...).ErrorOrNil(), "at least one execution failed")
}
