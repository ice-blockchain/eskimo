// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	_ "embed"
	"testing"

	"github.com/ice-blockchain/eskimo/cmd/fixture"
	usersfixture "github.com/ice-blockchain/eskimo/users/fixture"
	connectorsfixture "github.com/ice-blockchain/wintr/connectors/fixture"
	serverfixture "github.com/ice-blockchain/wintr/server/fixture"
)

var (
	//nolint:gochecknoglobals // Because those are global, set only once for the whole package test runtime and execution.
	bridge *fixture.TestConnectorsBridge
	//go:embed .testdata/expected_swagger.json
	expectedSwaggerJSON string
)

func TestMain(m *testing.M) {
	const order = usersfixture.TestConnectorsOrder + 1
	read := serverfixture.NewTestConnector("cmd/eskimo", "eskimo", swaggerRoot, expectedSwaggerJSON, order+1, usersfixture.RContainerMounts()...)
	write := serverfixture.NewTestConnector("cmd/eskimo-hut", "eskimo", "", "", order, usersfixture.WContainerMounts()...)
	bridge = fixture.NewBridge(read, write)

	connectorsfixture.
		NewTestRunner(applicationYamlKey, nil, append(usersfixture.WTestConnectors(), write, read)...).
		RunTests(m)
}

func TestSwagger(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	bridge.R.TestSwagger(ctx, t)
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	bridge.W.TestHealthCheck(ctx, t)
	bridge.R.TestHealthCheck(ctx, t)
}
