package emaillink

import (
	"context"
	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/pkg/errors"
)

func New(ctx context.Context, _ context.CancelFunc) Repository {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &repository{
		cfg:      &cfg,
		shutdown: db.Close,
		db:       db,
	}
}
func StartProcessor(ctx context.Context, _ context.CancelFunc) Processor {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &processor{&repository{
		cfg:      &cfg,
		shutdown: db.Close,
		db:       db,
	},
	}
}
func (r *repository) Close() error {
	return errors.Wrap(r.shutdown(), "closing auth/emaillink repository failed")
}
