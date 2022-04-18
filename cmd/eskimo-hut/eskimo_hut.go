// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"github.com/ICE-Blockchain/eskimo/cmd/eskimo/api"
	"github.com/ICE-Blockchain/eskimo/users"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	appCfg "github.com/ICE-Blockchain/wintr/config"
	"github.com/ICE-Blockchain/wintr/log"
	"github.com/ICE-Blockchain/wintr/server"
)

//nolint:godot // Because those are comments parsed by swagger
// @title                    User Account API
// @version                  latest
// @description              API that handles everything related to write only operations for user's account.
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
	s.setupUserValidationRoutes(engine)
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
