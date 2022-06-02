// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/cmd/eskimo-hut/api"
	"github.com/ice-blockchain/eskimo/users"
	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/server"
)

//nolint:godot,lll // Because those are comments parsed by swagger
// @title                    User Accounts, User Devices, User Statistics API
// @version                  latest
// @description              API that handles everything related to write only operations for user's account, user's devices and statistics about those accounts and devices.
// @query.collection.format  multi
// @schemes                  https
// @contact.name             ice.io
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
	s.setupUserValidationRoutes(engine)
	s.setupDevicesRoutes(engine)
}

func (s *service) Init(ctx context.Context, cancel context.CancelFunc) {
	s.usersProcessor = users.StartProcessor(ctx, cancel)
}

func (s *service) Close(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "could not close usersProcessor because context ended")
	}

	return errors.Wrap(s.usersProcessor.Close(), "could not close usersProcessor")
}

func (s *service) CheckHealth(ctx context.Context, r *server.RequestCheckHealth) server.Response {
	log.Debug("checking health...", "package", "users")

	if err := s.usersProcessor.CheckHealth(ctx); err != nil {
		return server.Unexpected(err)
	}

	return server.OK(r)
}
