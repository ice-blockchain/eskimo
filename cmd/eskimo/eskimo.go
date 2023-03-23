// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/cmd/eskimo/api"
	"github.com/ice-blockchain/eskimo/users"
	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/server"
)

// @title						User Accounts, User Devices, User Statistics API
// @version					latest
// @description				API that handles everything related to read only operations for user's account, user's devices and statistics about accounts and devices.
// @query.collection.format	multi
// @schemes					https
// @contact.name				ice.io
// @contact.url				https://ice.io
// @BasePath					/v1r
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	api.SwaggerInfo.Host = cfg.Host
	api.SwaggerInfo.Version = cfg.Version
	server.New(new(service), applicationYamlKey, swaggerRoot).ListenAndServe(ctx, cancel)
}

func (s *service) RegisterRoutes(router *server.Router) {
	s.setupUserRoutes(router)
	s.setupUserReferralRoutes(router)
	s.setupUserStatisticsRoutes(router)
}

func (s *service) Init(ctx context.Context, cancel context.CancelFunc) {
	s.usersRepository = users.New(ctx, cancel)
}

func (s *service) Close(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "could not close repository because context ended")
	}

	return errors.Wrap(s.usersRepository.Close(), "could not close repository")
}

func (s *service) CheckHealth(ctx context.Context) error {
	log.Debug("checking health...", "package", "users")
	_, err := s.usersRepository.GetTopCountries(ctx, "", 1, 0)

	return errors.Wrapf(err, "get top countries failed")
}
