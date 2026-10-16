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
	forever           = time.Hour * 24 * 365 * 10
	CreateDIPName     = "create-dip"
	maxExportAttempts = 3
	exportRetryDelay  = 10 * time.Second
)

type state struct {
	logger     temporalsdk_log.Logger
	workingDir string
	dipPath    string
	objectKey  string
	dip        datatypes.DIP
	aips       []*aip
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

	defer func() {
		if e == nil && ctx.Err() != nil {
			e = ctx.Err()
		}

		params := activities.UpdateDIPParams{
			UUID:      state.dip.UUID,
			Status:    enums.DIPStatusDone,
			ObjectKey: state.objectKey,
			Retention: max(0, w.retentionPeriod),
		}
		if e != nil {
			params.Status = enums.DIPStatusFailed
			params.Retention = 0
			params.ErrorMessage = state.dip.ErrorMessage
			if params.ErrorMessage == "" {
				params.ErrorMessage = activityErrorMessage(e)
			}
		}

		// Persist the final update even if the workflow was canceled.
		dctx, cancel := temporalsdk_workflow.NewDisconnectedContext(ctx)
		defer cancel()
		result, err := updateDIP(dctx, params)
		if err != nil {
			e = errors.Join(e, err)
		} else {
			state.dip = result.DIP
		}

		// Remove the object if generation or its final update failed, even after cancellation.
		if e != nil {
			if state.objectKey != "" {
				e = errors.Join(e, deleteDIP(dctx, state.dip.UUID, state.objectKey))
			}
			r = nil
			return
		}

		e = errors.Join(e, w.deleteDIPAfterRetention(ctx, state.dip.UUID, state.objectKey))
		if e != nil {
			r = nil
			return
		}
		r = &CreateDIPResult{DIP: state.dip}
	}()

	initial, err := updateDIP(ctx, activities.UpdateDIPParams{
		WorkflowID: temporalsdk_workflow.GetInfo(ctx).WorkflowExecution.ID,
		UUID:       state.dip.UUID,
		Status:     enums.DIPStatusInProgress,
	})
	if err != nil {
		state.dip.ErrorMessage = "DIP persistence update failed."
		return nil, err
	}
	state.dip = initial.DIP

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

	return &CreateDIPResult{DIP: state.dip}, nil
}

func updateDIP(
	ctx temporalsdk_workflow.Context,
	params activities.UpdateDIPParams,
) (activities.UpdateDIPResult, error) {
	var result activities.UpdateDIPResult
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForPersistenceOperation(ctx),
		activities.UpdateDIPName,
		&params,
	).Get(ctx, &result)
	return result, err
}

