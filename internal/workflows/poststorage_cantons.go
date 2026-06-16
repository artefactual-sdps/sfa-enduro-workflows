package workflows

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/archivezip"
	"github.com/artefactual-sdps/temporal-activities/bucketupload"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/google/uuid"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/cantons"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
)

type PoststorageCantons struct {
	cfg config.PoststorageConfig
}

func NewPoststorageCantons(cfg config.PoststorageConfig) *PoststorageCantons {
	return &PoststorageCantons{cfg: cfg}
}

func (w *PoststorageCantons) Execute(
	ctx temporalsdk_workflow.Context,
	params *childwf.PostStorageParams,
) (r *childwf.PostStorageResult, e error) {
	logger := temporalsdk_workflow.GetLogger(ctx)
	logger.Debug("Poststorage Cantons workflow running!", "params", params)

	defer func() {
		logger.Debug("Poststorage Cantons workflow finished!", "result", r, "error", e)
	}()

	aipUUID, err := uuid.Parse(params.AIPUUID)
	if err != nil {
		return nil, fmt.Errorf("parse AIP UUID: %v", err)
	}

	var getAIPPathResult amss.GetAIPPathActivityResult
	err = temporalsdk_workflow.ExecuteActivity(
		temporalsdk_workflow.WithActivityOptions(
			ctx,
			temporalsdk_workflow.ActivityOptions{
				ScheduleToCloseTimeout: 10 * time.Minute,
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

			sessErr = w.SessionHandler(sessCtx, aipUUID, getAIPPathResult.Path)

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

	return &childwf.PostStorageResult{}, nil
}

func (w *PoststorageCantons) SessionHandler(
	ctx temporalsdk_workflow.Context,
	aipUUID uuid.UUID,
	aipPath string,
) (e error) {
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

	aipUUIDString := aipUUID.String()
	// In case the AIP is compressed, remove its UUID and the possible
	// extension from the directory/file name, and append the UUID back.
	aipDirName := strings.Split(filepath.Base(aipPath), aipUUIDString)[0] + aipUUIDString
	localDir := filepath.Join(w.cfg.WorkingDir, fmt.Sprintf("search-md_%s", aipDirName))
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
		return e
	}

	var parseResult cantons.ParseActivityResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		cantons.ParseActivityName,
		&cantons.ParseActivityParams{METSPath: metsPath},
	).Get(ctx, &parseResult)
	if e != nil {
		return e
	}

	var metadataRelPath string
	if parseResult.UpdatedAreldaMetadataRelPath != "" {
		metadataRelPath = parseResult.UpdatedAreldaMetadataRelPath
	} else if parseResult.MetadataRelPath != "" {
		metadataRelPath = parseResult.MetadataRelPath
	} else {
		return errors.New("UpdatedAreldaMetadata.xml and metadata.xml files not found in METS")
	}

	metadataPath := filepath.Join(localDir, filepath.Base(metadataRelPath))

	var fetchMetadataResult amss.FetchActivityResult
	e = temporalsdk_workflow.ExecuteActivity(
		withActivityOptsForLongLivedRequest(ctx),
		amss.FetchActivityName,
		&amss.FetchActivityParams{
			AIPUUID:      aipUUID,
			RelativePath: fmt.Sprintf("%s/data/%s", aipDirName, metadataRelPath),
			Destination:  metadataPath,
		},
	).Get(ctx, &fetchMetadataResult)
	if e != nil {
		return e
	}

	var combineMDResult cantons.CombineMDActivityResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		cantons.CombineMDActivityName,
		cantons.CombineMDActivityParams{
			AreldaPath: metadataPath,
			METSPath:   metsPath,
			LocalDir:   localDir,
		},
	).Get(ctx, &combineMDResult)
	if e != nil {
		return e
	}

	var zipResult archivezip.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		archivezip.Name,
		&archivezip.Params{SourceDir: localDir},
	).Get(ctx, &zipResult)
	if e != nil {
		return e
	}

	removePaths = append(removePaths, zipResult.Path)

	var uploadResult bucketupload.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withActivityOptsForLongLivedRequest(ctx),
		bucketupload.Name,
		&bucketupload.Params{Path: zipResult.Path},
	).Get(ctx, &uploadResult)
	if e != nil {
		return e
	}

	return nil
}
