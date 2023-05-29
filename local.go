// SPDX-License-Identifier: ice License 1.0

package main

import (
	"flag"

	"github.com/google/uuid"

	"github.com/ice-blockchain/eskimo/users/fixture"
	"github.com/ice-blockchain/eskimo/users/seeding"
	authfixture "github.com/ice-blockchain/wintr/auth/fixture"
	"github.com/ice-blockchain/wintr/log"
)

//nolint:gochecknoglobals // Because those are flags
var (
	generateFirebaseAuth = flag.String("generateFirebaseAuth", "", "generate a new auth for a random user, with the specified role")
	generateIceAuth      = flag.String("generateIceAuth", "", "generate a new auth for a random user, with the specified role")
	startSeeding         = flag.Bool("startSeeding", false, "whether to start seeding a remote database or not")
)

func main() {
	flag.Parse()
	if generateFirebaseAuth != nil && *generateFirebaseAuth != "" {
		userID, accessToken := testingAuthorization(*generateFirebaseAuth)
		formatToken(userID, "", accessToken)

		return
	}
	if generateIceAuth != nil && *generateIceAuth != "" {
		userID := uuid.NewString()
		refreshToken, accessToken, err := authfixture.GenerateTokens(userID, *generateIceAuth)
		log.Panic(err) //nolint:revive // .
		formatToken(userID, refreshToken, accessToken)

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

func formatToken(userID, refresh, access string) {
	log.Info("UserID")
	log.Info("=================================================================================")
	log.Info(userID)
	log.Info("Refresh Token")
	log.Info("=================================================================================")
	log.Info(refresh)
	log.Info("Authorization Bearer Token")
	log.Info("=================================================================================")
	log.Info(access)
}
