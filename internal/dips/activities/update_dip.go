package activities

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

const UpdateDIPName = "update-dip"

type UpdateDIPParams struct {
	UUID           uuid.UUID
	Status         enums.DIPStatus
	ObjectKey      string
	ErrorMessage   string
	Retention      time.Duration
	WorkflowID     string
	ClearObjectKey bool
}

type UpdateDIPResult struct {
	DIP datatypes.DIP
}

type UpdateDIP struct {
	perSvc persistence.Service
}

func NewUpdateDIP(perSvc persistence.Service) *UpdateDIP {
	return &UpdateDIP{perSvc: perSvc}
}

func (a *UpdateDIP) Execute(ctx context.Context, params *UpdateDIPParams) (*UpdateDIPResult, error) {
	updater := func(d *datatypes.DIP) (*datatypes.DIP, error) {
		if params.ClearObjectKey {
			if d.Status == enums.DIPStatusFailed || d.Status == enums.DIPStatusDeleted {
				d.ObjectKey = ""
			}
			return d, nil
		}

		// The database update may succeed even if Temporal does not receive the result.
		// On retry, leave completed generations unchanged: publishing again would reset
		// completion and expiry times, possibly overwriting an API renewal. Likewise,
		// do not overwrite a failed or deleted generation. Deletion is handled below
		// because it must still check whether a completed generation has expired.
		if params.Status != enums.DIPStatusDeleted &&
			(d.Status == enums.DIPStatusDone || d.Status == enums.DIPStatusFailed || d.Status == enums.DIPStatusDeleted) {
			return d, nil
		}

		now := time.Now().UTC()
		switch params.Status {
		case enums.DIPStatusInProgress:
			d.Status = params.Status
			d.StartedAt = now
			d.WorkflowID = params.WorkflowID
		case enums.DIPStatusDone:
			if params.ObjectKey == "" || params.Retention < 0 {
				return nil, fmt.Errorf("%w: invalid DIP publication", persistence.ErrNotValid)
			}
			d.Status = params.Status
			d.ObjectKey = params.ObjectKey
			d.CompletedAt = now
			if params.Retention > 0 {
				d.ExpiresAt = now.Add(params.Retention)
			}
		case enums.DIPStatusFailed:
			d.Status = params.Status
			d.ObjectKey = params.ObjectKey
			d.ErrorMessage = params.ErrorMessage
			d.CompletedAt = now
		case enums.DIPStatusDeleted:
			// Check expiry under the row lock to avoid racing with API renewals.
			// Return the DIP unchanged if it is not eligible for deletion.
			if d.Status == enums.DIPStatusDone && !d.ExpiresAt.IsZero() && !d.ExpiresAt.After(now) {
				d.Status = params.Status
			}
		default:
			return nil, fmt.Errorf("%w: invalid DIP update status", persistence.ErrNotValid)
		}

		return d, nil
	}

	d, err := a.perSvc.UpdateDIP(ctx, params.UUID, updater)
	if err != nil {
		if errors.Is(err, persistence.ErrNotValid) || errors.Is(err, persistence.ErrNotFound) {
			return nil, temporal.NewNonRetryableError(fmt.Errorf("update DIP: %v", err))
		}

		return nil, err
	}

	return &UpdateDIPResult{DIP: *d}, nil
}
