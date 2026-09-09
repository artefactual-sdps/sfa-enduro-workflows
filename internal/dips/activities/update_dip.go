package activities

import (
	"context"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

const UpdateDIPName = "update-dip"

type UpdateDIPParams struct {
	DIP datatypes.DIP
}

type UpdateDIPResult struct{}

type UpdateDIP struct {
	perSvc persistence.Service
}

func NewUpdateDIP(perSvc persistence.Service) *UpdateDIP {
	return &UpdateDIP{perSvc: perSvc}
}

func (a *UpdateDIP) Execute(ctx context.Context, params *UpdateDIPParams) (*UpdateDIPResult, error) {
	updater := func(d *datatypes.DIP) (*datatypes.DIP, error) {
		if params.DIP.Status.IsValid() {
			d.Status = params.DIP.Status
		}
		if !params.DIP.StartedAt.IsZero() {
			d.StartedAt = params.DIP.StartedAt
		}
		if !params.DIP.CompletedAt.IsZero() {
			d.CompletedAt = params.DIP.CompletedAt
		}
		if params.DIP.ErrorMessage != "" {
			d.ErrorMessage = params.DIP.ErrorMessage
		}
		if params.DIP.ObjectKey != "" {
			d.ObjectKey = params.DIP.ObjectKey
		}
		return d, nil
	}
	_, err := a.perSvc.UpdateDIP(ctx, params.DIP.UUID, updater)
	if err != nil {
		return nil, fmt.Errorf("failed to update DIP: %v", err)
	}
	return &UpdateDIPResult{}, nil
}
