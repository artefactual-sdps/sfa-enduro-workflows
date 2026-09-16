package workflows

import (
	"errors"
	"fmt"
	"time"

	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
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
			withOptsForPersistenceOperation(dctx),
			activities.UpdateDIPName,
			&activities.UpdateDIPParams{DIP: r.DIP},
		).Get(dctx, nil)
		if err != nil {
			e = errors.Join(e, err)
		}
	}()

	// Initial DIP update.
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForPersistenceOperation(ctx),
		activities.UpdateDIPName,
		&activities.UpdateDIPParams{DIP: r.DIP},
	).Get(ctx, nil)
	if err != nil {
		r.DIP.ErrorMessage = "DIP persistence update failed."
		return r, err
	}

	// Retrieve the document from ACTApro.
	var document actapro.GetDocumentResult
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForACTAproRequest(ctx),
		actapro.GetDocumentActivityName,
		&actapro.GetDocumentParams{DocKey: r.DIP.DocKey},
	).Get(ctx, &document)
	if err != nil {
		r.DIP.ErrorMessage = fmt.Sprintf("ACTApro document retrieval failed: %s", activityErrorMessage(err))
		return r, err
	}

	// Start the document export.
	var export actapro.CreateExportResult
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForACTAproRequest(ctx),
		actapro.CreateExportActivityName,
		&actapro.CreateExportParams{DocKey: r.DIP.DocKey},
	).Get(ctx, &export)
	if err != nil {
		r.DIP.ErrorMessage = fmt.Sprintf("ACTApro export creation failed: %s", activityErrorMessage(err))
		return r, err
	}

	// Poll the export status until it is completed, failed, or canceled.
	var exportStatus actapro.PollExportStatusResult
	err = temporalsdk_workflow.ExecuteActivity(
		temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: 24 * time.Hour,
			HeartbeatTimeout:    time.Minute,
			RetryPolicy: &temporalsdk_temporal.RetryPolicy{
				InitialInterval:    5 * time.Second,
				BackoffCoefficient: 2,
				MaximumAttempts:    3,
			},
		}),
		actapro.PollExportStatusActivityName,
		&actapro.PollExportStatusParams{ExportID: export.ExportID},
	).Get(ctx, &exportStatus)
	if err != nil {
		r.DIP.ErrorMessage = fmt.Sprintf("ACTApro export polling failed: %s", activityErrorMessage(err))
		return r, err
	}

	if exportStatus.Status != actapro.ExportStatusCompleted {
		r.DIP.ErrorMessage = "ACTApro export failed or canceled."
		if exportStatus.Logs != "" {
			r.DIP.ErrorMessage += " Logs:\n" + exportStatus.Logs
		}
		return r, errors.New(r.DIP.ErrorMessage)
	}

	// TODO: Add session handling, export download, DIP generation, bucket upload, etc.
	r.DIP.ObjectKey = fmt.Sprintf("DIP_%s.zip", r.DIP.UUID.String())

	return r, nil
}

func activityErrorMessage(err error) string {
	var applicationErr *temporalsdk_temporal.ApplicationError
	if errors.As(err, &applicationErr) {
		return applicationErr.Message()
	}
	return err.Error()
}

func withOptsForPersistenceOperation(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
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

func withOptsForACTAproRequest(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporalsdk_temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})
}
