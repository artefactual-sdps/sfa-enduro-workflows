package client

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db/dip"
)

// RequestDIP selects the newest eligible pending or cached DIP, or creates a new generation.
// Retention is only applied when reusing a completed DIP. For new, queued, or
// in-progress DIPs, no expiry is set here; the workflow sets it on completion
// using its configured retention period.
func (c *client) RequestDIP(ctx context.Context, req persistence.RequestDIPParams) (*datatypes.DIP, error) {
	if req.UUID == uuid.Nil {
		return nil, newRequiredFieldError("UUID")
	}
	if req.DocKey == "" {
		return nil, newRequiredFieldError("DocKey")
	}
	if req.Retention < 0 {
		return nil, fmt.Errorf("%w: retention must not be negative", persistence.ErrNotValid)
	}

	// Repeatable read provides gap locks for absent document keys, preventing
	// concurrent requests from both creating a generation for the same key.
	tx, err := c.ent.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, newDBError(err)
	}

	// Lock candidates before selecting a pending or usable cached generation.
	rows, err := tx.DIP.Query().Where(
		dip.DocKey(req.DocKey),
		dip.StatusIn(enums.DIPStatusQueued, enums.DIPStatusInProgress, enums.DIPStatusDone),
	).Order(db.Desc(dip.FieldCreatedAt)).ForUpdate().All(ctx)
	if err != nil {
		return nil, rollback(tx, newDBError(err))
	}

	now := time.Now().UTC()
	var selected *db.DIP
	for _, d := range rows {
		// Skip expired cached DIPs, or all cached DIPs when ignoring the cache.
		if d.Status == enums.DIPStatusDone &&
			(req.IgnoreCache || (!d.ExpiresAt.IsZero() && !d.ExpiresAt.After(now))) {
			continue
		}
		selected = d
		break
	}

	// If the selected generation is cached, update its expiration without reducing it.
	selectedUpdated := false
	if selected != nil && selected.Status == enums.DIPStatusDone && !selected.ExpiresAt.IsZero() {
		next := now.Add(req.Retention)
		if req.Retention == 0 || next.After(selected.ExpiresAt) {
			q := tx.DIP.UpdateOne(selected)
			if req.Retention == 0 {
				q.ClearExpiresAt()
			} else {
				q.SetExpiresAt(next)
			}
			selected, err = q.Save(ctx)
			if err != nil {
				return nil, rollback(tx, newDBError(err))
			}
			selectedUpdated = true
		}
	}

	if selected != nil {
		// Commit expiry changes; otherwise just release the locks.
		if selectedUpdated {
			if err := tx.Commit(); err != nil {
				return nil, newDBError(err)
			}
		} else {
			if err := rollback(tx, nil); err != nil {
				return nil, err
			}
		}
		return datatypes.NewDIP(selected), nil
	}

	// If no pending or cached generation was selected, create a new DIP.
	selected, err = tx.DIP.Create().
		SetUUID(req.UUID).
		SetDocKey(req.DocKey).
		SetStatus(enums.DIPStatusQueued).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, rollback(tx, newDBError(err))
	}

	if err := tx.Commit(); err != nil {
		// MySQL may commit successfully even if the driver fails to read its response.
		// Recover a newly created DIP so its workflow is still submitted; otherwise
		// later requests would keep reusing a queued DIP with no workflow.
		d, readErr := c.ReadDIP(ctx, req.UUID)
		if readErr == nil {
			return d, nil
		}

		// database/sql marks the transaction done before calling the driver's Commit;
		// a rollback afterward cannot undo the commit and returns sql.ErrTxDone.
		return nil, newDBError(err)
	}

	return datatypes.NewDIP(selected), nil
}

// ReadDIP returns the DIP identified by id.
func (c *client) ReadDIP(ctx context.Context, id uuid.UUID) (*datatypes.DIP, error) {
	dbd, err := c.ent.DIP.Query().Where(dip.UUID(id)).Only(ctx)
	if err != nil {
		return nil, newDBError(err)
	}

	return datatypes.NewDIP(dbd), nil
}

// UpdateDIP updates the persisted DIP identified by id using updater and
// returns the updated DIP. The row is locked before calling updater.
//
// DBID, UUID, DocKey, and CreatedAt cannot be updated with this method.
func (c *client) UpdateDIP(
	ctx context.Context,
	id uuid.UUID,
	updater persistence.DIPUpdater,
) (*datatypes.DIP, error) {
	tx, err := c.ent.BeginTx(ctx, nil)
	if err != nil {
		return nil, newDBError(err)
	}

	dbd, err := tx.DIP.Query().Where(dip.UUID(id)).ForUpdate().Only(ctx)
	if err != nil {
		return nil, rollback(tx, newDBError(err))
	}

	dbID := dbd.ID

	up, err := updater(datatypes.NewDIP(dbd))
	if err != nil {
		return nil, rollback(tx, newUpdaterError(err))
	}

	q := tx.DIP.UpdateOneID(dbID)

	if up.Status.IsValid() {
		q.SetStatus(up.Status)
	}
	if up.ErrorMessage != "" {
		q.SetErrorMessage(up.ErrorMessage)
	}
	if !up.StartedAt.IsZero() {
		q.SetStartedAt(up.StartedAt)
	}
	if !up.CompletedAt.IsZero() {
		q.SetCompletedAt(up.CompletedAt)
	}
	if !up.ExpiresAt.IsZero() && up.ExpiresAt.After(dbd.ExpiresAt) {
		q.SetExpiresAt(up.ExpiresAt)
	}
	if up.WorkflowID != "" {
		q.SetWorkflowID(up.WorkflowID)
	}
	if up.ObjectKey != "" {
		q.SetObjectKey(up.ObjectKey)
	} else {
		q.ClearObjectKey()
	}

	dbd, err = q.Save(ctx)
	if err != nil {
		return nil, rollback(tx, newDBError(err))
	}
	if err = tx.Commit(); err != nil {
		// database/sql marks the transaction done before calling the driver's Commit;
		// a rollback afterward cannot undo the commit and returns sql.ErrTxDone.
		return nil, newDBError(err)
	}

	return datatypes.NewDIP(dbd), nil
}

// DeleteDIP deletes the persisted DIP identified by id.
func (c *client) DeleteDIP(ctx context.Context, id uuid.UUID) error {
	n, err := c.ent.DIP.Delete().Where(dip.UUID(id)).Exec(ctx)
	if err != nil {
		return newDBErrorWithDetails(err, "delete DIP")
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", persistence.ErrNotFound, "db: dip not found: delete DIP")
	}

	return nil
}
