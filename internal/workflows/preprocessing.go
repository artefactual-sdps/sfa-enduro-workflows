package workflows

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/archiveextract"
	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/bagvalidate"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"go.artefactual.dev/tools/fsutil"
	"go.artefactual.dev/tools/temporal"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/localact"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

type Preprocessing struct {
	cfg         config.PreprocessingConfig
	apisEnabled bool
}

const APISDefaultValuesFailureMessage = "No matching SIP information for this accession number could be found " +
	"in the AIS, causing the analysis to fail. Please check if the accession information in the SIP matches with " +
	"the record in the AIS and if it is linked with a (Teil-)Bestand and then restart the ingest."

func NewPreprocessing(
	cfg config.PreprocessingConfig,
	apisEnabled bool,
) *Preprocessing {
	return &Preprocessing{
		cfg:         cfg,
		apisEnabled: apisEnabled,
	}
}

func (w *Preprocessing) Execute(
	ctx temporalsdk_workflow.Context,
	params *childwf.PreprocessingParams,
) (*childwf.PreprocessingResult, error) {
	var e error
	result := &childwf.PreprocessingResult{}
	logger := temporalsdk_workflow.GetLogger(ctx)
	logger.Debug("Preprocessing workflow running!", "params", params)

	defer func() {
		logger.Debug("Preprocessing workflow finished!", "result", result, "error", e)
	}()

	if params == nil || params.RelativePath == "" {
		e = temporal.NewNonRetryableError(
			fmt.Errorf("error calling workflow with unexpected inputs"),
		)
		return nil, e
	}
	result.RelativePath = params.RelativePath

	localPath := filepath.Join(w.cfg.SharedPath, filepath.Clean(params.RelativePath))

	// Extract SIP.
	localPath = w.extractSIP(ctx, result, localPath, params.SIPName)
	if result.Outcome == childwf.OutcomeSystemError {
		return result, nil
	}

	// Check if the SIP is a BagIt bag.
	var isBag localact.IsBagResult
	e = temporalsdk_workflow.ExecuteLocalActivity(
		withLocalActOpts(ctx),
		localact.IsBag,
		&localact.IsBagParams{Path: localPath},
	).Get(ctx, &isBag)
	if e != nil {
		return nil, fmt.Errorf("bag check: %v", e)
	}

	// Unbag the SIP if it is a bag.
	if isBag.IsBag {
		task := result.NewTask(temporalsdk_workflow.Now(ctx), "Validate Bag")
		var bagValidateResult bagvalidate.Result
		e = temporalsdk_workflow.ExecuteActivity(
			withFilesystemActivityOpts(ctx),
			bagvalidate.Name,
			&bagvalidate.Params{Path: localPath},
		).Get(ctx, &bagValidateResult)
		if e != nil {
			logger.Error("System error", "message", e.Error())
			result.SystemError(
				temporalsdk_workflow.Now(ctx),
				task,
				"Bag validation has failed.",
				"An error occurred during the bag validation process. Please try again, or ask a system administrator to investigate.",
			)
			return result, nil
		}
		if bagValidateResult.Error != "" {
			result.ValidationError(
				temporalsdk_workflow.Now(ctx),
				task,
				"Bag validation has failed.",
				// TODO: Add BagIt tool and version info.
				// "An attempt to validate the bag using [tool] - version [version] has failed:",
				bagValidateResult.Error,
				"Please ensure the bag is well-formed before reattempting ingest.",
				"Your SIP has been moved to the failed-sips directory.",
			)
		} else {
			// TODO: Add BagIt tool and version info.
			task.Succeed(temporalsdk_workflow.Now(ctx), "Bag successfully validated")
		}

		task = result.NewTask(temporalsdk_workflow.Now(ctx), "Unbag SIP")
		var unbagResult activities.UnbagResult
		e = temporalsdk_workflow.ExecuteActivity(
			withFilesystemActivityOpts(ctx),
			activities.UnbagName,
			&activities.UnbagParams{Path: localPath},
		).Get(ctx, &unbagResult)
		if e != nil {
			logger.Error("System error", "message", e.Error())
			result.SystemError(
				temporalsdk_workflow.Now(ctx),
				task,
				"SIP unbagging has failed.",
				"An error occurred during the SIP unbagging process. Please try again, or ask a system administrator to investigate.",
			)
			return result, nil
		}

		localPath = unbagResult.Path
		result.RelativePath, e = filepath.Rel(w.cfg.SharedPath, localPath)
		if e != nil {
			logger.Error("System error", "message", e.Error())
			result.SystemError(
				temporalsdk_workflow.Now(ctx),
				task,
				"SIP unbagging has failed.",
				"An error occurred during the SIP unbagging process. Please try again, or ask a system administrator to investigate.",
			)
			return result, nil
		}
		task.Succeed(temporalsdk_workflow.Now(ctx), "SIP unbagged")
	}

	// Identify SIP.
	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Identify SIP structure")
	var identifySIP activities.IdentifySIPResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.IdentifySIPName,
		&activities.IdentifySIPParams{Path: localPath},
	).Get(ctx, &identifySIP)
	if e != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP identification has failed.",
			"Enduro could not identify the package type. Please ensure that your SIP matches one of the supported package structures.",
		)
		return result, nil
	}

	sip := identifySIP.SIP
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP structure identified: %s", sip.Type)

	// Validate structure.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP structure")
	var validateStructure activities.ValidateStructureResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateStructureName,
		&activities.ValidateStructureParams{SIP: sip},
	).Get(ctx, &validateStructure)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP structure validation has failed.",
			"An error occurred during the structure validation process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	if validateStructure.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP structure validation has failed.",
			ul(validateStructure.Failures),
			fmt.Sprintf("Please review the SIP and ensure that its structure matches the %s specifications.", sip.Type),
		)
	} else {
		task.Succeed(temporalsdk_workflow.Now(ctx), "SIP structure matches validation criteria")
	}

	// Validate SIP name.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP name")
	var validateSIPName activities.ValidateSIPNameResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateSIPNameName,
		&activities.ValidateSIPNameParams{SIP: sip},
	).Get(ctx, &validateSIPName)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP name validation has failed.",
			"An error occurred during the SIP name validation process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	if validateSIPName.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP name validation has failed.",
			fmt.Sprintf(
				"The name used for the package does not match the expected convention for the %q type.",
				sip.Type,
			),
			ul(validateSIPName.Failures),
			"Please review the naming conventions specified for this type of SIP.",
		)
	} else {
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"SIP name matches expected naming convention for the identified structure type",
		)
	}

	// Verify that package contents match the manifest.
	manifestTask := result.NewTask(temporalsdk_workflow.Now(ctx), "Verify SIP manifest")
	checksumTask := result.NewTask(temporalsdk_workflow.Now(ctx), "Verify SIP checksums")
	var verifyManifest activities.VerifyManifestResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.VerifyManifestName,
		&activities.VerifyManifestParams{SIP: sip},
	).Get(ctx, &verifyManifest)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			manifestTask,
			"SIP manifest verification has failed.",
			"An error occurred during the manifest verification process. Please try again, or ask a system administrator to investigate.",
		)
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			checksumTask,
			"SIP checksum verification has failed.",
			"An error occurred during the checksum verification process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}

	if len(verifyManifest.ManifestFailures) > 0 || len(verifyManifest.MissingFiles) > 0 ||
		len(verifyManifest.UnexpectedFiles) > 0 {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			manifestTask,
			fmt.Sprintf(
				"%q manifest could not be verified against the contents of the SIP.",
				filepath.Base(sip.ManifestPath),
			),
			ul(
				slices.Concat(
					verifyManifest.ManifestFailures,
					verifyManifest.MissingFiles,
					verifyManifest.UnexpectedFiles,
				),
			),
			"Please review the SIP and ensure that its contents match those listed in the metadata manifest.",
		)
	} else {
		manifestTask.Succeed(temporalsdk_workflow.Now(ctx), "SIP contents match manifest")
	}

	if len(verifyManifest.ChecksumFailures) > 0 {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			checksumTask,
			"SIP checksums do not match file contents.",
			ul(verifyManifest.ChecksumFailures),
			"Please review the SIP and ensure that the metadata checksums match those of the files.",
		)
	} else {
		checksumTask.Succeed(temporalsdk_workflow.Now(ctx), "SIP checksums match file contents")
	}

	// Check for disallowed file formats (SIP types only).
	if sip.IsSIP() {
		task = result.NewTask(temporalsdk_workflow.Now(ctx), "Check for disallowed file formats")
		var ffvalidateResult ffvalidate.Result
		e = temporalsdk_workflow.ExecuteActivity(
			withFilesystemActivityOpts(ctx),
			ffvalidate.Name,
			&ffvalidate.Params{Path: sip.ContentPath},
		).Get(ctx, &ffvalidateResult)
		if e != nil {
			logger.Error("System error", "message", e.Error())
			result.SystemError(
				temporalsdk_workflow.Now(ctx),
				task,
				"file format check has failed.",
				"An error occurred when checking for disallowed file formats. Please try again, or ask a system administrator to investigate.",
			)
			return result, nil
		}

		if ffvalidateResult.Failures != nil {
			result.ValidationError(
				temporalsdk_workflow.Now(ctx),
				task,
				"file format check has failed.",
				"One or more file formats are not allowed:",
				ul(ffvalidateResult.Failures),
				"Please review the SIP and remove or replace all disallowed file formats.",
			)
		} else {
			task.Succeed(temporalsdk_workflow.Now(ctx), "No disallowed file formats found")
		}
	}

	// Validate SIP file formats against the format specifications.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP file formats")
	var validateFilesResult activities.ValidateFilesResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateFilesName,
		&activities.ValidateFilesParams{SIP: sip},
	).Get(ctx, &validateFilesResult)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"file format validation has failed.",
			"An error occurred during the file format validation process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}

	if validateFilesResult.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"file format validation has failed.",
			// TODO: Add tool name and version info.
			ul(validateFilesResult.Failures),
			"Please ensure all files are well-formed.",
		)
	} else {
		task.Succeed(temporalsdk_workflow.Now(ctx), "No invalid files found")
	}

	// Validate metadata.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP metadata")
	var validateMetadata xmlvalidate.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		xmlvalidate.Name,
		&xmlvalidate.Params{
			XMLPath: sip.ManifestPath,
			XSDPath: sip.XSDPath,
		},
	).Get(ctx, &validateMetadata)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"metadata validation has failed.",
			fmt.Sprintf(
				"An error has occurred while attempting to validate the %q file. Please try again, or ask a system administrator to investigate.",
				filepath.Base(sip.ManifestPath),
			),
		)
		return result, nil
	}

	if validateMetadata.Failures != nil {
		for idx, f := range validateMetadata.Failures {
			validateMetadata.Failures[idx] = strings.ReplaceAll(f, sip.Path+"/", "")
		}
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"metadata validation has failed.",
			ul(validateMetadata.Failures),
			"Please ensure all metadata files are present and well-formed.",
		)
	} else {
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"Metadata validation successful on the following file(s):\n\n%s",
			ul([]string{filepath.Base(sip.ManifestPath)}),
		)
	}

	// Validate logical metadata (AIP types only).
	if sip.IsAIP() {
		task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate logical metadata")
		var validateLMD activities.ValidatePREMISResult
		e = temporalsdk_workflow.ExecuteActivity(
			withFilesystemActivityOpts(ctx),
			activities.ValidatePREMISName,
			activities.ValidatePREMISParams{Path: sip.LogicalMDPath},
		).Get(ctx, &validateLMD)
		if e != nil {
			logger.Error("System error", "message", e.Error())
			result.SystemError(
				temporalsdk_workflow.Now(ctx),
				task,
				"logical metadata validation has failed.",
				fmt.Sprintf(
					"An error has occurred while attempting to validate the %q file. Please try again, or ask a system administrator to investigate.",
					filepath.Base(sip.LogicalMDPath),
				),
			)
			return result, nil
		}
		if validateLMD.Failures != nil {
			result.ValidationError(
				temporalsdk_workflow.Now(ctx),
				task,
				"logical metadata validation has failed.",
				ul(validateLMD.Failures),
				"Please ensure all metadata files are present and well-formed.",
			)
		} else {
			task.Succeed(temporalsdk_workflow.Now(ctx), "Logical metadata validation successful")
		}
	}

	// Stop here if the SIP content isn't valid.
	if result.Outcome == childwf.OutcomeContentError {
		return result, nil
	}

	// Create APIS import task.
	if w.apisEnabled {
		if ok := w.createAPISImportTask(ctx, result, sip, apisUsername(params.User)); !ok {
			return result, nil
		}
	}

	// Write PREMIS XML.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Create premis.xml")
	if e = writePREMISFile(ctx, sip); e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"premis.xml creation has failed",
			"An error has occurred while attempting to create the premis.xml file and store it in the metadata directory. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "Created a premis.xml file and stored it in the metadata directory")

	// Re-structure SIP.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Restructure SIP")
	var transformSIP activities.TransformSIPResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.TransformSIPName,
		&activities.TransformSIPParams{SIP: sip},
	).Get(ctx, &transformSIP)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"restructuring has failed",
			"An error has occurred while attempting to restructure the SIP for preservation processing. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP has been restructured for preservation processing")

	// Write the identifiers.json file.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Create identifier.json")
	var writeIDFile activities.WriteIdentifierFileResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.WriteIdentifierFileName,
		&activities.WriteIdentifierFileParams{PIP: transformSIP.PIP},
	).Get(ctx, &writeIDFile)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"identifier.json creation has failed.",
			"An error has occurred while attempting to create the identifier.json file and store it in the metadata directory. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(
		temporalsdk_workflow.Now(ctx),
		"Created an identifier.json file and stored it in the metadata directory",
	)

	// Bag the SIP for Enduro processing.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Bag SIP")
	var createBag bagcreate.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		bagcreate.Name,
		&bagcreate.Params{SourcePath: localPath},
	).Get(ctx, &createBag)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP bagging has failed.",
			"An error has occurred while attempting to bag the SIP. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP has been bagged")

	return result, nil
}

