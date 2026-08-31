package client

import (
	"github.com/go-logr/logr"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
)

type client struct {
	logger logr.Logger
	ent    *db.Client
}

var _ persistence.Service = (*client)(nil)

func New(logger logr.Logger, ent *db.Client) persistence.Service {
	return &client{logger: logger, ent: ent}
}
