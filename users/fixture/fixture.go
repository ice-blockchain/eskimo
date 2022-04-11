package fixture

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	messagebrokerfixture "github.com/ICE-Blockchain/wintr/connectors/message_broker/fixture"
	"github.com/ICE-Blockchain/wintr/connectors/storage"
	storagefixture "github.com/ICE-Blockchain/wintr/connectors/storage/fixture"
	"github.com/ICE-Blockchain/wintr/log"
	"github.com/framey-io/go-tarantool"
)

func TestSetup() func() {
	cleanUpStorage, cleanUpMessageBroker := setupDBAndMessageBroker()

	return func() {
		dbError, mbError := cleanUp(cleanUpStorage, cleanUpMessageBroker)
		if dbError != nil || mbError != nil {
			err := errors.New("users fixture cleanup failed")
			log.Panic(err, "dbError", dbError, "mbError", mbError)
		}
	}
}

func setupDBAndMessageBroker() (func(), func()) {
	wg := new(sync.WaitGroup)
	var cleanUpStorage func()
	var cleanUpMessageBroker func()
	wg.Add(1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		cleanUpStorage = storagefixture.TestSetup("users")
	}()
	go func() {
		defer wg.Done()
		cleanUpMessageBroker = messagebrokerfixture.TestSetup("users")
	}()
	wg.Wait()

	return cleanUpStorage, cleanUpMessageBroker
}

func cleanUp(cleanUpStorage, cleanUpMessageBroker func()) (error, error) {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Add(1)
	var dbError error
	var mbError error
	go func() {
		defer wg.Done()
		if err := recover(); err != nil {
			dbError = err.(error)
		}
		cleanUpStorage()
	}()
	go func() {
		defer wg.Done()
		if err := recover(); err != nil {
			mbError = err.(error)
		}
		cleanUpMessageBroker()
	}()
	wg.Wait()

	return dbError, mbError
}

func MustInsertUserWithTime(t *testing.T, db tarantool.Connector, user *User) {
	t.Helper()
	user.ID = fmt.Sprintf("%v%v", user.ID, uuid.New().String())

	sql := `INSERT INTO users (id, email, fullName, phoneNumber, username, referredBy, profilePictureURL, createdAt)
				VALUES(:id, :email, :fullName, :phoneNumber, :username, :referredBy, :profilePictureURL, :createdAt)`

	params := map[string]interface{}{
		"id":                user.ID,
		"email":             user.Email,
		"fullName":          user.FullName,
		"phoneNumber":       user.PhoneNumber,
		"username":          user.Username,
		"referredBy":        user.ReferredBy,
		"profilePictureURL": user.ProfilePictureURL,
		"createdAt":         user.CreatedAt.UnixNano(),
	}

	require.NoError(t, storage.CheckSQLDMLErr(db.PrepareExecute(sql, params)))
}
