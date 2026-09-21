package workflows

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/artefactual-sdps/temporal-activities/removepaths"
	temporalsdk_log "go.temporal.io/sdk/log"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
)

const (
	// We use this constant to represent a long period of time (10 years).
	forever       = time.Hour * 24 * 365 * 10
	CreateDIPName = "create-dip"
)

type state struct {
	logger     temporalsdk_log.Logger
	workingDir string
	dip        datatypes.DIP
	exportID   string
}

type CreateDIPParams struct {
	DIP datatypes.DIP
}

type CreateDIPResult struct {
	DIP datatypes.DIP
}

type CreateDIP struct {
	workingDir string
}

func NewCreateDIP(workingDir string) *CreateDIP {
	return &CreateDIP{workingDir: workingDir}
}

func (w *CreateDIP) Execute(ctx temporalsdk_workflow.Context, params *CreateDIPParams) (r *CreateDIPResult, e error) {
	state := &state{
		logger:     temporalsdk_workflow.GetLogger(ctx),
		workingDir: w.workingDir,
		dip:        params.DIP,
	}

	state.logger.Debug("Create DIP workflow running!", "params", params)
	defer func() {
		state.logger.Debug("Create DIP workflow finished!", "result", r, "error", e)
	}()

	// Record the final DIP update.
	defer func() {
		// The DIP's object key and error message are updated before this.
		state.dip.CompletedAt = temporalsdk_workflow.Now(ctx)
		state.dip.Status = enums.DIPStatusDone
		if e != nil {
			state.dip.Status = enums.DIPStatusFailed
		}
		// Persist the final update even if the workflow was canceled.
		dctx, cancel := temporalsdk_workflow.NewDisconnectedContext(ctx)
		defer cancel()
		err := temporalsdk_workflow.ExecuteActivity(
			withOptsForPersistenceOperation(dctx),
			activities.UpdateDIPName,
			&activities.UpdateDIPParams{DIP: state.dip},
		).Get(dctx, nil)
		if err != nil {
			e = errors.Join(e, err)
		}
		if r != nil {
			// The return expression copies state.dip before this defer runs.
			// Refresh the result with the final status and completion time.
			r.DIP = state.dip
		}
	}()

	// Initial DIP update.
	state.dip.Status = enums.DIPStatusInProgress
	state.dip.StartedAt = temporalsdk_workflow.Now(ctx)
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForPersistenceOperation(ctx),
		activities.UpdateDIPName,
		&activities.UpdateDIPParams{DIP: state.dip},
	).Get(ctx, nil)
	if err != nil {
		state.dip.ErrorMessage = "DIP persistence update failed."
		return nil, err
	}

	// Retrieve the document from ACTApro.
	var document actapro.GetDocumentResult
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForACTAproRequest(ctx),
		actapro.GetDocumentActivityName,
		&actapro.GetDocumentParams{DocKey: state.dip.DocKey},
	).Get(ctx, &document)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("ACTApro document retrieval failed: %s", activityErrorMessage(err))
		return nil, err
	}

	// Start the document export.
	var export actapro.CreateExportResult
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForACTAproRequest(ctx),
		actapro.CreateExportActivityName,
		&actapro.CreateExportParams{DocKey: state.dip.DocKey},
	).Get(ctx, &export)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("ACTApro export creation failed: %s", activityErrorMessage(err))
		return nil, err
	}

	state.exportID = export.ExportID

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
		&actapro.PollExportStatusParams{ExportID: state.exportID},
	).Get(ctx, &exportStatus)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("ACTApro export polling failed: %s", activityErrorMessage(err))
		return nil, err
	}

	if exportStatus.Status != actapro.ExportStatusCompleted {
		state.dip.ErrorMessage = "ACTApro export failed or canceled."
		if exportStatus.Logs != "" {
			state.dip.ErrorMessage += " Logs:\n" + exportStatus.Logs
		}
		return nil, errors.New(state.dip.ErrorMessage)
	}

	// Activities running within a session.
	{
		var sessErr error
		maxAttempts := 5

		ctx = temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: time.Minute,
		})
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			sessCtx, err := temporalsdk_workflow.CreateSession(ctx, &temporalsdk_workflow.SessionOptions{
				CreationTimeout:  forever,
				ExecutionTimeout: forever,
			})
			if err != nil {
				return nil, fmt.Errorf("error creating session: %v", err)
			}

			sessErr = w.sessionHandler(sessCtx, state)

			// We want to retry the session if it has been canceled as a result
			// of losing the worker but not otherwise. This scenario seems to be
			// identifiable when we have an error but the root context has not
			// been canceled.
			if sessErr != nil &&
				(errors.Is(sessErr, temporalsdk_workflow.ErrSessionFailed) || temporalsdk_temporal.IsCanceledError(sessErr)) {
				// Root context canceled, hence workflow canceled.
				if ctx.Err() == temporalsdk_workflow.ErrCanceled {
					return nil, ctx.Err()
				}

				state.logger.Error(
					"Session failed, will retry shortly (10s)...",
					"err", ctx.Err(),
					"attemptFailed", attempt,
					"attemptsLeft", maxAttempts-attempt,
				)

				_ = temporalsdk_workflow.Sleep(ctx, time.Second*10)

				continue
			}
			break
		}

		if sessErr != nil {
			return nil, sessErr
		}
	}

	// A successful session may follow a failed attempt.
	state.dip.ErrorMessage = ""
	return &CreateDIPResult{DIP: state.dip}, nil
}

// sessionHandler runs activities that belong to the same session.
func (w *CreateDIP) sessionHandler(ctx temporalsdk_workflow.Context, state *state) error {
	// Cleanup session files on exit.
	dipWorkingDir := filepath.Join(state.workingDir, state.dip.UUID.String())
	defer func() {
		// Allow cleanup to finish even if the workflow was canceled.
		dctx, cancel := temporalsdk_workflow.NewDisconnectedContext(ctx)
		defer cancel()
		opts := temporalsdk_workflow.WithActivityOptions(dctx, temporalsdk_workflow.ActivityOptions{
			ScheduleToCloseTimeout: 15 * time.Minute,
			RetryPolicy: &temporalsdk_temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		})

		err := temporalsdk_workflow.ExecuteActivity(
			opts,
			removepaths.Name,
			removepaths.Params{Paths: []string{dipWorkingDir}},
		).Get(opts, nil)
		if err != nil {
			state.logger.Error(
				"session cleanup: error(s) removing temporary directories",
				"errors", err.Error(),
			)
		}

		temporalsdk_workflow.CompleteSession(opts)
	}()

	// Download the metadata.xml export.
	metadataExportPath := filepath.Join(dipWorkingDir, "metadata.xml")
	downloadCtx := temporalsdk_workflow.WithHeartbeatTimeout(withOptsForACTAproRequest(ctx), 10*time.Second)
	// Wait for the download to stop before removing the session files.
	downloadCtx = temporalsdk_workflow.WithWaitForCancellation(downloadCtx, true)
	err := temporalsdk_workflow.ExecuteActivity(
		downloadCtx,
		actapro.DownloadExportActivityName,
		&actapro.DownloadExportParams{
			ExportID:     state.exportID,
			MetadataPath: metadataExportPath,
		},
	).Get(ctx, nil)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("ACTApro export download failed: %s", activityErrorMessage(err))
		return err
	}

	// TODO: Add export validation.

	// TODO: Add DIP generation and bucket upload.
	state.dip.ObjectKey = fmt.Sprintf("DIP_%s.zip", state.dip.UUID.String())

	return nil
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