// createAPISImportTask submits metadata to APIS, waits for analysis, and records
// the resulting custom metadata for the poststorage workflow. It returns false
// when processing should stop after recording the failure in the workflow result.
func (w *Preprocessing) createAPISImportTask(
	ctx temporalsdk_workflow.Context,
	result *childwf.PreprocessingResult,
	sip sip.SIP,
	username string,
) bool {
	logger := temporalsdk_workflow.GetLogger(ctx)

	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Submit metadata to APIS")
	var createAPISImportTask apis.CreateImportTaskResult
	err := temporalsdk_workflow.ExecuteActivity(
		temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: time.Minute * 5,
			RetryPolicy: &temporalsdk_temporal.RetryPolicy{
				InitialInterval:    time.Second * 5,
				BackoffCoefficient: 2,
				MaximumAttempts:    3,
			},
		}),
		apis.CreateImportTaskActivityName,
		&apis.CreateImportTaskParams{
			SIP:      sip,
			Username: username,
		},
	).Get(ctx, &createAPISImportTask)
	if err != nil {
		if isCreateAPISImportTaskDefaultValuesError(err) {
			result.Outcome = childwf.OutcomeContentError
			task.Complete(
				temporalsdk_workflow.Now(ctx),
				childwf.TaskOutcomeValidationFailure,
				APISDefaultValuesFailureMessage,
			)
			return false
		}
		logger.Error("System error", "message", err.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"failed to submit metadata to APIS.",
			"An error occurred while creating the APIS import task. Please try again, or ask a system administrator to investigate.",
		)
		return false
	}
	metadata := apis.CustomMetadata{ImportTaskID: createAPISImportTask.TaskID}
	task.Succeed(
		temporalsdk_workflow.Now(ctx),
		"Submitted metadata to APIS with import task ID %q",
		metadata.ImportTaskID,
	)

	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Wait for APIS analysis")
	var pollAPISImportTaskStatus apis.PollImportTaskStatusResult
	err = temporalsdk_workflow.ExecuteActivity(
		temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
			StartToCloseTimeout: time.Hour * 24,
			HeartbeatTimeout:    time.Minute,
			RetryPolicy: &temporalsdk_temporal.RetryPolicy{
				InitialInterval:    time.Second * 5,
				BackoffCoefficient: 2,
				MaximumAttempts:    3,
			},
		}),
		apis.PollImportTaskStatusActivityName,
		&apis.PollImportTaskStatusParams{TaskID: metadata.ImportTaskID},
	).Get(ctx, &pollAPISImportTaskStatus)
	if err != nil {
		logger.Error("System error", "message", err.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"failed to get an APIS analysis result.",
			"An error occurred while checking the APIS import task status. Please try again, or ask a system administrator to investigate.",
		)
		return false
	}

	switch pollAPISImportTaskStatus.AnalysisResult {
	case apisgen.AnalysisResultAlleNeu, apisgen.AnalysisResultAlleGleich:
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"APIS analysis completed for import task ID %q with result %q",
			metadata.ImportTaskID,
			pollAPISImportTaskStatus.AnalysisResult,
		)
	case apisgen.AnalysisResultKonflikte:
		decision, ok := w.waitForAPISDecision(ctx, result, task, metadata.ImportTaskID)
		metadata.Decision = decision
		if !ok {
			return false
		}
	case apisgen.AnalysisResultFehler:
		logger.Error(
			"System error",
			"message",
			fmt.Sprintf("APIS analysis failed for task %q", metadata.ImportTaskID),
		)
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"submission to APIS has failed.",
			"APIS reported an analysis error while processing the submitted metadata. Please try again, or ask a system administrator to investigate.",
		)
		return false
	default:
		logger.Error("System error", "message", fmt.Errorf(
			"unexpected APIS analysis result %q for task %q",
			pollAPISImportTaskStatus.AnalysisResult,
			metadata.ImportTaskID,
		))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"submission to APIS has failed.",
			"APIS returned an unexpected analysis result. Please ask a system administrator to investigate.",
		)
		return false
	}

	data, err := metadata.Marshal()
	if err != nil {
		logger.Error("System error", "message", err.Error())
		task = result.NewTask(temporalsdk_workflow.Now(ctx), "Record APIS metadata")
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"APIS metadata recording has failed.",
			"An error occurred while recording APIS metadata for later workflow steps. Please try again, or ask a system administrator to investigate.",
		)
		return false
	}
	result.CustomMetadata = childwf.CustomMetadata{apis.CustomMetadataKey: data}

	return true
}

