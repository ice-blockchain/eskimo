// SPDX-License-Identifier: ice License 1.0

package main

import (
	"flag"

	"github.com/ice-blockchain/eskimo/users/fixture"
	"github.com/ice-blockchain/eskimo/users/seeding"
	authfixture "github.com/ice-blockchain/wintr/auth/fixture"
	"github.com/ice-blockchain/wintr/log"
)

//nolint:gochecknoglobals // Because those are flags
var (
	generateAuth = flag.String("generateAuth", "", "generate a new auth for a random user, with the specified role")
	startSeeding = flag.Bool("startSeeding", false, "whether to start seeding a remote database or not")
)

func main() {
	flag.Parse()
	if generateAuth != nil && *generateAuth != "" {
		userID, token := testingAuthorization(*generateAuth)
		log.Info("UserID")
		log.Info("=================================================================================")
		log.Info(userID)
		log.Info("Authorization Bearer Token")
		log.Info("=================================================================================")
		log.Info(token)

		return
	}
	if *startSeeding {
		seeding.StartSeeding()

		return
	}

	fixture.StartLocalTestEnvironment()
}

func testingAuthorization(role string) (userID, token string) {
	return authfixture.CreateUser(role)
}
