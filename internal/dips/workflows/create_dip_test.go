package workflows_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_converter "go.temporal.io/sdk/converter"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/workflows"
)

type CreateDIPTestSuite struct {
	suite.Suite
	temporalsdk_testsuite.WorkflowTestSuite

	env        *temporalsdk_testsuite.TestWorkflowEnvironment
	workflow   *workflows.CreateDIP
	workingDir string
	dip        datatypes.DIP
}

var createDIPTestTime = time.Date(2024, 6, 6, 15, 8, 39, 0, time.UTC)

func (s *CreateDIPTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetStartTime(createDIPTestTime)
	s.env.SetWorkerOptions(temporalsdk_worker.Options{EnableSessionWorker: true})
	s.env.RegisterActivityWithOptions(
		activities.NewUpdateDIP(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.UpdateDIPName},
	)
	s.env.RegisterActivityWithOptions(
		actapro.NewGetDocumentActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: actapro.GetDocumentActivityName},
	)
	s.env.RegisterActivityWithOptions(
		actapro.NewCreateExportActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: actapro.CreateExportActivityName},
	)
	s.env.RegisterActivityWithOptions(
		actapro.NewPollExportStatusActivity(nil, actapro.DefaultPollInterval).Execute,
		temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
	)
	s.env.RegisterActivityWithOptions(
		actapro.NewDownloadExportActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: actapro.DownloadExportActivityName},
	)
	s.env.RegisterActivityWithOptions(
		removepaths.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: removepaths.Name},
	)
	s.workingDir = s.T().TempDir()
	s.workflow = workflows.NewCreateDIP(s.workingDir)
	s.dip = datatypes.DIP{
		DBID:      1,
		UUID:      uuid.MustParse("9390594f-84c2-457d-bd6a-618f21f7c954"),
		DocKey:    "CH-000001",
		Status:    enums.DIPStatusQueued,
		CreatedAt: createDIPTestTime.Add(-time.Second),
	}
}

func TestCreateDIP(t *testing.T) {
	suite.Run(t, new(CreateDIPTestSuite))
}

func (s *CreateDIPTestSuite) TestSuccess() {
	s.testSuccess(nil, 0)
}

func (s *CreateDIPTestSuite) TestCleanupFailureDoesNotFailWorkflow() {
	s.testSuccess(errors.New("remove paths failed"), 0)
}

func (s *CreateDIPTestSuite) TestCleanupCompletesAfterCancellation() {
	s.env.RegisterDelayedCallback(s.env.CancelWorkflow, time.Second)
	s.testSuccess(nil, 2*time.Second)
	s.Equal(createDIPTestTime.Add(2*time.Second), s.env.Now().UTC())
}

