package workflows

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/google/uuid"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
)

type PoststorageAPIS struct {
	cfg config.PoststorageConfig
}

func NewPoststorageAPIS(cfg config.PoststorageConfig) *PoststorageAPIS {
	return &PoststorageAPIS{cfg: cfg}
}

func (w *PoststorageAPIS) Execute(
	ctx temporalsdk_workflow.Context,
	params *childwf.PostStorageParams,
) (r *childwf.PostStorageResult, e error) {
	r = &childwf.PostStorageResult{}
	logger := temporalsdk_workflow.GetLogger(ctx)
	logger.Debug("Poststorage workflow running!", "params", params)

	defer func() {
		logger.Debug("Poststorage workflow finished!", "result", r, "error", e)
	}()

	aipUUID, err := uuid.Parse(params.AIPUUID)
	if err != nil {
		return nil, fmt.Errorf("parse AIP UUID: %v", err)
	}

	apisMetadata, err := getApisMetadata(params.CustomMetadata)
	if err != nil {
		return nil, err
	}

	// Activities running within a session.
	{
		var sessErr error
		maxAttempts := 5

		activityOpts := temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: time.Minute,
		})
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			sessCtx, err := temporalsdk_workflow.CreateSession(activityOpts, &temporalsdk_workflow.SessionOptions{
				CreationTimeout:  forever,
				ExecutionTimeout: forever,
			})
			if err != nil {
				return nil, fmt.Errorf("error creating session: %v", err)
			}

			sessErr = w.SessionHandler(sessCtx, r, aipUUID, apisMetadata, apisUsername(params.User))

			// We want to retry the session if it has been canceled as a result
			// of losing the worker but not otherwise. This scenario seems to be
			// identifiable when we have an error but the root context has not
			// been canceled.
			if sessErr != nil &&
				(errors.Is(sessErr, temporalsdk_workflow.ErrSessionFailed) || temporalsdk_temporal.IsCanceledError(sessErr)) {
				// Root context canceled, hence workflow canceled.
				if ctx.Err() == temporalsdk_workflow.ErrCanceled {
					return nil, nil
				}

				logger.Error(
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

	return r, nil
}

func (w *PoststorageAPIS) SessionHandler(
	ctx temporalsdk_workflow.Context,
	result *childwf.PostStorageResult,
	aipUUID uuid.UUID,
	apisMetadata apis.CustomMetadata,
	username string,
) (e error) {
	logger := temporalsdk_workflow.GetLogger(ctx)
	removePaths := []string{}

	defer func() {
		var removeResult removepaths.Result
		err := temporalsdk_workflow.ExecuteActivity(
			withFilesystemActivityOpts(ctx),
			removepaths.Name,
			&removepaths.Params{Paths: removePaths},
		).Get(ctx, &removeResult)
		if err != nil {
			e = errors.Join(e, err)
		}

		temporalsdk_workflow.CompleteSession(ctx)
	}()

	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Download AIP METS")
	var getAIPPathResult amss.GetAIPPathActivityResult
	e = temporalsdk_workflow.ExecuteActivity(
		temporalsdk_workflow.WithActivityOptions(
			ctx,
			temporalsdk_workflow.ActivityOptions{
				ScheduleToCloseTimeout: 15 * time.Minute,
				RetryPolicy: &temporalsdk_temporal.RetryPolicy{
					InitialInterval:    15 * time.Second,
					BackoffCoefficient: 2,
					MaximumInterval:    time.Minute,
					MaximumAttempts:    5,
				},
			},
		),
		amss.GetAIPPathActivityName,
		&amss.GetAIPPathActivityParams{
			AIPUUID: aipUUID,
		},
	).Get(ctx, &getAIPPathResult)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"AIP METS download has failed.",
			"The AIP is stored, but the poststorage workflow failed to get the AIP path. Please try again, or ask a system administrator to investigate.",
		)
		return nil
	}

	// In case the AIP is compressed, remove its UUID and the possible
	// extension from the directory/file name, and append the UUID back.
	aipUUIDString := aipUUID.String()
	aipDirName := strings.Split(filepath.Base(getAIPPathResult.Path), aipUUIDString)[0] + aipUUIDString
	localDir := filepath.Join(w.cfg.WorkingDir, aipUUIDString)
	metsName := fmt.Sprintf("METS.%s.xml", aipUUIDString)
	metsPath := filepath.Join(localDir, metsName)
	removePaths = append(removePaths, localDir)

	var fetchMETSResult amss.FetchActivityResult
	e = temporalsdk_workflow.ExecuteActivity(
		withActivityOptsForLongLivedRequest(ctx),
		amss.FetchActivityName,
		&amss.FetchActivityParams{
			AIPUUID:      aipUUID,
			RelativePath: fmt.Sprintf("%s/data/%s", aipDirName, metsName),
			Destination:  metsPath,
		},
	).Get(ctx, &fetchMETSResult)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"AIP METS download has failed.",
			"The AIP is stored, but the poststorage workflow failed while downloading the AIP METS file. Please try again, or ask a system administrator to investigate.",
		)
		return nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "AIP METS downloaded")

	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Submit AIP METS to APIS")
	importBehaviour, err := apisMetadata.ImportBehaviour()
	if err != nil {
		logger.Error("System error", "message", err.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"AIP METS submission to APIS has failed.",
			"The AIP is stored, but the post-storage workflow failed while preparing to deliver the AIP METS file to APIS for AIS import. Please try again, or ask a system administrator to investigate.",
		)
		return nil
	}

	var createImportRun apis.CreateImportRunResult
	err = temporalsdk_workflow.ExecuteActivity(
		withAPISActivityOpts(ctx),
		apis.CreateImportRunActivityName,
		&apis.CreateImportRunParams{
			TaskID:          apisMetadata.ImportTaskID,
			METSPath:        metsPath,
			ImportBehaviour: importBehaviour,
			Username:        username,
		},
	).Get(ctx, &createImportRun)
	if err != nil {
		logger.Error("System error", "message", err.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"AIP METS submission to APIS has failed.",
			"The AIP is stored, but the post-storage workflow failed while delivering the AIP METS file to APIS for AIS import. Please try again, or ask a system administrator to investigate.",
		)
		return nil
	}
	task.Succeed(
		temporalsdk_workflow.Now(ctx),
		"Submitted AIP METS to APIS with import task ID %q, import run ID %q, and import behaviour %q",
		apisMetadata.ImportTaskID,
		createImportRun.RunID,
		importBehaviour,
	)

	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Wait for APIS import")
	var pollImportRunStatus apis.PollImportRunStatusResult
	err = temporalsdk_workflow.ExecuteActivity(
		withAPISPollActivityOpts(ctx),
		apis.PollImportRunStatusActivityName,
		&apis.PollImportRunStatusParams{TaskID: apisMetadata.ImportTaskID},
	).Get(ctx, &pollImportRunStatus)
	if err != nil {
		logger.Error("System error", "message", err.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"APIS import status check has failed.",
			"The AIP is stored, but the post-storage workflow failed while checking whether APIS imported the AIP METS file for AIS. Please try again, or ask a system administrator to investigate.",
		)
		return nil
	}

	switch pollImportRunStatus.ImportResult {
	case apisgen.ImportResultErfolgreich:
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"APIS import completed for import task ID %q with result %q",
			apisMetadata.ImportTaskID,
			pollImportRunStatus.ImportResult,
		)
		return nil
	case apisgen.ImportResultFehler:
		logger.Error(
			"System error",
			"message",
			fmt.Sprintf("APIS import failed for task %q", apisMetadata.ImportTaskID),
		)
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"APIS import has failed.",
			"The AIP is stored, but APIS reported an error while importing the AIP METS file into AIS. Please try again, or ask a system administrator to investigate.",
		)
		return nil
	default:
		logger.Error("System error", "message", fmt.Errorf(
			"unexpected APIS import result %q for task %q",
			pollImportRunStatus.ImportResult,
			apisMetadata.ImportTaskID,
		))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"APIS import has failed.",
			"The AIP is stored, but APIS returned an unexpected import result while processing the AIP METS file. Please ask a system administrator to investigate.",
		)
		return nil
	}
}

func getApisMetadata(data childwf.CustomMetadata) (apis.CustomMetadata, error) {
	customMetadata, ok := data[apis.CustomMetadataKey]
	if !ok {
		return apis.CustomMetadata{}, fmt.Errorf("APIS custom metadata is required when APIS integration is enabled")
	}
	var apisMetadata apis.CustomMetadata
	if err := apisMetadata.Unmarshal(customMetadata); err != nil {
		return apis.CustomMetadata{}, err
	}
	return apisMetadata, nil
}