func isCreateAPISImportTaskDefaultValuesError(err error) bool {
	var applicationErr *temporalsdk_temporal.ApplicationError
	return errors.As(err, &applicationErr) && applicationErr.Type() == apis.CreateImportTaskDefaultValuesErrorType
}

// waitForAPISDecision asks the parent workflow for a decision. It returns the
// selected decision and false when processing should stop after recording the
// failure in the workflow result.
func (w *Preprocessing) waitForAPISDecision(
	ctx temporalsdk_workflow.Context,
	result *childwf.PreprocessingResult,
	task *childwf.Task,
	taskID string,
) (string, bool) {
	logger := temporalsdk_workflow.GetLogger(ctx)
	info := temporalsdk_workflow.GetInfo(ctx)
	if info.ParentWorkflowExecution == nil {
		logger.Error("System error", "message", fmt.Errorf(
			"missing parent workflow execution while waiting for a decision on APIS import task %q",
			taskID,
		))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"submission to APIS has failed.",
			"An error occurred requesting a human decision on how to continue processing. Please try again, or ask a system administrator to investigate.",
		)
		return "", false
	}

	err := temporalsdk_workflow.SignalExternalWorkflow(
		ctx,
		info.ParentWorkflowExecution.ID,
		info.ParentWorkflowExecution.RunID,
		childwf.DecisionRequestSignalName,
		childwf.DecisionRequest{
			Message: fmt.Sprintf(
				"APIS detected metadata conflicts for import task ID %q. Review the APIS task and choose how ingest should continue.",
				taskID,
			),
			Options: []string{
				apis.DecisionOptionCancelIngest,
				apis.DecisionOptionContinueOverwrite,
				apis.DecisionOptionContinueAppend,
			},
		},
	).Get(ctx, nil)
	if err != nil {
		logger.Error("System error", "message", fmt.Errorf(
			"signal parent workflow for decision on APIS import task %q: %w", taskID, err,
		))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"submission to APIS has failed.",
			"Could not notify the parent workflow that human review is required. Please try again, or ask a system administrator to investigate.",
		)
		return "", false
	}

	var decision childwf.DecisionResponse
	temporalsdk_workflow.GetSignalChannel(ctx, childwf.DecisionResponseSignalName).Receive(ctx, &decision)

	switch decision.Option {
	case apis.DecisionOptionContinueOverwrite, apis.DecisionOptionContinueAppend:
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"APIS detected metadata conflicts for import task ID %q but ingest was continued with user decision %q.",
			taskID,
			decision.Option,
		)
		return decision.Option, true
	case apis.DecisionOptionCancelIngest:
		// TODO: Add and use canceled as workflow outcome.
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"ingest was canceled after APIS metadata conflict review.",
			fmt.Sprintf(
				"APIS detected metadata conflicts for import task ID %q and ingest was canceled by user decision.",
				taskID,
			),
		)
		return decision.Option, false
	default:
		logger.Error("System error", "message", fmt.Errorf(
			"unsupported decision %q for APIS import task %q", decision.Option, taskID,
		))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"submission to APIS has failed.",
			fmt.Sprintf(
				"Received unsupported user decision %q while resolving APIS metadata conflicts. Please ask a system administrator to investigate.",
				decision.Option,
			),
		)
		return decision.Option, false
	}
}

