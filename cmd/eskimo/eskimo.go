// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/cmd/eskimo/api"
	"github.com/ice-blockchain/eskimo/users"
	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/server"
)

//nolint:godot // Because those are comments parsed by swagger
// @title                    User Account API
// @version                  latest
// @description              API that handles everything related to read only operations for user's account.
// @query.collection.format  multi
// @schemes                  https
// @contact.name             ICE
// @contact.url              https://ice.io
// @BasePath                 /v1
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	api.SwaggerInfo.Host = cfg.Host
	api.SwaggerInfo.Version = cfg.Version
	srv := server.New(new(service), applicationYamlKey, "/users")
	srv.ListenAndServe(ctx, cancel)
}

func (s *service) RegisterRoutes(engine *gin.Engine) {
	s.setupUserRoutes(engine)
	s.setupUserReferralRoutes(engine)
	s.setupUserStatisticsRoutes(engine)
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

func (s *service) CheckHealth(_ context.Context, r *server.RequestCheckHealth) server.Response {
	log.Debug("checking health...", "package", "users")
	//nolint:nolintlint,godox // TODO to be implemented. // Not yet

	return server.OK(r)
}
