// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"regexp"
	"testing"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	messagebrokerfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
	. "github.com/ice-blockchain/wintr/testing"
)

//nolint:paralleltest,funlen // We need a clean database.
func TestProcessor_Process_UserSnapshotSource_Success_IncrDecrUsersPerCountry(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	SETUP("we cleanup everything in the database", func() {
		mustDeleteEverything(ctx, t)
	})
	GIVEN("we have no users or progress in the database", func() {})
	var err error
	WHEN("processing a first created `UserSnapshot` message", func() {
		createdUser1 := (&UserSnapshot{User: new(User).completelyRandomizeForCreate(), Before: nil}).message(ctx, t)
		err = userSnapshotProcessor.Process(ctx, createdUser1)
	})
	THEN(func() {
		IT("is successfully processed", func() {
			require.NoError(t, err)
		})
		IT("incremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
		})
	})
	WHEN("processing a second created `UserSnapshot` message", func() {
		createdUser2 := (&UserSnapshot{User: new(User).completelyRandomizeForCreate(), Before: nil}).message(ctx, t)
		err = userSnapshotProcessor.Process(ctx, createdUser2)
	})
	THEN(func() {
		IT("is successfully processed", func() {
			require.NoError(t, err)
		})
		IT("incremented `users_per_country` by 1 for its country(default one)", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 2)
		})
	})
	WHEN("processing a updated `UserSnapshot` message where the user changed its country", func() {
		before := new(User).completelyRandomizeForCreate()
		after := *before
		after.Country = "RU"
		updatedUser := (&UserSnapshot{User: &after, Before: before}).message(ctx, t)
		err = userSnapshotProcessor.Process(ctx, updatedUser)
	})
	THEN(func() {
		IT("is successfully processed", func() {
			require.NoError(t, err)
		})
		IT("incremented `users_per_country` by 1 for `RU` and decremented by 1 for the default one", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
			assertUsersPerCountry(ctx, t, "RU", 1)
		})
	})
	WHEN("processing a updated `UserSnapshot` message where the country didn't change", func() {
		before := new(User).completelyRandomizeForCreate()
		after := *before
		after.FirstName = "somethingNew"
		updatedUser := (&UserSnapshot{User: &after, Before: before}).message(ctx, t)
		err = userSnapshotProcessor.Process(ctx, updatedUser)
	})
	THEN(func() {
		IT("is successfully processed", func() {
			require.NoError(t, err)
		})
		IT("didn't change anything", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 1)
			assertUsersPerCountry(ctx, t, "RU", 1)
		})
	})
	WHEN("processing a deleted `UserSnapshot` message", func() {
		deletedUser := (&UserSnapshot{User: nil, Before: new(User).completelyRandomizeForCreate()}).message(ctx, t)
		err = userSnapshotProcessor.Process(ctx, deletedUser)
	})
	THEN(func() {
		IT("is successfully processed", func() {
			require.NoError(t, err)
		})
		IT("decremented `users_per_country` by 1 for the default one and left the `RU` one unchanged", func() {
			assertUsersPerCountry(ctx, t, defaultClientIPCountry, 0)
			assertUsersPerCountry(ctx, t, "RU", 1)
		})
	})
}

func (s *UserSnapshot) message(ctx context.Context, tb testing.TB) *messagebroker.Message {
	tb.Helper()
	var key string
	if s.User == nil {
		key = s.Before.ID
	} else {
		key = s.ID
	}
	valueBytes, err := json.MarshalContext(ctx, s)
	require.NoError(tb, err, "failed to marshal %#v", s)

	return &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     key,
		Value:   valueBytes,
	}
}

func verifyUserSnapshotMessages(ctx context.Context, tb testing.TB, userSnapshots ...*UserSnapshot) {
	tb.Helper()
	require.NoError(tb, ctx.Err())

	messages := make([]messagebrokerfixture.RawMessage, 0, len(userSnapshots))
	for _, userSnapshot := range userSnapshots {
		valueBytes, err := json.MarshalContext(ctx, userSnapshot)
		require.NoError(tb, err)

		var id UserID
		if userSnapshot.User != nil {
			id = userSnapshot.User.ID
		} else {
			id = userSnapshot.Before.ID
		}

		messages = append(messages, messagebrokerfixture.RawMessage{
			Key:   id,
			Value: regexp.QuoteMeta(string(valueBytes)),
			Topic: cfg.MessageBroker.Topics[0].Name, // | users-table.
		})
	}

	assert.NoError(tb, mbConnector.VerifyMessages(ctx, messages...))
}

const (
	createdRegex = `^{((?!,"before":).)*}$`
	updatedRegex = `^{.+,"before":{.+}$`
	deletedRegex = `^{"before":{.+}$`
	anyRegex     = "^.*$"
)

func verifyNoUserSnapshotMessages(ctx context.Context, tb testing.TB, tpe UserSnapshotMessageType, userIDs ...string) {
	tb.Helper()
	assert.NoError(tb, verifyNoUserSnapshotMessagesWithError(ctx, tb, tpe, userIDs...))
}

func verifyNoUserSnapshotMessagesWithError(ctx context.Context, tb testing.TB, tpe UserSnapshotMessageType, userIDs ...string) error {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	messages := tpe.messagesForVerification(userIDs...)
	windowedCtx, cancelWindowed := context.WithTimeout(ctx, 2*stdlibtime.Second)
	defer cancelWindowed()

	return errors.Wrapf(mbConnector.VerifyNoMessages(windowedCtx, messages...), "no messages for %v %v detected", tpe, userIDs)
}

func verifySomeUserSnapshotMessages(ctx context.Context, tb testing.TB, tpe UserSnapshotMessageType, userIDs ...string) {
	tb.Helper()
	assert.NoError(tb, verifySomeUserSnapshotMessagesWithError(ctx, tb, tpe, userIDs...))
}

func verifySomeUserSnapshotMessagesWithError(ctx context.Context, tb testing.TB, tpe UserSnapshotMessageType, userIDs ...string) error {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	messages := tpe.messagesForVerification(userIDs...)
	windowedCtx, cancelWindowed := context.WithTimeout(ctx, 2*stdlibtime.Second)
	defer cancelWindowed()

	return errors.Wrapf(mbConnector.VerifyMessages(windowedCtx, messages...), "no messages for %v %v detected", tpe, userIDs)
}

func (tpe UserSnapshotMessageType) messagesForVerification(userIDs ...UserID) []messagebrokerfixture.RawMessage {
	messages := make([]messagebrokerfixture.RawMessage, 0, len(userIDs))
	for _, userID := range userIDs {
		var valueRegex string
		switch tpe {
		case ANY:
			valueRegex = anyRegex
		case CREATE:
			valueRegex = createdRegex
		case UPDATE:
			valueRegex = updatedRegex
		case DELETE:
			valueRegex = deletedRegex
		}
		messages = append(messages, messagebrokerfixture.RawMessage{
			Key:   userID,
			Value: valueRegex,
			Topic: cfg.MessageBroker.Topics[0].Name, // | users-table.
		})
	}

	return messages
}

func verifyAnyOfUserSnapshotMessages(ctx context.Context, tb testing.TB, funcs ...verifyMessages) {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	for i := range funcs {
		if funcs[i]() == nil {
			break
		}
	}
}

const (
	ANY UserSnapshotMessageType = iota
	CREATE
	UPDATE
	DELETE
)

type (
	UserSnapshotMessageType byte
	verifyMessages          func() error
)
