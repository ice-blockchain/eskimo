package emaillink

import (
	_ "embed"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"io"
)

// Public API.
type (
	Processor interface {
		Repository
	}
	Repository interface {
		io.Closer
	}
)

// Private API.
const applicationYamlKey = "auth/email-link"

type (
	repository struct {
		db       *storage.DB
		cfg      *config
		shutdown func() error
	}
	processor struct {
		*repository
	}
	config struct{}
)

var (
	//go:embed DDL.sql
	ddl string
	cfg config
)
