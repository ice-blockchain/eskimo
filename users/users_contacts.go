// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"strings"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
)

//nolint:funlen,gocritic,revive // It needs a better breakdown.
func (r *repository) findAgendaContactIDs(ctx context.Context, usr *User) ([]UserID, []UserID, []*Contact, error) {
	if usr.AgendaPhoneNumberHashes == nil || *usr.AgendaPhoneNumberHashes == "" {
		return nil, nil, nil, nil
	}
	before, err := r.getAgendaContacts(ctx, usr.ID)
	if err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return nil, nil, nil, errors.Wrapf(err, "can't get contacts for user id: %v", usr.ID)
	}
	sql := `SELECT id FROM users WHERE phone_number_hash = ANY($1)`
	contactIDs, err := storage.Select[UserID](ctx, r.db, sql, strings.Split(*usr.AgendaPhoneNumberHashes, ","))
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "can't get user ids by agenda hashes:%#v for userID:%v", *usr.AgendaPhoneNumberHashes, usr.ID)
	}
	if len(contactIDs) == 0 {
		return before, nil, nil, nil
	}
	var toUpsert, unique []UserID
	if before != nil {
		unique = contactDiff(before, contactIDs)
		toUpsert = append(toUpsert, before...)
	} else {
		for _, val := range contactIDs {
			unique = append(unique, *val)
		}
	}
	toUpsert = append(toUpsert, unique...)
	if len(unique) == 0 {
		return before, nil, nil, nil
	}
	contacts := make([]*Contact, 0, len(unique))
	for _, contactUserID := range unique {
		contacts = append(contacts, &Contact{
			UserID:        usr.ID,
			ContactUserID: contactUserID,
		})
	}

	return before, toUpsert, contacts, nil
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

func (r *repository) getAgendaContacts(ctx context.Context, userID UserID) ([]UserID, error) {
	type contacts struct {
		AgendaContactUserIDs []UserID `db:"agenda_contact_user_ids"`
	}
	sql := `SELECT COALESCE(agenda_contact_user_ids,'{}'::TEXT[]) as agenda_contact_user_ids FROM users WHERE id = $1`
	res, err := storage.Get[contacts](ctx, r.db, sql, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get contact user ids for userID:%v", userID)
	}

	return res.AgendaContactUserIDs, nil
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
