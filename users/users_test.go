// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"testing"

	"github.com/ice-blockchain/eskimo/users/fixture"
	messagebrokerfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
	storagefixture "github.com/ice-blockchain/wintr/connectors/storage/fixture"
)

//nolint:gochecknoglobals // Because those are global, set only once for the whole package test runtime and execution.
var (
	dbConnector storagefixture.TestConnector
	mbConnector messagebrokerfixture.TestConnector
)

func TestMain(m *testing.M) {
	fixture.RunTests(m, &dbConnector, &mbConnector)
}
