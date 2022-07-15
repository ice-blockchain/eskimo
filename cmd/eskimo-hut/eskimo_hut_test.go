// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	_ "embed"
	"testing"
	"time"

	usersfixture "github.com/ice-blockchain/eskimo/users/fixture"
	connectorsfixture "github.com/ice-blockchain/wintr/connectors/fixture"
	serverfixture "github.com/ice-blockchain/wintr/server/fixture"
)

const (
	order        = usersfixture.TestConnectorsOrder + 1
	testDeadline = 30 * time.Second
)

var (
	//nolint:gochecknoglobals // Because those are global, set only once for the whole package test runtime and execution.
	serverConnector serverfixture.TestConnector
	//go:embed .testdata/expected_swagger.json
	expectedSwaggerJSON string
)

func TestMain(m *testing.M) {
	serverConnector = serverfixture.NewTestConnector(applicationYamlKey, swaggerRoot, expectedSwaggerJSON, order, main, usersfixture.WContainerMounts()...)
	testConnectors := usersfixture.WTestConnectors()
	testConnectors = append(testConnectors, serverConnector)

	connectorsfixture.
		NewTestRunner(applicationYamlKey, nil, testConnectors...).
		RunTests(m)
}

func TestSwagger(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	serverConnector.TestSwagger(ctx, t)
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	serverConnector.TestHealthCheck(ctx, t)
}