func (w *Preprocessing) extractSIP(
	ctx temporalsdk_workflow.Context,
	result *childwf.PreprocessingResult,
	path string,
	sipName string,
) string {
	logger := temporalsdk_workflow.GetLogger(ctx)

	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Extract SIP")
	var archiveExtract archiveextract.Result
	e := temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		archiveextract.Name,
		&archiveextract.Params{SourcePath: path},
	).Get(ctx, &archiveExtract)
	if e != nil {
		logger.Error("System error", "message", fmt.Errorf("extract SIP: %w", e))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP extraction has failed.",
			fmt.Sprintf(
				`%q could not be successfully extracted. Please try again, or ask a system administrator to investigate.`,
				filepath.Base(path),
			),
		)
		return ""
	}

	// Verify that the extraction directory has the same name as the uploaded
	// archive minus the file extension (e.g. "example.zip" -> "example").
	if filepath.Base(archiveExtract.ExtractPath) != fsutil.BaseNoExt(sipName) {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP extraction has failed.",
			fmt.Sprintf(
				"The extracted SIP is missing the top-level %q folder.",
				fsutil.BaseNoExt(sipName),
			),
			"Please ensure that the SIP is well-formed and try again.",
		)
		return archiveExtract.ExtractPath
	}

	result.RelativePath, e = filepath.Rel(w.cfg.SharedPath, archiveExtract.ExtractPath)
	if e != nil {
		logger.Error("System error", "message", fmt.Errorf("extract SIP: set relative path: %w", e))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP extraction has failed.",
			fmt.Sprintf(
				`%s could not be successfully extracted. Please try again, or ask a system administrator to investigate.`,
				filepath.Base(path),
			),
		)
		return ""
	}

	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP extracted")

	return archiveExtract.ExtractPath
}

