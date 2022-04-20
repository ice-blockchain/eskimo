// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/framey-io/go-tarantool"
	"github.com/hashicorp/go-multierror"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	appCfg "github.com/ICE-Blockchain/wintr/config"
	messagebroker "github.com/ICE-Blockchain/wintr/connectors/message_broker"
	"github.com/ICE-Blockchain/wintr/connectors/storage"
	"github.com/ICE-Blockchain/wintr/log"
)

func New(ctx context.Context, cancel context.CancelFunc) Repository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)

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

//nolint:funlen // A lot of SQL
func (u *users) AddUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "add user failed because context failed")
	}
	user.created()

	sql := `INSERT INTO users (ID, HASH_CODE, EMAIL, FULL_NAME, PHONE_NUMBER,
			USERNAME, REFERRED_BY, PROFILE_PICTURE, COUNTRY, CREATED_AT, UPDATED_AT)
			VALUES(:id, :hashCode, :email, :fullName, :phoneNumber, 
			:username, :referredBy, :profilePictureURL, :country, :createdAt, :updatedAt)`

	var refer UserID
	if user.ReferredBy != "" {
		refer = user.ReferredBy
	} else {
		newUser, err := u.GetUser(ctx, "")
		if err != nil {
			return errors.Wrapf(err, "failed to get random user")
		}
		refer = newUser.ID
	}

	params := map[string]interface{}{
		"id":                user.ID,
		"hashCode":          u.hash(user.ID),
		"email":             user.Email,
		"fullName":          user.FullName,
		"phoneNumber":       user.PhoneNumber,
		"username":          user.Username,
		"referredBy":        refer,
		"profilePictureURL": user.ProfilePicture.Filename,
		"country":           user.Country,
		"createdAt":         user.CreatedAt.UnixNano(),
		"updatedAt":         user.UpdatedAt.UnixNano(),
	}

	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to add user %#v", user)
	}

	u.sendUsersMessage(ctx, user)

	return nil
}

func (u *users) hash(data string) uint64 {
	return xxh3.HashStringSeed(data, uint64(time.Now().UTC().UnixNano()))
}

func (u *User) created() *User {
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt

	return u
}

func (u *users) GetUser(ctx context.Context, id UserID) (*User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result := new(user)

	var key interface{}

	if id == "" {
		key = []interface{}{}
	} else {
		key = []interface{}{id}
	}

	if err := u.db.GetTyped("USERS", "pk_unnamed_USERS_1", key, result); err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	if result.ID == "" {
		return nil, errors.Wrapf(ErrNotFound, "no user found with id %v", id)
	}

	return result.toUser(), nil
}

func (u *user) toUser() *User {
	profilePictureURL := fmt.Sprintf("%s/%s/%s", cfg.Storage.URLDownload,
		cfg.Storage.ProfilePath,
		u.ProfilePicture)

	return &User{
		ID:                u.ID,
		HashCode:          u.HashCode,
		ReferredBy:        u.ReferredBy,
		Username:          u.Username,
		Email:             u.Email,
		FullName:          u.FullName,
		PhoneNumber:       u.PhoneNumber,
		ProfilePictureURL: profilePictureURL,
		Country:           u.Country,
		CreatedAt:         time.Unix(0, int64(u.CreatedAt)).UTC(),
		UpdatedAt:         time.Unix(0, int64(u.UpdatedAt)).UTC(),
		DeletedAt:         nil,
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
	params := map[string]interface{}{"id": userID}
	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to remove user with id %v", userID)
	}
	u.sendUsersMessage(ctx, gUser.deleted())

	return nil
}

func (u *User) deleted() *User {
	now := time.Now().UTC()
	u.DeletedAt = &now

	return u
}

func (u *users) ModifyUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	user.updated()

	gUser, err := u.GetUser(ctx, user.ID)
	if err != nil {
		return errors.Wrapf(ErrNotFound, "no user found with id %v", user.ID)
	}

	if user.ProfilePicture.Filename != "" && user.ProfilePicture.Size != 0 {
		user.ProfilePicture.Filename = fmt.Sprintf("%v", gUser.HashCode)
		err = u.UploadProfilePicture(ctx, &user.ProfilePicture)
		if err != nil {
			return errors.Wrapf(err, "failed to upload user image, id %v", user.ID)
		}
	}

	sql, params := user.GenSQLUpdate()
	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to update user with id %v", user.ID)
	}

	u.sendUsersMessage(ctx, user)

	return nil
}

//nolint:funlen // Fill DB cause a lot of fields
func (u *User) GenSQLUpdate() (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	sql = "UPDATE USERS set "

	params["id"] = u.ID
	params["updatedAt"] = u.UpdatedAt.UnixNano()
	sql += "UPDATED_AT = :updatedAt"

	if u.Email != "" {
		params["email"] = u.Email
		sql += ", EMAIL = :email"
	}

	if u.FullName != "" {
		params["fullName"] = u.FullName
		sql += ", FULL_NAME = :fullName"
	}

	if u.PhoneNumber != "" {
		params["phoneNumber"] = u.PhoneNumber
		sql += ", PHONE_NUMBER = :phoneNumber"
	}

	if u.Username != "" {
		params["username"] = u.Username
		sql += ", USERNAME = :username"
	}

	if u.ProfilePicture.Filename != "" {
		params["profilePictureURL"] = u.ProfilePicture.Filename
		sql += ", PROFILE_PICTURE = :profilePictureURL"
	}

	if u.Country != "" {
		params["country"] = u.Country
		sql += ", COUNTRY = :country"
	}

	sql += " WHERE ID = :id"

	return sql, params
}

func (u *User) updated() *User {
	now := time.Now().UTC()
	u.UpdatedAt = now

	return u
}

func (u *users) UploadProfilePicture(_ context.Context, data *multipart.FileHeader) error {
	file, err := data.Open()
	defer file.Close()
	if err != nil {
		return errors.Wrap(err, "error opening file")
	}
	fileData, err := io.ReadAll(file)
	if err != nil {
		return errors.Wrap(err, "error reading file")
	}
	url := fmt.Sprintf("%s/%s/%s/%s", cfg.Storage.URLUpload,
		cfg.Storage.ZoneName,
		cfg.Storage.ProfilePath,
		data.Filename)
	_, err = req.
		SetRetryCount(int(cfg.Storage.RetryCount)).
		SetRetryCondition(func(resp *req.Response, err error) bool {
			return (err != nil) || (resp.StatusCode == http.StatusTooManyRequests)
		}).
		SetHeader("AccessKey", cfg.Storage.AccessKey).
		SetHeader("Content-Type", data.Header.Get("Content-Type")).
		SetBodyBytes(fileData).Put(url)
	if err != nil {
		return errors.Wrap(err, "error uploading file")
	}

	return nil
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