func (w *CreateDIP) deleteDIPAfterRetention(ctx temporalsdk_workflow.Context, id uuid.UUID, key string) error {
	for {
		// Retry the deletion request after each wait because the API may renew expiry.
		result, err := updateDIP(ctx, activities.UpdateDIPParams{
			UUID:   id,
			Status: enums.DIPStatusDeleted,
		})
		if err != nil {
			return err
		}

		// The activity leaves unexpired DIPs unchanged; use the returned status
		// to decide whether to delete the object or wait for the stored expiry.
		switch result.DIP.Status {
		case enums.DIPStatusDeleted, enums.DIPStatusFailed:
			return deleteDIP(ctx, id, key)
		case enums.DIPStatusDone:
			if result.DIP.ExpiresAt.IsZero() {
				return nil
			}
			if wait := result.DIP.ExpiresAt.Sub(temporalsdk_workflow.Now(ctx)); wait > 0 {
				if err := temporalsdk_workflow.Sleep(ctx, wait); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected cleanup status: %s", result.DIP.Status)
		}
	}
}

func deleteDIP(ctx temporalsdk_workflow.Context, id uuid.UUID, key string) error {
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForAPIRequest(ctx),
		bucketdelete.Name,
		&bucketdelete.Params{Key: key},
	).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete DIP archive: %v", err)
	}
	// Keep the key for later cleanup if deleting the object fails.
	_, err = updateDIP(ctx, activities.UpdateDIPParams{
		UUID:           id,
		ClearObjectKey: true,
	})
	return err
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

	metadataExportPath := filepath.Join(dipWorkingDir, "metadata.xml")
	err := w.fetchValidMetadata(ctx, state, metadataExportPath)
	if err != nil {
		return err
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
	var metadataFailures []string
	if len(metadata.ContentFiles) == 0 {
		metadataFailures = append(metadataFailures, "DIP metadata contains no content files.")
	}
	if len(metadata.HeaderFiles) == 0 {
		metadataFailures = append(metadataFailures, "DIP metadata contains no header files.")
	}
	if len(metadataFailures) > 0 {
		state.dip.ErrorMessage = strings.Join(metadataFailures, "\n")
		return errors.New(state.dip.ErrorMessage)
	}

	// Prepare the DIP header before adding content files.
	state.dipPath = filepath.Join(dipWorkingDir, "DIP_"+state.dip.UUID.String())
	err = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.PrepareDIPName,
		&activities.PrepareDIPParams{
			DIPPath:      state.dipPath,
			MetadataPath: metadataExportPath,
			XSDDir:       w.xsdDir,
			HeaderFiles:  metadata.HeaderFiles,
		},
	).Get(ctx, nil)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("DIP preparation failed: %s", activityErrorMessage(err))
		return err
	}

	// Download each content file directly into the DIP and validate its checksum.
	for _, file := range metadata.ContentFiles {
		err = temporalsdk_workflow.ExecuteActivity(
			withOptsForFileTransfer(ctx),
			amss.FetchActivityName,
			&amss.FetchActivityParams{
				AIPUUID:           file.AIPUUID,
				RelativePath:      file.AIPPath,
				Destination:       filepath.Join(state.dipPath, file.DIPPath),
				Checksum:          file.Checksum,
				ChecksumAlgorithm: file.ChecksumAlgorithm,
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

	// Track the key before uploading so cleanup also covers an uncertain upload result.
	state.objectKey = "DIP_" + state.dip.UUID.String() + ".zip"

	// Upload the ZIP before cleaning up the session files.
	var uploadResult bucketupload.Result
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForFileTransfer(ctx),
		bucketupload.Name,
		&bucketupload.Params{
			Path: state.dipPath,
			Key:  state.objectKey,
		},
	).Get(ctx, &uploadResult)
	if err != nil {
		state.dip.ErrorMessage = fmt.Sprintf("DIP upload failed: %s", activityErrorMessage(err))
		return err
	}

	return nil
}

// fetchValidMetadata will retry the whole metadata.xml export sequence up to maxExportAttempts
// times per session. An activity's retry policy applies only within an individual export attempt.
func (w *CreateDIP) fetchValidMetadata(
	ctx temporalsdk_workflow.Context,
	state *state,
	metadataExportPath string,
) error {
	var err error
	for attempt := 1; attempt <= maxExportAttempts; attempt++ {
		if attempt > 1 {
			if err := temporalsdk_workflow.Sleep(ctx, exportRetryDelay); err != nil {
				return err
			}
		}
		err = w.fetchMetadata(ctx, state.dip.DocKey, metadataExportPath)
		if err == nil {
			return nil
		}

		// Let the caller handle cancellation and session recovery. These must
		// not consume the remaining export attempts on an unusable session.
		if errors.Is(err, temporalsdk_workflow.ErrSessionFailed) || temporalsdk_temporal.IsCanceledError(err) {
			return err
		}

		state.logger.Warn("ACTApro export attempt failed", "attempt", attempt, "error", err)
	}

	state.dip.ErrorMessage = fmt.Sprintf(
		"ACTApro export failed after %d attempts:\n%s", maxExportAttempts, activityErrorMessage(err),
	)

	return errors.New(state.dip.ErrorMessage)
}

// fetchMetadata requests, polls, downloads and validates a single export.
func (w *CreateDIP) fetchMetadata(
	ctx temporalsdk_workflow.Context,
	docKey string,
	metadataExportPath string,
) error {
	// Start the document export.
	var export actapro.CreateExportResult
	err := temporalsdk_workflow.ExecuteActivity(
		withOptsForAPIRequest(ctx),
		actapro.CreateExportActivityName,
		&actapro.CreateExportParams{DocKey: docKey},
	).Get(ctx, &export)
	if err != nil {
		return err
	}

	// Poll the export until it reaches a terminal status.
	var exportStatus actapro.PollExportStatusResult
	err = temporalsdk_workflow.ExecuteActivity(
		temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: 24 * time.Hour,
			HeartbeatTimeout:    time.Minute,
			WaitForCancellation: true,
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
		return err
	}

	if exportStatus.Status != actapro.ExportStatusCompleted {
		errorMessage := fmt.Sprintf("export status: %s", exportStatus.Status)
		if exportStatus.Logs != "" {
			errorMessage += "\nLogs:\n" + exportStatus.Logs
		}
		return errors.New(errorMessage)
	}

	// Download the metadata.xml export.
	err = temporalsdk_workflow.ExecuteActivity(
		withOptsForFileTransfer(ctx),
		actapro.DownloadExportActivityName,
		&actapro.DownloadExportParams{
			ExportID:     export.ExportID,
			MetadataPath: metadataExportPath,
		},
	).Get(ctx, nil)
	if err != nil {
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
		return err
	}
	if len(validation.Failures) > 0 {
		return fmt.Errorf("metadata validation failed:\n%s", strings.Join(validation.Failures, "\n"))
	}

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