func writePREMISFile(ctx temporalsdk_workflow.Context, sip sip.SIP) error {
	var e error
	path := filepath.Join(sip.Path, "metadata", "premis.xml")

	// Add PREMIS objects.
	var addPREMISObjects activities.AddPREMISObjectsResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISObjectsName,
		&activities.AddPREMISObjectsParams{
			SIP:            sip,
			PREMISFilePath: path,
		},
	).Get(ctx, &addPREMISObjects)
	if e != nil {
		return e
	}

	// Add PREMIS event noting validate structure result.
	validateStructureOutcomeDetail := fmt.Sprintf(
		"SIP structure identified: %s. SIP structure matches validation criteria.",
		sip.Type.String(),
	)

	var addPREMISEvent activities.AddPREMISEventResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP structure\"",
			OutcomeDetail:  validateStructureOutcomeDetail,
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add PREMIS event noting validate SIP name result.
	validateSIPNameOutcomeDetail := fmt.Sprintf(
		"SIP name %q matches validation criteria.",
		sip.Name(),
	)

	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP name\"",
			OutcomeDetail:  validateSIPNameOutcomeDetail,
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	if sip.IsSIP() {
		// Add PREMIS events for the disallowed file format check (SIP types
		// only).
		e = temporalsdk_workflow.ExecuteActivity(
			withFilesystemActivityOpts(ctx),
			activities.AddPREMISEventName,
			&activities.AddPREMISEventParams{
				PREMISFilePath: path,
				Agent:          premis.AgentDefault(),
				Type:           "validation",
				Detail:         "name=\"Check for disallowed file formats\"",
				OutcomeDetail:  "Format allowed",
				Failures:       nil,
			},
		).Get(ctx, &addPREMISEvent)
		if e != nil {
			return e
		}
	}

	// Add PREMIS events for file format validation.
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISValidationEventName,
		&activities.AddPREMISValidationEventParams{
			SIP:            sip,
			PREMISFilePath: path,
			Summary: premis.EventSummary{
				Type:          "validation",
				Detail:        "name=\"Validate SIP file formats\"",
				Outcome:       "valid",
				OutcomeDetail: "File format complies with specification",
			},
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add PREMIS events for metadata validation.
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP metadata\"",
			OutcomeDetail:  "Metadata validation successful",
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add Enduro PREMIS agent.
	var addPREMISEnduroAgent activities.AddPREMISAgentResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISAgentName,
		&activities.AddPREMISAgentParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
		},
	).Get(ctx, &addPREMISEnduroAgent)
	if e != nil {
		return e
	}

	return nil
}

// ul formats a list of strings as an unordered, Markdown-style list.
func ul(items []string) string {
	if len(items) == 0 {
		return ""
	}

	var s strings.Builder
	for _, i := range items {
		fmt.Fprintf(&s, "- %s\n", i)
	}

	return strings.TrimSuffix(s.String(), "\n")
}
