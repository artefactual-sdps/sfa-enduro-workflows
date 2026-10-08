package workflows

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/artefactual-sdps/temporal-activities/archivezip"
	"github.com/artefactual-sdps/temporal-activities/bucketdelete"
	"github.com/artefactual-sdps/temporal-activities/bucketupload"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"github.com/google/uuid"
	temporalsdk_log "go.temporal.io/sdk/log"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
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
	dipPath    string
	dip        datatypes.DIP
	exportID   string
	aips       []*aip
	files      []*datatypes.File
}

type aip struct {
	// The UUID of the AIP from the ACTApro document.
	uuid uuid.UUID
	// The relative path of the AIP in the Archivematica Storage Service.
	relativePath string
	// The AIP directory name, without its archive extension.
	dirName string
	// The path to the METS file for the AIP in the local filesystem.
	metsPath string
}

type CreateDIPParams struct {
	DIP datatypes.DIP
}

type CreateDIPResult struct {
	DIP datatypes.DIP
}

type CreateDIP struct {
	workingDir      string
	xsdDir          string
	retentionPeriod time.Duration
}

func NewCreateDIP(workingDir, xsdDir string, retentionPeriod time.Duration) *CreateDIP {
	return &CreateDIP{workingDir: workingDir, xsdDir: xsdDir, retentionPeriod: retentionPeriod}
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

	// Run retention cleanup after the final DIP update, only on success.
	defer func() {
		if e != nil {
			return
		}
		if err := w.deleteDIPAfterRetention(ctx, state.dip.ObjectKey); err != nil {
			state.logger.Error("Failed to delete DIP", "err", err.Error())
		}
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
			// Remove the uploaded archive immediately if its final update could
			// not be persisted, even if the workflow was canceled.
			if state.dip.ObjectKey != "" {
				e = errors.Join(e, deleteDIP(dctx, state.dip.ObjectKey))
			}
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
		withOptsForAPIRequest(ctx),
		actapro.GetDocumentActivityName,
		&actapro.GetDocumentParams{DocKey: state.dip.DocKey},
	).Get(ctx, &document)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("ACTApro document retrieval failed: %s", activityErrorMessage(err))
		return nil, err
	}
	if len(document.AIPUUIDs) == 0 {
		state.dip.ErrorMessage = "ACTApro document contains no AIPs."
		return nil, errors.New(state.dip.ErrorMessage)
	}

	// Get AMSS AIP paths.
	var aipPathErrs error
	for _, aipUUID := range document.AIPUUIDs {
		var getAIPPathResult amss.GetAIPPathActivityResult
		err = temporalsdk_workflow.ExecuteActivity(
			withOptsForAPIRequest(ctx),
			amss.GetAIPPathActivityName,
			&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
		).Get(ctx, &getAIPPathResult)
		if err != nil {
			aipPathErrs = errors.Join(aipPathErrs, fmt.Errorf("AIP %s: %s", aipUUID, activityErrorMessage(err)))
			continue
		}
		state.aips = append(state.aips, &aip{uuid: aipUUID, relativePath: getAIPPathResult.Path})
	}
	if aipPathErrs != nil {
		state.dip.ErrorMessage = fmt.Sprintf("AMSS AIP path retrieval failed:\n%v", aipPathErrs)
		return nil, errors.New(state.dip.ErrorMessage)
	}

	// Start the document export.
	var export actapro.CreateExportResult
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForAPIRequest(ctx),
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

func (w *CreateDIP) deleteDIPAfterRetention(ctx temporalsdk_workflow.Context, key string) error {
	// Record the configured duration so worker configuration changes do not
	// alter a retention timer if the workflow is replayed.
	var retentionPeriod time.Duration
	if err := temporalsdk_workflow.SideEffect(ctx, func(temporalsdk_workflow.Context) any {
		return w.retentionPeriod
	}).Get(&retentionPeriod); err != nil {
		return fmt.Errorf("read retention period: %v", err)
	}
	if retentionPeriod < 0 {
		return nil
	}

	if err := temporalsdk_workflow.Sleep(ctx, retentionPeriod); err != nil {
		return fmt.Errorf("retention period timer failed: %v", err)
	}

	return deleteDIP(ctx, key)
}

