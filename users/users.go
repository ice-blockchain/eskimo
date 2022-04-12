// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"
	"fmt"
	appCfg "github.com/ICE-Blockchain/wintr/config"
	messagebroker "github.com/ICE-Blockchain/wintr/connectors/message_broker"
	"github.com/ICE-Blockchain/wintr/connectors/storage"
	"github.com/ICE-Blockchain/wintr/log"
	"github.com/framey-io/go-tarantool"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"time"
)

func New(ctx context.Context, cancel context.CancelFunc) Repository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	db := storage.MustConnect(ctx, cancel, DDL, applicationYamlKey)

	mb := messagebroker.MustConnect(ctx, applicationYamlKey)

	return &repository{
		close:          closeAll(db, mb),
		UserRepository: &users{mb: mb, db: db},
	}
}

func closeAll(db tarantool.Connector, mb messagebroker.Client) func() error {
	return func() error {
		err1 := errors.Wrap(db.Close(), "closing db connection failed")
		err2 := errors.Wrap(mb.Close(), "closing message broker connection failed")
		if err1 != nil && err2 != nil {
			return multierror.Append(err1, err2)
		}
		var err error
		if err1 != nil {
			err = err1
		}
		if err2 != nil {
			err = err2
		}

		return errors.Wrapf(err, "failed to close all resources")
	}
}

func (r *repository) Close() error {
	log.Info("closing users repository...")

	return errors.Wrap(r.close(), "closing users repository failed")
}

func (u *users) AddUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "add user failed because context failed")
	}
	user.created()

	sql := `INSERT INTO users (ID, EMAIL, FULL_NAME, PHONE_NUMBER, USERNAME, REFERREDBY, PROFILE_PICTURE, CREATED_AT)
				VALUES(:id, :email, :fullName, :phoneNumber, :username, :referredBy, :profilePictureURL, :createdAt)`

	var refer interface{}
	if user.ReferredBy != "" {
		refer = user.ReferredBy
	}

	params := map[string]interface{}{
		"id":                user.ID,
		"email":             user.Email,
		"fullName":          user.FullName,
		"phoneNumber":       user.PhoneNumber,
		"username":          user.Username,
		"referredBy":        refer,
		"profilePictureURL": user.ProfilePictureURL,
		"createdAt":         user.CreatedAt.UnixNano(),
	}

	query, err := u.db.PrepareExecute(sql, params)

	if err := storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to add user %#v", user)
	}

	u.sendUsersMessage(ctx, user)

	return nil
}

func (u *User) created() *User {
	u.ID = fmt.Sprintf("%v%v", u.ID, uuid.New().String())
	u.CreatedAt = time.Now().UTC()

	return u
}

func (u *users) GetUser(ctx context.Context, id UserID) (*User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result := new(user)
	if err := u.db.GetTyped("USERS", nil, []interface{}{id}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	if result.ID == "" {
		return nil, errors.Wrapf(ErrNotFound, "no user found with id %v", id)
	}

	return result.toUser(), nil
}

func (u *user) toUser() *User {
	return &User{
		ID:                string(u.ID),
		ReferredBy:        string(u.ReferredBy),
		Username:          u.Username,
		Email:             u.Email,
		FullName:          u.FullName,
		PhoneNumber:       u.PhoneNumber,
		ProfilePictureURL: u.ProfilePicture,
		CreatedAt:         time.Unix(0, int64(u.CreatedAt)).UTC(),
		UpdatedAt:         time.Unix(0, int64(u.UpdatedAt)).UTC(),
		DeletedAt:         time.Unix(0, int64(u.DeletedAt)).UTC(),
	}
}

func (u *users) RemoveUser(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "remove user failed because context failed")
	}

	gUser, err := u.GetUser(ctx, userID)

	if err != nil {
		return err
	}

	sql := `DELETE FROM users WHERE id = :id`

	params := map[string]interface{}{
		"id": userID,
	}

	if err = storage.CheckSQLDMLErr(u.db.PrepareExecute(sql, params)); err != nil {
		return errors.Wrapf(err, "failed to remove user with id %v", userID)
	}

	u.sendUsersMessage(ctx, gUser.deleted())

	return nil
}

func (u *User) deleted() *User {
	now := time.Now().UTC()
	u.DeletedAt = now

	return u
}

func (u *users) ModifyUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	user.updated()

	sql := `UPDATE users SET EMAIL = :email, FULL_NAME = :fullName, PHONE_NUMBER = :phoneNumber, USERNAME = :username, PROFILE_PICTURE = :profilePictureURL, UPDATED_AT = :updatedAt WHERE ID = :id`

	params := map[string]interface{}{
		"id":                user.ID,
		"email":             user.Email,
		"fullName":          user.FullName,
		"phoneNumber":       user.PhoneNumber,
		"username":          user.Username,
		"profilePictureURL": user.ProfilePictureURL,
		"updatedAt":         user.UpdatedAt.UnixNano(),
	}

	if err := storage.CheckSQLDMLErr(u.db.PrepareExecute(sql, params)); err != nil {
		return errors.Wrapf(err, "failed to update user with id %v", user.ID)
	}

	u.sendUsersMessage(ctx, user)

	return nil
}

func (u *User) updated() *User {
	now := time.Now().UTC()
	u.UpdatedAt = now

	return u
}

func (u *users) sendUsersMessage(ctx context.Context, user *User) {
	valueBytes, err := json.Marshal(user)

	if err != nil {
		log.Error(errors.Wrapf(err, "failed to marshal user %v", user))

		return
	}

	//nolint:govet // Because we don`t need to cancel it cuz its a fire and forget action.
	pCtx, _ := context.WithTimeout(context.Background(), messageBrokerProduceRecordDeadline)

	var responder chan<- error
	if ctx.Value(messageBrokerProduceMessageResponseChanKey{}) != nil {
		responder = ctx.Value(messageBrokerProduceMessageResponseChanKey{}).(chan error)
	}

	u.mb.SendMessage(pCtx, &messagebroker.Message{
		Key:     user.ID,
		Value:   valueBytes,
		Headers: map[string]string{"producer": "eskimo"},
		Topic:   cfg.MessageBroker.Topics[0].Name,
	}, responder)
}
