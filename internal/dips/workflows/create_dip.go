package workflows

import (
	"errors"
	"fmt"
	"time"

	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
)

const CreateDIPName = "create-dip"

type CreateDIPParams struct {
	DIP datatypes.DIP
}

type CreateDIPResult struct {
	DIP datatypes.DIP
}

type CreateDIP struct{}

func NewCreateDIP() *CreateDIP {
	return &CreateDIP{}
}

func (a *CreateDIP) Execute(ctx temporalsdk_workflow.Context, params *CreateDIPParams) (r *CreateDIPResult, e error) {
	logger := temporalsdk_workflow.GetLogger(ctx)
	logger.Debug("Create DIP workflow running!", "params", params)
	defer func() {
		logger.Debug("Create DIP workflow finished!", "result", r, "error", e)
	}()

	r = &CreateDIPResult{DIP: params.DIP}
	r.DIP.Status = enums.DIPStatusInProgress
	r.DIP.StartedAt = temporalsdk_workflow.Now(ctx)

	// Record the final DIP update.
	defer func() {
		// The DIP's object key and error message are updated before this.
		r.DIP.CompletedAt = temporalsdk_workflow.Now(ctx)
		r.DIP.Status = enums.DIPStatusDone
		if e != nil {
			r.DIP.Status = enums.DIPStatusFailed
		}
		// Persist the final update even if the workflow was canceled.
		dctx, cancel := temporalsdk_workflow.NewDisconnectedContext(ctx)
		defer cancel()
		err := temporalsdk_workflow.ExecuteActivity(
			withOptsForRequest(dctx),
			activities.UpdateDIPName,
			&activities.UpdateDIPParams{DIP: r.DIP},
		).Get(dctx, nil)
		if err != nil {
			e = errors.Join(e, err)
		}
	}()

	// Initial DIP update.
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForRequest(ctx),
		activities.UpdateDIPName,
		&activities.UpdateDIPParams{DIP: r.DIP},
	).Get(ctx, nil)
	if err != nil {
		r.DIP.ErrorMessage = "DIP persistence update failed."
		return r, err
	}

	// TODO: Add session handling, ACTAPro activities, DIP generation, bucket upload, etc.
	r.DIP.ObjectKey = fmt.Sprintf("DIP_%s.zip", r.DIP.UUID.String())

	return r, nil
}

func withOptsForRequest(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(
		ctx,
		temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: time.Second * 10,
			WaitForCancellation: true,
			RetryPolicy: &temporalsdk_temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2,
				MaximumAttempts:    3,
			},
		},
	)
}