func (s *CreateDIPTestSuite) testSuccess(cleanupErr error, cleanupDelay time.Duration) {
	s.T().Helper()

	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	initialUpdate := s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()
	getDocument := s.env.OnActivity(
		actapro.GetDocumentActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
	).Return(&actapro.GetDocumentResult{
		AIPUUIDs: []string{"28c1a3e2-abd3-4b9b-9214-ae851c87b1a6", "1231e569-a94e-4ac1-873e-65e1f524b1c8"},
	}, nil).Once().NotBefore(initialUpdate)
	createExport := s.env.OnActivity(
		actapro.CreateExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.CreateExportParams{DocKey: s.dip.DocKey},
	).Return(&actapro.CreateExportResult{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"}, nil).
		Once().NotBefore(getDocument)
	pollExport := s.env.OnActivity(
		actapro.PollExportStatusActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.PollExportStatusParams{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"},
	).Return(&actapro.PollExportStatusResult{Status: "COMPLETED"}, nil).Once().NotBefore(createExport)
	downloadExport := s.env.OnActivity(
		actapro.DownloadExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.DownloadExportParams{
			ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
			MetadataPath: filepath.Join(s.workingDir, s.dip.UUID.String(), "metadata.xml"),
		},
	).Return(&actapro.DownloadExportResult{}, nil).Once().NotBefore(pollExport)
	cleanup := s.env.OnActivity(
		removepaths.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&removepaths.Params{Paths: []string{filepath.Join(s.workingDir, s.dip.UUID.String())}},
	).Return(&removepaths.Result{}, cleanupErr).After(cleanupDelay).Once().NotBefore(downloadExport)

	wDIP.ObjectKey = "DIP_9390594f-84c2-457d-bd6a-618f21f7c954.zip"
	wDIP.Status = enums.DIPStatusDone
	wDIP.CompletedAt = createDIPTestTime.Add(cleanupDelay)
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once().NotBefore(cleanup)

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	s.NoError(s.env.GetWorkflowError())

	var result workflows.CreateDIPResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(workflows.CreateDIPResult{DIP: wDIP}, result)
}

func (s *CreateDIPTestSuite) TestSessionRecoveryClearsDownloadError() {
	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()
	s.env.OnActivity(
		actapro.GetDocumentActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
	).Return(&actapro.GetDocumentResult{}, nil).Once()
	s.env.OnActivity(
		actapro.CreateExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.CreateExportParams{DocKey: s.dip.DocKey},
	).Return(&actapro.CreateExportResult{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"}, nil).Once()
	s.env.OnActivity(
		actapro.PollExportStatusActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.PollExportStatusParams{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"},
	).Return(&actapro.PollExportStatusResult{Status: "COMPLETED"}, nil).Once()

	downloadParams := &actapro.DownloadExportParams{
		ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
		MetadataPath: filepath.Join(s.workingDir, s.dip.UUID.String(), "metadata.xml"),
	}
	// A canceled download with an active workflow triggers a new session.
	failedDownload := s.env.OnActivity(
		actapro.DownloadExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		downloadParams,
	).Return(nil, temporalsdk_temporal.NewCanceledError()).Once()
	s.env.OnActivity(
		actapro.DownloadExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		downloadParams,
	).Return(&actapro.DownloadExportResult{}, nil).Once().NotBefore(failedDownload)
	cleanup := s.env.OnActivity(
		removepaths.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&removepaths.Params{Paths: []string{filepath.Join(s.workingDir, s.dip.UUID.String())}},
	).Return(&removepaths.Result{}, nil).Twice()

	wDIP.ObjectKey = "DIP_9390594f-84c2-457d-bd6a-618f21f7c954.zip"
	wDIP.Status = enums.DIPStatusDone
	wDIP.CompletedAt = createDIPTestTime.Add(10 * time.Second)
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once().NotBefore(cleanup)

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	s.Require().NoError(s.env.GetWorkflowError())
	var result workflows.CreateDIPResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(workflows.CreateDIPResult{DIP: wDIP}, result)
}

func (s *CreateDIPTestSuite) TestExportFailed() {
	for _, tt := range []struct {
		name string
		logs string
		want string
	}{
		{
			name: "Includes formatted activity logs in the DIP and workflow errors",
			logs: "- [ERROR] CH-000001: Export script failed\nSee the ACTApro logfile for details.",
			want: "ACTApro export failed or canceled. Logs:\n" +
				"- [ERROR] CH-000001: Export script failed\nSee the ACTApro logfile for details.",
		},
		{
			name: "Handles no logs",
			want: "ACTApro export failed or canceled.",
		},
	} {
		s.Run(tt.name, func() {
			s.testExportFailed(tt.logs, tt.want)
		})
	}
}

func (s *CreateDIPTestSuite) testExportFailed(logs, wantMessage string) {
	s.SetupTest()

	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()
	s.env.OnActivity(
		actapro.GetDocumentActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
	).Return(&actapro.GetDocumentResult{AIPUUIDs: []string{"28c1a3e2-abd3-4b9b-9214-ae851c87b1a6"}}, nil).Once()
	s.env.OnActivity(
		actapro.CreateExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.CreateExportParams{DocKey: s.dip.DocKey},
	).Return(&actapro.CreateExportResult{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"}, nil).Once()
	s.env.OnActivity(
		actapro.PollExportStatusActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.PollExportStatusParams{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"},
	).Return(&actapro.PollExportStatusResult{
		Status: "FAILED",
		Logs:   logs,
	}, nil).Once()

	wDIP.Status = enums.DIPStatusFailed
	wDIP.CompletedAt = createDIPTestTime
	wDIP.ErrorMessage = wantMessage
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	var applicationErr *temporalsdk_temporal.ApplicationError
	s.Require().True(errors.As(s.env.GetWorkflowError(), &applicationErr))
	s.Equal(wantMessage, applicationErr.Message())
}

func (s *CreateDIPTestSuite) TestGetDocumentFails() {
	for _, tt := range []struct {
		name         string
		err          error
		attempts     int
		errorMessage string
	}{
		{
			name:         "Retries transient errors",
			err:          errors.New("ACTApro unavailable"),
			attempts:     3,
			errorMessage: "ACTApro document retrieval failed: ACTApro unavailable",
		},
		{
			name:         "Does not retry permanent errors",
			err:          temporalsdk_temporal.NewNonRetryableApplicationError("document not found", "", nil),
			attempts:     1,
			errorMessage: "ACTApro document retrieval failed: document not found",
		},
	} {
		s.Run(tt.name, func() {
			s.SetupTest()

			wDIP := s.dip
			wDIP.Status = enums.DIPStatusInProgress
			wDIP.StartedAt = createDIPTestTime
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once()
			s.env.OnActivity(
				actapro.GetDocumentActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
			).Return(nil, tt.err).Times(tt.attempts)

			wDIP.Status = enums.DIPStatusFailed
			wDIP.CompletedAt = createDIPTestTime.Add(time.Duration(tt.attempts-1) * 5 * time.Second)
			wDIP.ErrorMessage = tt.errorMessage
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once()

			s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			s.ErrorContains(s.env.GetWorkflowError(), tt.err.Error())
		})
	}
}

func (s *CreateDIPTestSuite) TestExportActivityFails() {
	for _, tt := range []struct {
		name         string
		poll         bool
		download     bool
		err          error
		attempts     int
		errorMessage string
	}{
		{
			name:         "Retries transient creation errors",
			err:          errors.New("ACTApro unavailable"),
			attempts:     3,
			errorMessage: "ACTApro export creation failed: ACTApro unavailable",
		},
		{
			name:         "Does not retry permanent creation errors",
			err:          temporalsdk_temporal.NewNonRetryableApplicationError("unknown script", "", nil),
			attempts:     1,
			errorMessage: "ACTApro export creation failed: unknown script",
		},
		{
			name:         "Retries transient polling errors without recreating the export",
			poll:         true,
			err:          errors.New("ACTApro unavailable"),
			attempts:     3,
			errorMessage: "ACTApro export polling failed: ACTApro unavailable",
		},
		{
			name:         "Does not retry permanent polling errors",
			poll:         true,
			err:          temporalsdk_temporal.NewNonRetryableApplicationError("export not found", "", nil),
			attempts:     1,
			errorMessage: "ACTApro export polling failed: export not found",
		},
		{
			name:         "Retries transient download errors without recreating the export",
			download:     true,
			err:          errors.New("ACTApro unavailable"),
			attempts:     3,
			errorMessage: "ACTApro export download failed: ACTApro unavailable",
		},
		{
			name:         "Does not retry permanent download errors",
			download:     true,
			err:          temporalsdk_temporal.NewNonRetryableApplicationError("export not found", "", nil),
			attempts:     1,
			errorMessage: "ACTApro export download failed: export not found",
		},
	} {
		s.Run(tt.name, func() {
			s.SetupTest()

			wDIP := s.dip
			wDIP.Status = enums.DIPStatusInProgress
			wDIP.StartedAt = createDIPTestTime
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once()
			s.env.OnActivity(
				actapro.GetDocumentActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
			).Return(&actapro.GetDocumentResult{AIPUUIDs: []string{"28c1a3e2-abd3-4b9b-9214-ae851c87b1a6"}}, nil).Once()
			createExport := s.env.OnActivity(
				actapro.CreateExportActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&actapro.CreateExportParams{DocKey: s.dip.DocKey},
			)
			if tt.poll || tt.download {
				createExport.Return(&actapro.CreateExportResult{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"}, nil).
					Once()
				pollExport := s.env.OnActivity(
					actapro.PollExportStatusActivityName,
					mock.AnythingOfType("*context.timerCtx"),
					&actapro.PollExportStatusParams{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"},
				)
				if tt.download {
					pollExport.Return(&actapro.PollExportStatusResult{Status: "COMPLETED"}, nil).Once()
					downloadExport := s.env.OnActivity(
						actapro.DownloadExportActivityName,
						mock.AnythingOfType("*context.timerCtx"),
						&actapro.DownloadExportParams{
							ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
							MetadataPath: filepath.Join(s.workingDir, s.dip.UUID.String(), "metadata.xml"),
						},
					).Return(nil, tt.err).Times(tt.attempts)
					s.env.OnActivity(
						removepaths.Name,
						mock.AnythingOfType("*context.timerCtx"),
						&removepaths.Params{Paths: []string{filepath.Join(s.workingDir, s.dip.UUID.String())}},
					).Return(&removepaths.Result{}, nil).Once().NotBefore(downloadExport)
				} else {
					pollExport.Return(nil, tt.err).Times(tt.attempts)
				}
			} else {
				createExport.Return(nil, tt.err).Times(tt.attempts)
			}

			wDIP.Status = enums.DIPStatusFailed
			wDIP.CompletedAt = createDIPTestTime.Add(time.Duration(tt.attempts-1) * 5 * time.Second)
			wDIP.ErrorMessage = tt.errorMessage
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once()

			s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			s.ErrorContains(s.env.GetWorkflowError(), tt.err.Error())
		})
	}
}

func (s *CreateDIPTestSuite) TestBothUpdatesFail() {
	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(nil, errors.New("initial update failed")).Times(3)

	wDIP.Status = enums.DIPStatusFailed
	wDIP.CompletedAt = createDIPTestTime.Add(2 * time.Second)
	wDIP.ErrorMessage = "DIP persistence update failed."
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(nil, errors.New("final update failed")).Times(3)

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	err := s.env.GetWorkflowError()
	s.ErrorContains(err, "initial update failed")
	s.ErrorContains(err, "final update failed")
}

func (s *CreateDIPTestSuite) TestCancellationPersistsFinalStatus() {
	for _, tt := range []struct {
		name                  string
		activityDelay         time.Duration
		getDocument           bool
		cancelAfterCompletion bool
		wantStatus            enums.DIPStatus
		wantErrorMessage      string
	}{
		{
			name:             "While initial update is pending",
			activityDelay:    5 * time.Second,
			wantStatus:       enums.DIPStatusFailed,
			wantErrorMessage: "DIP persistence update failed.",
		},
		{
			name:                  "After initial update succeeds",
			activityDelay:         time.Second,
			cancelAfterCompletion: true,
			wantStatus:            enums.DIPStatusFailed,
			wantErrorMessage:      "ACTApro document retrieval failed: canceled",
		},
		{
			name:             "While document request is pending",
			activityDelay:    5 * time.Second,
			getDocument:      true,
			wantStatus:       enums.DIPStatusFailed,
			wantErrorMessage: "ACTApro document retrieval failed: canceled",
		},
		{
			name:                  "After document request succeeds",
			activityDelay:         time.Second,
			getDocument:           true,
			cancelAfterCompletion: true,
			wantStatus:            enums.DIPStatusFailed,
			wantErrorMessage:      "ACTApro export creation failed: canceled",
		},
	} {
		s.Run(tt.name, func() {
			s.SetupTest()

			wDIP := s.dip
			wDIP.Status = enums.DIPStatusInProgress
			wDIP.StartedAt = createDIPTestTime
			initialUpdateDelay := tt.activityDelay
			if tt.getDocument {
				initialUpdateDelay = 0
				s.env.OnActivity(
					actapro.GetDocumentActivityName,
					mock.AnythingOfType("*context.timerCtx"),
					&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
				).Return(&actapro.GetDocumentResult{}, nil).After(tt.activityDelay).Once()
			}
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).After(initialUpdateDelay).Once()

			wDIP.Status = tt.wantStatus
			wDIP.CompletedAt = createDIPTestTime.Add(time.Second)
			wDIP.ErrorMessage = tt.wantErrorMessage
			if tt.wantStatus == enums.DIPStatusDone {
				wDIP.ObjectKey = "DIP_9390594f-84c2-457d-bd6a-618f21f7c954.zip"
			}
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).After(2 * time.Second).Once()

			var cancel temporalsdk_workflow.CancelFunc
			if tt.cancelAfterCompletion {
				activityName := activities.UpdateDIPName
				if tt.getDocument {
					activityName = actapro.GetDocumentActivityName
				}
				s.env.SetOnActivityCompletedListener(func(
					info *temporalsdk_activity.Info,
					_ temporalsdk_converter.EncodedValue,
					_ error,
				) {
					// Cancel after the activity succeeds, before the workflow resumes.
					if info.ActivityType.Name == activityName {
						cancel()
					}
				})
			} else {
				s.env.RegisterDelayedCallback(s.env.CancelWorkflow, time.Second)
			}
			s.env.ExecuteWorkflow(func(
				ctx temporalsdk_workflow.Context,
				params *workflows.CreateDIPParams,
			) (*workflows.CreateDIPResult, error) {
				ctx, cancel = temporalsdk_workflow.WithCancel(ctx)
				defer cancel()
				return s.workflow.Execute(ctx, params)
			}, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			if tt.wantStatus == enums.DIPStatusDone {
				s.NoError(s.env.GetWorkflowError())
				var result workflows.CreateDIPResult
				s.Require().NoError(s.env.GetWorkflowResult(&result))
				s.Equal(workflows.CreateDIPResult{DIP: wDIP}, result)
			} else {
				s.True(temporalsdk_temporal.IsCanceledError(s.env.GetWorkflowError()))
			}
			s.Equal(createDIPTestTime.Add(3*time.Second), s.env.Now().UTC())
		})
	}
}
