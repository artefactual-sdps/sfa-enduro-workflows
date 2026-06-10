package workflows_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/workflows"
)

type TestSuite struct {
	suite.Suite
	temporalsdk_testsuite.WorkflowTestSuite

	env      *temporalsdk_testsuite.TestWorkflowEnvironment
	workflow *workflows.Poststorage
}

const (
	poststorageAIPUUIDString = "9390594f-84c2-457d-bd6a-618f21f7c954"
	poststorageAIPName       = "test-" + poststorageAIPUUIDString
	poststorageAIPPath       = "9390/594f/84c2/457d/bd6a/618f/21f7/c954/" + poststorageAIPName + ".zip"
	poststorageImportTaskID  = "task-000001"
	poststorageImportRunID   = "run-000001"
	poststorageMETSName      = "METS." + poststorageAIPUUIDString + ".xml"
	poststorageMETSRelPath   = poststorageAIPName + "/data/" + poststorageMETSName
	poststorageWorkflowUser  = "sfa-enduro"
)

var (
	poststorageAIPUUID    = uuid.MustParse(poststorageAIPUUIDString)
	poststorageWorkingDir = filepath.Join(os.TempDir(), "sfa-enduro-poststorage-test")
	poststorageMETSPath   = filepath.Join(poststorageWorkingDir, poststorageAIPUUIDString, poststorageMETSName)
	poststorageSessionCtx = mock.AnythingOfType("*context.timerCtx")
	poststorageTestTime   = time.Date(2024, 6, 6, 15, 8, 39, 0, time.UTC)

	poststorageAppendMetadata = childwf.CustomMetadata{
		apis.CustomMetadataKey: json.RawMessage(fmt.Sprintf(
			`{"importTaskId":%q,"decision":%q}`,
			poststorageImportTaskID,
			apis.DecisionOptionContinueAppend,
		)),
	}
	poststorageOverwriteMetadata = childwf.CustomMetadata{
		apis.CustomMetadataKey: json.RawMessage(fmt.Sprintf(
			`{"importTaskId":%q,"decision":%q}`,
			poststorageImportTaskID,
			apis.DecisionOptionContinueOverwrite,
		)),
	}
)

func (s *TestSuite) setup(cfg *config.PoststorageConfig, apisEnabled bool) {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetStartTime(poststorageTestTime)
	s.env.SetWorkerOptions(temporalsdk_worker.Options{EnableSessionWorker: true})
	cfg.WorkingDir = poststorageWorkingDir

	s.env.RegisterActivityWithOptions(
		amss.NewGetAIPPathActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.GetAIPPathActivityName},
	)
	s.env.RegisterActivityWithOptions(
		amss.NewFetchActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.FetchActivityName},
	)
	s.env.RegisterActivityWithOptions(
		removepaths.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: removepaths.Name},
	)
	s.env.RegisterActivityWithOptions(
		apis.NewCreateImportRunActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: apis.CreateImportRunActivityName},
	)
	s.env.RegisterActivityWithOptions(
		apis.NewPollImportRunStatusActivity(nil, 0).Execute,
		temporalsdk_activity.RegisterOptions{Name: apis.PollImportRunStatusActivityName},
	)

	s.workflow = workflows.NewPoststorage(*cfg, apisEnabled)
}

func TestPoststorage(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) assertWorkflow(expected *childwf.PostStorageResult, errorContains string) {
	s.T().Helper()

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())

	if errorContains != "" {
		s.ErrorContains(s.env.GetWorkflowError(), errorContains)
		return
	}

	var result childwf.PostStorageResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal(expected, &result)
}

func (s *TestSuite) TestAPISDisabled() {
	s.setup(&config.PoststorageConfig{}, false)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{
			AIPUUID: poststorageAIPUUIDString,
		},
	)

	s.assertWorkflow(&childwf.PostStorageResult{}, "")
}

func (s *TestSuite) TestInvalidAIPUUID() {
	s.setup(&config.PoststorageConfig{}, true)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{
			AIPUUID: "not-a-uuid",
		},
	)

	s.assertWorkflow(nil, "parse AIP UUID")
}

func (s *TestSuite) TestAPISMetadataMissing() {
	s.setup(&config.PoststorageConfig{}, true)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{
			AIPUUID: poststorageAIPUUIDString,
		},
	)

	s.assertWorkflow(nil, "APIS custom metadata is required when APIS integration is enabled")
}

