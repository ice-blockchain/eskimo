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
	order          = usersfixture.TestConnectorsOrder + 1
	testDeadline   = 30 * time.Second
	testMagicToken = "WyIweGFhNTBiZTcwNzI5Y2E3MDViYTdjOGQwMDE4NWM2ZjJkYTQ3OWQwZm" +
		"NkZTUzMTFjYTRjZTViMWJhNzE1YzhhNzIxYzVmMTk0ODQzNGY5NmZmNTc3ZDdiMmI2YWQ4MmQ" +
		"zZGQ1YTI0NTdmZTY5OThiMTM3ZWQ5YmMwOGQzNmU1NDljMWIiLCJ7XCJpYXRcIjoxNTg2NzY0" +
		"MjcwLFwiZXh0XCI6MTExNzM1Mjg1MDAsXCJpc3NcIjpcImRpZDpldGhyOjB4NEI3M0M1ODM3M" +
		"EFFZmNFZjg2QTYwMjFhZkNEZTU2NzM1MTEzNzZCMlwiLFwic3ViXCI6XCJOanJBNTNTY1E4SV" +
		"Y4ME5Kbng0dDNTaGk5LWtGZkY1cWF2RDJWcjBkMWRjPVwiLFwiYXVkXCI6XCJkaWQ6bWFnaWM" +
		"6NzMxODQ4Y2MtMDg0ZS00MWZmLWJiZGYtN2YxMDM4MTdlYTZiXCIsXCJuYmZcIjoxNTg2NzY0" +
		"MjcwLFwidGlkXCI6XCJlYmNjODgwYS1mZmM5LTQzNzUtODRhZS0xNTRjY2Q1Yzc0NmRcIixcI" +
		"mFkZFwiOlwiMHg4NGQ2ODM5MjY4YTFhZjkxMTFmZGVjY2QzOTZmMzAzODA1ZGNhMmJjMDM0NT" +
		"BiN2ViMTE2ZTJmNWZjOGM1YTcyMmQxZmI5YWYyMzNhYTczYzVjMTcwODM5Y2U1YWQ4MTQxYjl" +
		"iNDY0MzM4MDk4MmRhNGJmYmIwYjExMjg0OTg4ZjFiXCJ9Il0="
	testMagicToken2ndUser = "WyIweDAwNzAxZDc3MjRjNjMzY2M1Mzk3MTBlYzIxNzMwM2Y3ZDFhZ" +
		"jEyYzJhNWQwODZjZmNmMzAwNzg0MTU1MWI0NTY1OWU2ZTVkZDM3ODk5ODJiNGEwYmY1OTJlNW" +
		"UzYjc0MDM3M2IyMDQ2Y2U4ZGEzYjJhNWNlNjcyYmJiYTU3NjIwMWIiLCJ7XCJpYXRcIjoxNjU" +
		"4Mzk3OTMwLFwiZXh0XCI6MTk3MzkzMDczMCxcImlzc1wiOlwiZGlkOmV0aHI6MHhEZWIyQTIw" +
		"MzYzRTkwNjNhZDUyMUIzMTU2MzA0YjlFMTI4MzQ2NDRCXCIsXCJzdWJcIjpcImhUVmNBSVpqb" +
		"3NyX2NSTFp6TWhnSHZtQ1FCbkdxTmxYQjh1NHh2VUU2dE09XCIsXCJhdWRcIjpcIlN1dnVJRE" +
		"ZEM0hUdVkwbjh1bXREWHY3Tk8zcHFmNExSODJXaEFmdlJyNnc9XCIsXCJuYmZcIjoxNjU4Mzk" +
		"3OTMwLFwidGlkXCI6XCI0OGM0MThmMy0wMDgzLTQ1YzMtYTVhZi05ZTNmM2IxM2M1ZTFcIixc" +
		"ImFkZFwiOlwiMHg1NWJjNzU2ODNlZTA5ZWQzODRmMTc4NTY3MjU5YzVkNjZjYTg2MjdjMzUzN" +
		"WYyYTFlODM5NjUxNTI0YjNlZDdiNWRhZTg1NGRmNWQyYWYwYzRjOTA3Y2UyMTIyMWIzYTU5ND" +
		"c0MGY1OWExNzI2NjFjYTY3ZjYwOWZkMzhjOThmYzFjXCJ9Il0="
	timeRegex = "[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[1-2][0-9]|3[0-1])T(2[0-3]|[01][0-9]):[0-5][0-9]:([0-9]+)[.]([0-9]+)Z"
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
