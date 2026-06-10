package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence/ent/db"
)

type client struct {
	ent *db.Client
}

var _ persistence.Service = (*client)(nil)

// New returns a new ent client that implements the persistence service.
func New(ent *db.Client) persistence.Service {
	return &client{ent: ent}
}

func (c *client) CreateSIP(ctx context.Context, name, checksum string) error {
	if name == "" {
		return errors.New("CreateSIP: name field is required")
	}
	if checksum == "" {
		return errors.New("CreateSIP: checksum field is required")
	}

	err := c.ent.SIP.Create().
		SetName(name).
		SetChecksum(checksum).
		Exec(ctx)
	if err != nil {
		if db.IsConstraintError(err) {
			err = persistence.ErrDuplicatedSIP
		}
		return fmt.Errorf("CreateSIP: %w", err)
	}

	return nil
}