func (s *TestSuite) TestSuccess() {
	s.setup(&config.PoststorageConfig{}, true)
	s.mockActivitiesSuccess()
	s.env.OnActivity(
		apis.CreateImportRunActivityName,
		poststorageSessionCtx,
		&apis.CreateImportRunParams{
			TaskID:          poststorageImportTaskID,
			METSPath:        poststorageMETSPath,
			ImportBehaviour: apisgen.ImportBehaviourTypeOverwriteAndAppend,
			Username:        poststorageWorkflowUser,
		},
	).Return(
		&apis.CreateImportRunResult{RunID: poststorageImportRunID}, nil,
	)
	s.env.OnActivity(
		apis.PollImportRunStatusActivityName,
		poststorageSessionCtx,
		&apis.PollImportRunStatusParams{TaskID: poststorageImportTaskID},
	).Return(
		&apis.PollImportRunStatusResult{ImportResult: apisgen.ImportResultErfolgreich}, nil,
	)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{
			AIPUUID:        poststorageAIPUUIDString,
			CustomMetadata: poststorageOverwriteMetadata,
		},
	)

	s.assertWorkflow(
		&childwf.PostStorageResult{
			Outcome: childwf.OutcomeSuccess,
			Tasks: []*childwf.Task{
				{
					Name:        "Download AIP METS",
					Message:     "AIP METS downloaded",
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime,
				},
				{
					Name: "Submit AIP METS to APIS",
					Message: fmt.Sprintf(
						`Submitted AIP METS to APIS with import task ID %q, import run ID %q, and import behaviour %q`,
						poststorageImportTaskID,
						poststorageImportRunID,
						apisgen.ImportBehaviourTypeOverwriteAndAppend,
					),
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime,
				},
				{
					Name: "Wait for APIS import",
					Message: fmt.Sprintf(
						`APIS import completed for import task ID %q with result %q`,
						poststorageImportTaskID,
						apisgen.ImportResultErfolgreich,
					),
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime,
				},
			},
		},
		"",
	)
}

func (s *TestSuite) TestDownloadFailure() {
	s.setup(&config.PoststorageConfig{}, true)
	s.env.OnActivity(
		amss.GetAIPPathActivityName,
		poststorageSessionCtx,
		&amss.GetAIPPathActivityParams{AIPUUID: poststorageAIPUUID},
	).Return(
		&amss.GetAIPPathActivityResult{
			Path: poststorageAIPPath,
		}, nil,
	)
	s.env.OnActivity(
		amss.FetchActivityName,
		poststorageSessionCtx,
		&amss.FetchActivityParams{
			AIPUUID:      poststorageAIPUUID,
			RelativePath: poststorageMETSRelPath,
			Destination:  poststorageMETSPath,
		},
	).Return(
		nil, errors.New("download failed"),
	)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{
			AIPUUID:        poststorageAIPUUIDString,
			CustomMetadata: poststorageAppendMetadata,
		},
	)

	s.assertWorkflow(
		&childwf.PostStorageResult{
			Outcome: childwf.OutcomeSystemError,
			Tasks: []*childwf.Task{
				{
					Name: "Download AIP METS",
					Message: `System error: AIP METS download has failed.

The AIP is stored, but the poststorage workflow failed while downloading the AIP METS file. Please try again, or ask a system administrator to investigate.`,
					Outcome:     childwf.TaskOutcomeSystemFailure,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime.Add(4 * time.Second),
				},
			},
		},
		"",
	)
}

func (s *TestSuite) TestAPISImportFailure() {
	s.setup(&config.PoststorageConfig{}, true)
	s.mockActivitiesSuccess()
	s.env.OnActivity(
		apis.CreateImportRunActivityName,
		poststorageSessionCtx,
		&apis.CreateImportRunParams{
			TaskID:          poststorageImportTaskID,
			METSPath:        poststorageMETSPath,
			ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			Username:        poststorageWorkflowUser,
		},
	).Return(
		&apis.CreateImportRunResult{RunID: poststorageImportRunID}, nil,
	)
	s.env.OnActivity(
		apis.PollImportRunStatusActivityName,
		poststorageSessionCtx,
		&apis.PollImportRunStatusParams{TaskID: poststorageImportTaskID},
	).Return(
		&apis.PollImportRunStatusResult{ImportResult: apisgen.ImportResultFehler}, nil,
	)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{
			AIPUUID:        poststorageAIPUUIDString,
			CustomMetadata: poststorageAppendMetadata,
		},
	)

	s.assertWorkflow(
		&childwf.PostStorageResult{
			Outcome: childwf.OutcomeSystemError,
			Tasks: []*childwf.Task{
				{
					Name:        "Download AIP METS",
					Message:     "AIP METS downloaded",
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime,
				},
				{
					Name: "Submit AIP METS to APIS",
					Message: fmt.Sprintf(
						`Submitted AIP METS to APIS with import task ID %q, import run ID %q, and import behaviour %q`,
						poststorageImportTaskID,
						poststorageImportRunID,
						apisgen.ImportBehaviourTypeAppendOnly,
					),
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime,
				},
				{
					Name: "Wait for APIS import",
					Message: `System error: APIS import has failed.

The AIP is stored, but APIS reported an error while importing the AIP METS file into AIS. Please try again, or ask a system administrator to investigate.`,
					Outcome:     childwf.TaskOutcomeSystemFailure,
					StartedAt:   poststorageTestTime,
					CompletedAt: poststorageTestTime,
				},
			},
		},
		"",
	)
}

func (s *TestSuite) mockActivitiesSuccess() {
	s.env.OnActivity(
		amss.GetAIPPathActivityName,
		poststorageSessionCtx,
		&amss.GetAIPPathActivityParams{AIPUUID: poststorageAIPUUID},
	).Return(
		&amss.GetAIPPathActivityResult{
			Path: poststorageAIPPath,
		}, nil,
	)
	s.env.OnActivity(
		amss.FetchActivityName,
		poststorageSessionCtx,
		&amss.FetchActivityParams{
			AIPUUID:      poststorageAIPUUID,
			RelativePath: poststorageMETSRelPath,
			Destination:  poststorageMETSPath,
		},
	).Return(
		&amss.FetchActivityResult{}, nil,
	)
}
