package client

import (
	"errors"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
)

func rollback(tx *db.Tx, err error) error {
	rerr := tx.Rollback()
	if rerr == nil {
		return err
	}

	return errors.Join(
		err,
		fmt.Errorf("%w: failed transaction rollback: %v", persistence.ErrInternal, rerr),
	)
}