func deleteDIP(ctx temporalsdk_workflow.Context, key string) error {
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForAPIRequest(ctx),
		bucketdelete.Name,
		&bucketdelete.Params{Key: key},
	).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete DIP archive: %v", err)
	}
	return nil
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
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForFileTransfer(ctx),
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

	// Validate the export against the configured schema.
	var validation xmlvalidate.Result
	err = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		xmlvalidate.Name,
		&xmlvalidate.Params{
			XMLPath: metadataExportPath,
			XSDPath: filepath.Join(w.xsdDir, "arelda.xsd"),
		},
	).Get(ctx, &validation)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("ACTApro export validation failed: %s", activityErrorMessage(err))
		return err
	}
	if len(validation.Failures) > 0 {
		state.dip.ErrorMessage = "ACTApro export validation failed:\n" + strings.Join(validation.Failures, "\n")
		return errors.New(state.dip.ErrorMessage)
	}

	// Download each AIP's METS file.
	var metsErrs error
	for _, aip := range state.aips {
		aipUUID := aip.uuid.String()
		// Strip any archive extension while preserving the AIP directory name.
		aip.dirName = strings.Split(filepath.Base(aip.relativePath), aipUUID)[0] + aipUUID
		metsName := fmt.Sprintf("METS.%s.xml", aipUUID)
		aip.metsPath = filepath.Join(dipWorkingDir, metsName)
		err = temporalsdk_workflow.ExecuteActivity(
			withOptsForFileTransfer(ctx),
			amss.FetchActivityName,
			&amss.FetchActivityParams{
				AIPUUID:      aip.uuid,
				RelativePath: fmt.Sprintf("%s/data/%s", aip.dirName, metsName),
				Destination:  aip.metsPath,
			},
		).Get(ctx, nil)
		if err != nil {
			metsErrs = errors.Join(metsErrs, fmt.Errorf("AIP %s: %s", aipUUID, activityErrorMessage(err)))
			continue
		}
	}
	if metsErrs != nil {
		state.dip.ErrorMessage = fmt.Sprintf("AMSS AIP METS download failed:\n%v", metsErrs)
		return errors.New(state.dip.ErrorMessage)
	}

	// Parse metadata export and AIP METS files.
	parseParams := &activities.ParseMetadataParams{MetadataPath: metadataExportPath}
	for _, aip := range state.aips {
		parseParams.AIPs = append(parseParams.AIPs, activities.AIP{
			UUID:     aip.uuid,
			DirName:  aip.dirName,
			METSPath: aip.metsPath,
		})
	}
	var metadata activities.ParseMetadataResult
	err = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ParseMetadataName,
		parseParams,
	).Get(ctx, &metadata)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("DIP metadata parsing failed: %s", activityErrorMessage(err))
		return err
	}
	if len(metadata.Files) == 0 {
		state.dip.ErrorMessage = "DIP metadata contains no files."
		return errors.New(state.dip.ErrorMessage)
	}
	state.files = metadata.Files

	// Prepare the DIP header before adding content files.
	state.dipPath = filepath.Join(dipWorkingDir, "DIP_"+state.dip.UUID.String())
	err = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.PrepareDIPName,
		&activities.PrepareDIPParams{
			DIPPath:      state.dipPath,
			MetadataPath: metadataExportPath,
			XSDDir:       w.xsdDir,
		},
	).Get(ctx, nil)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("DIP preparation failed: %s", activityErrorMessage(err))
		return err
	}

	// Download each content file directly into the DIP.
	for _, file := range state.files {
		err = temporalsdk_workflow.ExecuteActivity(
			withOptsForFileTransfer(ctx),
			amss.FetchActivityName,
			&amss.FetchActivityParams{
				AIPUUID:      file.AIPUUID,
				RelativePath: file.AIPPath,
				Destination:  filepath.Join(state.dipPath, file.DIPPath),
			},
		).Get(ctx, nil)
		if err != nil {
			state.dip.ErrorMessage = fmt.Sprintf(
				"AMSS content download failed for %q (AIP %s): %s",
				file.AIPPath, file.AIPUUID, activityErrorMessage(err),
			)
			return err
		}
	}

	// Create the ZIP alongside the DIP directory.
	var zipResult archivezip.Result
	err = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		archivezip.Name,
		&archivezip.Params{SourceDir: state.dipPath},
	).Get(ctx, &zipResult)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("DIP ZIP creation failed: %s", activityErrorMessage(err))
		return err
	}
	state.dipPath = zipResult.Path

	// Upload the ZIP before cleaning up the session files.
	var uploadResult bucketupload.Result
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForFileTransfer(ctx),
		bucketupload.Name,
		&bucketupload.Params{Path: state.dipPath},
	).Get(ctx, &uploadResult)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("DIP upload failed: %s", activityErrorMessage(err))
		return err
	}
	state.dip.ObjectKey = uploadResult.Key

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

func withOptsForAPIRequest(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporalsdk_temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})
}

func withOptsForFileTransfer(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		HeartbeatTimeout:    10 * time.Second,
		// Wait for the transfer to stop before removing the session files.
		WaitForCancellation: true,
		RetryPolicy: &temporalsdk_temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})
}

func withFilesystemActivityOpts(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		// Wait for filesystem operations to stop before removing session files.
		WaitForCancellation: true,
		RetryPolicy: &temporalsdk_temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
}
