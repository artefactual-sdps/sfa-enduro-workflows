package workflows_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/artefactual-sdps/temporal-activities/archivezip"
	"github.com/artefactual-sdps/temporal-activities/bucketdelete"
	"github.com/artefactual-sdps/temporal-activities/bucketupload"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
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
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
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
	xsdDir     string
	dip        datatypes.DIP
	retention  retentionTestOptions
}

type retentionTestOptions struct {
	period           time.Duration
	finalUpdateDelay time.Duration
	finalUpdateErr   error
	deleteErr        error
	cancelAfter      time.Duration
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
		amss.NewGetAIPPathActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.GetAIPPathActivityName},
	)
	s.env.RegisterActivityWithOptions(
		amss.NewFetchActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.FetchActivityName},
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
		xmlvalidate.New(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: xmlvalidate.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewParseMetadata().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ParseMetadataName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewPrepareDIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.PrepareDIPName},
	)
	s.env.RegisterActivityWithOptions(
		archivezip.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: archivezip.Name},
	)
	s.env.RegisterActivityWithOptions(
		bucketupload.New(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: bucketupload.Name},
	)
	s.env.RegisterActivityWithOptions(
		bucketdelete.New(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: bucketdelete.Name},
	)
	s.env.RegisterActivityWithOptions(
		removepaths.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: removepaths.Name},
	)
	s.workingDir = s.T().TempDir()
	s.xsdDir = "/schemas/custom"
	s.retention = retentionTestOptions{period: -time.Second}
	s.workflow = workflows.NewCreateDIP(s.workingDir, s.xsdDir, s.retention.period)
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
	s.testSessionResult(nil, 0, nil, nil, nil, nil, nil, nil)
}

func (s *CreateDIPTestSuite) TestRetention() {
	for _, tt := range []struct {
		name string
		opts retentionTestOptions
	}{
		{
			name: "Waits after the final update",
			opts: retentionTestOptions{period: 48 * time.Hour, finalUpdateDelay: time.Minute},
		},
		{
			name: "Deletes immediately with zero retention",
			opts: retentionTestOptions{finalUpdateDelay: time.Minute},
		},
		{
			name: "Retains indefinitely with negative retention",
			opts: retentionTestOptions{period: -time.Hour},
		},
		{
			name: "Deletes immediately when the final update fails",
			opts: retentionTestOptions{
				period:           time.Hour,
				finalUpdateDelay: time.Minute,
				finalUpdateErr:   temporalsdk_temporal.NewNonRetryableApplicationError("final update failed", "", nil),
			},
		},
		{
			name: "Final update failure overrides indefinite retention",
			opts: retentionTestOptions{
				period:         -time.Second,
				finalUpdateErr: temporalsdk_temporal.NewNonRetryableApplicationError("final update failed", "", nil),
			},
		},
		{
			name: "Final update failure with zero retention deletes only once",
			opts: retentionTestOptions{
				finalUpdateErr: temporalsdk_temporal.NewNonRetryableApplicationError("final update failed", "", nil),
			},
		},
		{
			name: "Returns both final update and deletion errors",
			opts: retentionTestOptions{
				period:         time.Hour,
				finalUpdateErr: temporalsdk_temporal.NewNonRetryableApplicationError("final update failed", "", nil),
				deleteErr:      temporalsdk_temporal.NewNonRetryableApplicationError("bucket deletion denied", "", nil),
			},
		},
		{
			name: "Deletion failure preserves the completed DIP",
			opts: retentionTestOptions{
				period:    time.Hour,
				deleteErr: temporalsdk_temporal.NewNonRetryableApplicationError("bucket deletion denied", "", nil),
			},
		},
		{
			name: "Cancellation during retention skips deletion",
			opts: retentionTestOptions{period: 48 * time.Hour, cancelAfter: time.Hour},
		},
	} {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.retention = tt.opts
			s.testSessionResult(nil, time.Second, nil, nil, nil, nil, nil, nil)
		})
	}
}

func (s *CreateDIPTestSuite) TestRetentionSkipsFailedDIP() {
	s.retention.period = time.Hour
	s.testSessionResult(nil, 0, nil, nil, nil, nil, nil,
		temporalsdk_temporal.NewNonRetryableApplicationError("bucket upload denied", "", nil),
	)
}

func (s *CreateDIPTestSuite) TestFinalUpdateFailureDeletesAfterCancellation() {
	s.retention.finalUpdateErr = temporalsdk_temporal.NewNonRetryableApplicationError("final update failed", "", nil)
	s.env.RegisterDelayedCallback(s.env.CancelWorkflow, time.Second)
	s.testSessionResult(nil, 2*time.Second, nil, nil, nil, nil, nil, nil)
}

func (s *CreateDIPTestSuite) TestFinalUpdateFailureBeforeUploadSkipsDeletion() {
	s.retention.finalUpdateErr = temporalsdk_temporal.NewNonRetryableApplicationError("final update failed", "", nil)
	s.testSessionResult(nil, 0, nil, nil, nil, nil, nil,
		temporalsdk_temporal.NewNonRetryableApplicationError("bucket upload denied", "", nil),
	)
	s.ErrorContains(s.env.GetWorkflowError(), "bucket upload denied")
}

func (s *CreateDIPTestSuite) TestCleanupFailureDoesNotFailWorkflow() {
	s.testSessionResult(errors.New("remove paths failed"), 0, nil, nil, nil, nil, nil, nil)
}

func (s *CreateDIPTestSuite) TestCleanupCompletesAfterCancellation() {
	s.env.RegisterDelayedCallback(s.env.CancelWorkflow, time.Second)
	s.testSessionResult(nil, 2*time.Second, nil, nil, nil, nil, nil, nil)
	s.Equal(createDIPTestTime.Add(2*time.Second), s.env.Now().UTC())
}

func (s *CreateDIPTestSuite) TestParseMetadataFails() {
	s.testSessionResult(nil, 0, nil, errors.New(
		"files not found in AIP METS:\n_file1 (content/file1.jp2)\n_file2 (content/file2.jp2)",
	), nil, nil, nil, nil)
}

func (s *CreateDIPTestSuite) TestMetadataContainsNoFiles() {
	s.testSessionResult(nil, 0, &activities.ParseMetadataResult{}, nil, nil, nil, nil, nil)
}

func (s *CreateDIPTestSuite) TestPrepareDIPFails() {
	s.testSessionResult(nil, 0, nil, nil, errors.New("move DIP metadata: permission denied"), nil, nil, nil)
}

func (s *CreateDIPTestSuite) TestDownloadContentFails() {
	s.testSessionResult(nil, 0, nil, nil, nil,
		temporalsdk_temporal.NewNonRetryableApplicationError("content file not found", "", nil),
		nil, nil,
	)
}

func (s *CreateDIPTestSuite) TestArchiveDIPFails() {
	s.testSessionResult(nil, 0, nil, nil, nil, nil,
		errors.New("archivezip: create destination: permission denied"),
		nil,
	)
}

func (s *CreateDIPTestSuite) TestUploadDIPFails() {
	s.testSessionResult(nil, 0, nil, nil, nil, nil, nil,
		temporalsdk_temporal.NewNonRetryableApplicationError("bucket upload denied", "", nil),
	)
}

func (s *CreateDIPTestSuite) testSessionResult(
	cleanupErr error,
	cleanupDelay time.Duration,
	parseResult *activities.ParseMetadataResult,
	parseErr error,
	prepareErr error,
	contentErr error,
	archiveErr error,
	uploadErr error,
) {
	s.T().Helper()
	s.workflow = workflows.NewCreateDIP(s.workingDir, s.xsdDir, s.retention.period)

	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	initialUpdate := s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()
	aipUUIDs := []uuid.UUID{
		uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6"),
		uuid.MustParse("1231e569-a94e-4ac1-873e-65e1f524b1c8"),
		uuid.MustParse("a38c3e43-c6c3-42f6-a7a0-8227c378deab"),
	}
	aipExtensions := []string{".7z", ".tar.gz", ""}
	getDocument := s.env.OnActivity(
		actapro.GetDocumentActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
	).Return(&actapro.GetDocumentResult{
		AIPUUIDs: aipUUIDs,
	}, nil).Once().NotBefore(initialUpdate)
	getAIPPath := getDocument
	for i, aipUUID := range aipUUIDs {
		getAIPPath = s.env.OnActivity(
			amss.GetAIPPathActivityName,
			mock.AnythingOfType("*context.timerCtx"),
			&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
		).Return(&amss.GetAIPPathActivityResult{
			Path: "aa/bb/test-" + aipUUID.String() + aipExtensions[i],
		}, nil).Once().NotBefore(getAIPPath)
	}
	createExport := s.env.OnActivity(
		actapro.CreateExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&actapro.CreateExportParams{DocKey: s.dip.DocKey},
	).Return(&actapro.CreateExportResult{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"}, nil).
		Once().NotBefore(getAIPPath)
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
	validateExport := s.env.OnActivity(
		xmlvalidate.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&xmlvalidate.Params{
			XMLPath: filepath.Join(s.workingDir, s.dip.UUID.String(), "metadata.xml"),
			XSDPath: filepath.Join(s.xsdDir, "arelda.xsd"),
		},
	).Return(&xmlvalidate.Result{}, nil).Once().NotBefore(downloadExport)
	previousActivity := validateExport
	parseParams := &activities.ParseMetadataParams{
		MetadataPath: filepath.Join(s.workingDir, s.dip.UUID.String(), "metadata.xml"),
	}
	for _, aipUUID := range aipUUIDs {
		metsName := "METS." + aipUUID.String() + ".xml"
		parseParams.AIPs = append(parseParams.AIPs, activities.AIP{
			UUID:     aipUUID,
			DirName:  "test-" + aipUUID.String(),
			METSPath: filepath.Join(s.workingDir, s.dip.UUID.String(), metsName),
		})
		fetchMETS := s.env.OnActivity(
			amss.FetchActivityName,
			mock.AnythingOfType("*context.timerCtx"),
			&amss.FetchActivityParams{
				AIPUUID:      aipUUID,
				RelativePath: "test-" + aipUUID.String() + "/data/" + metsName,
				Destination:  filepath.Join(s.workingDir, s.dip.UUID.String(), metsName),
			},
		).Return(&amss.FetchActivityResult{}, nil).Once().NotBefore(previousActivity)
		previousActivity = fetchMETS
	}
	parseMetadata := s.env.OnActivity(
		activities.ParseMetadataName,
		mock.AnythingOfType("*context.timerCtx"),
		parseParams,
	).Once().NotBefore(previousActivity)
	if parseErr != nil {
		parseMetadata.Return(nil, parseErr)
	} else {
		if parseResult == nil {
			// Cover compressed and directory AIPs, nested destinations, and repeated AIPs.
			parseResult = &activities.ParseMetadataResult{Files: []*datatypes.File{
				{
					DateiID: "_file1", DIPPath: "content/file1.jp2", Checksum: "checksum1", ChecksumAlgorithm: "MD5",
					AIPUUID: aipUUIDs[0], AIPPath: "test-" + aipUUIDs[0].String() + "/data/objects/file1.jp2",
				},
				{
					DateiID:           "_file2",
					DIPPath:           "content/dossier/file2.pdf",
					Checksum:          "checksum2",
					ChecksumAlgorithm: "MD5",
					AIPUUID:           aipUUIDs[1],
					AIPPath:           "test-" + aipUUIDs[1].String() + "/data/objects/originals/file2.pdf",
				},
				{
					DateiID:           "_file3",
					DIPPath:           "content/dossier/nested/file3.txt",
					Checksum:          "checksum3",
					ChecksumAlgorithm: "MD5",
					AIPUUID:           aipUUIDs[2],
					AIPPath:           "test-" + aipUUIDs[2].String() + "/data/objects/file3.txt",
				},
				{
					DateiID: "_file4", DIPPath: "content/file4.jp2", Checksum: "checksum4", ChecksumAlgorithm: "MD5",
					AIPUUID: aipUUIDs[0], AIPPath: "test-" + aipUUIDs[0].String() + "/data/objects/file4.jp2",
				},
			}}
		}
		parseMetadata.Return(parseResult, nil)
	}
	previousActivity = parseMetadata
	var contentErrorMessage string
	if parseErr == nil && len(parseResult.Files) > 0 {
		previousActivity = s.env.OnActivity(
			activities.PrepareDIPName,
			mock.AnythingOfType("*context.timerCtx"),
			&activities.PrepareDIPParams{
				DIPPath:      filepath.Join(s.workingDir, s.dip.UUID.String(), "DIP_"+s.dip.UUID.String()),
				MetadataPath: parseParams.MetadataPath,
				XSDDir:       s.xsdDir,
			},
		).Return(&activities.PrepareDIPResult{}, prepareErr).Once().NotBefore(parseMetadata)
		if prepareErr == nil {
			for i, file := range parseResult.Files {
				fetch := s.env.OnActivity(
					amss.FetchActivityName,
					mock.AnythingOfType("*context.timerCtx"),
					&amss.FetchActivityParams{
						AIPUUID:      file.AIPUUID,
						RelativePath: file.AIPPath,
						Destination: filepath.Join(
							s.workingDir,
							s.dip.UUID.String(),
							"DIP_"+s.dip.UUID.String(),
							file.DIPPath,
						),
					},
				).Once().NotBefore(previousActivity)
				previousActivity = fetch
				if contentErr != nil && i == 1 {
					// Fail the second download; later files must not be fetched.
					fetch.Return(nil, contentErr)
					contentErrorMessage = "AMSS content download failed for " +
						"\"test-1231e569-a94e-4ac1-873e-65e1f524b1c8/data/objects/originals/file2.pdf\" " +
						"(AIP 1231e569-a94e-4ac1-873e-65e1f524b1c8): content file not found"
					break
				}
				fetch.Return(&amss.FetchActivityResult{}, nil)
			}
			if contentErr == nil {
				// Archive after every file is downloaded; cleanup must wait for it.
				dipPath := filepath.Join(s.workingDir, s.dip.UUID.String(), "DIP_"+s.dip.UUID.String())
				previousActivity = s.env.OnActivity(
					archivezip.Name,
					mock.AnythingOfType("*context.timerCtx"),
					&archivezip.Params{SourceDir: dipPath},
				).Return(&archivezip.Result{Path: dipPath + ".zip"}, archiveErr).Once().NotBefore(previousActivity)
				if archiveErr == nil {
					// Use a distinct key to verify the workflow stores the upload result.
					previousActivity = s.env.OnActivity(
						bucketupload.Name,
						mock.AnythingOfType("*context.timerCtx"),
						&bucketupload.Params{Path: dipPath + ".zip"},
					).Return(&bucketupload.Result{
						Key: "uploads/DIP_9390594f-84c2-457d-bd6a-618f21f7c954.zip",
					}, uploadErr).Once().NotBefore(previousActivity)
				}
			}
		}
	}
	cleanup := s.env.OnActivity(
		removepaths.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&removepaths.Params{Paths: []string{filepath.Join(s.workingDir, s.dip.UUID.String())}},
	).Return(&removepaths.Result{}, cleanupErr).After(cleanupDelay).Once().NotBefore(previousActivity)

	wDIP.ObjectKey = "uploads/DIP_9390594f-84c2-457d-bd6a-618f21f7c954.zip"
	wDIP.Status = enums.DIPStatusDone
	if parseErr != nil {
		wDIP.ObjectKey = ""
		wDIP.Status = enums.DIPStatusFailed
		wDIP.ErrorMessage = "DIP metadata parsing failed: " + parseErr.Error()
	} else if len(parseResult.Files) == 0 {
		wDIP.ObjectKey = ""
		wDIP.Status = enums.DIPStatusFailed
		wDIP.ErrorMessage = "DIP metadata contains no files."
	} else if prepareErr != nil {
		wDIP.ObjectKey = ""
		wDIP.Status = enums.DIPStatusFailed
		wDIP.ErrorMessage = "DIP preparation failed: " + prepareErr.Error()
	} else if contentErr != nil {
		wDIP.ObjectKey = ""
		wDIP.Status = enums.DIPStatusFailed
		wDIP.ErrorMessage = contentErrorMessage
	} else if archiveErr != nil {
		wDIP.ObjectKey = ""
		wDIP.Status = enums.DIPStatusFailed
		wDIP.ErrorMessage = "DIP ZIP creation failed: " + archiveErr.Error()
	} else if uploadErr != nil {
		wDIP.ObjectKey = ""
		wDIP.Status = enums.DIPStatusFailed
		wDIP.ErrorMessage = "DIP upload failed: bucket upload denied"
	}
	wDIP.CompletedAt = createDIPTestTime.Add(cleanupDelay)
	finalUpdate := s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, s.retention.finalUpdateErr).
		After(s.retention.finalUpdateDelay).Once().NotBefore(cleanup)

	wantFinish := wDIP.CompletedAt.Add(s.retention.finalUpdateDelay)
	deleteImmediately := s.retention.finalUpdateErr != nil && wDIP.ObjectKey != ""
	deleteAfterRetention := wDIP.Status == enums.DIPStatusDone && s.retention.finalUpdateErr == nil &&
		s.retention.period >= 0
	if deleteAfterRetention {
		if s.retention.cancelAfter > 0 {
			wantFinish = wantFinish.Add(s.retention.cancelAfter)
			s.env.RegisterDelayedCallback(s.env.CancelWorkflow, wantFinish.Sub(createDIPTestTime))
			deleteAfterRetention = false
		} else {
			wantFinish = wantFinish.Add(s.retention.period)
		}
	}
	if deleteImmediately || deleteAfterRetention {
		s.env.OnActivity(
			bucketdelete.Name,
			mock.AnythingOfType("*context.timerCtx"),
			&bucketdelete.Params{Key: wDIP.ObjectKey},
		).Return(&bucketdelete.Result{}, s.retention.deleteErr).Once().NotBefore(finalUpdate).
			Run(func(mock.Arguments) {
				s.Equal(wantFinish, s.env.Now().UTC())
			})
	}

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	if !deleteImmediately && !deleteAfterRetention {
		s.env.AssertNotCalled(s.T(), bucketdelete.Name, mock.Anything, mock.Anything)
	}
	if wDIP.Status == enums.DIPStatusDone {
		s.Equal(wantFinish, s.env.Now().UTC())
	}
	if s.retention.finalUpdateErr != nil {
		s.ErrorContains(s.env.GetWorkflowError(), "final update failed")
		if s.retention.deleteErr != nil {
			s.ErrorContains(s.env.GetWorkflowError(), "bucket deletion denied")
		}
		return
	}
	if parseErr != nil {
		s.ErrorContains(s.env.GetWorkflowError(), parseErr.Error())
		return
	}
	if len(parseResult.Files) == 0 {
		applicationErr, ok := errors.AsType[*temporalsdk_temporal.ApplicationError](s.env.GetWorkflowError())
		s.Require().True(ok)
		s.Equal(wDIP.ErrorMessage, applicationErr.Message())
		return
	}
	if prepareErr != nil {
		s.ErrorContains(s.env.GetWorkflowError(), prepareErr.Error())
		return
	}
	if contentErr != nil {
		s.ErrorContains(s.env.GetWorkflowError(), "content file not found")
		return
	}
	if archiveErr != nil {
		s.ErrorContains(s.env.GetWorkflowError(), archiveErr.Error())
		return
	}
	if uploadErr != nil {
		s.ErrorContains(s.env.GetWorkflowError(), "bucket upload denied")
		return
	}
	s.NoError(s.env.GetWorkflowError())

	var result workflows.CreateDIPResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(workflows.CreateDIPResult{DIP: wDIP}, result)
}

func (s *CreateDIPTestSuite) TestSessionRecoveryClearsDownloadError() {
	aipUUID := uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")
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
	).Return(&actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{aipUUID}}, nil).Once()
	s.env.OnActivity(
		amss.GetAIPPathActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
	).Return(&amss.GetAIPPathActivityResult{Path: "test-" + aipUUID.String() + ".7z"}, nil).Once()
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
	downloadExport := s.env.OnActivity(
		actapro.DownloadExportActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		downloadParams,
	).Return(&actapro.DownloadExportResult{}, nil).Once().NotBefore(failedDownload)
	validateExport := s.env.OnActivity(
		xmlvalidate.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&xmlvalidate.Params{
			XMLPath: downloadParams.MetadataPath,
			XSDPath: filepath.Join(s.xsdDir, "arelda.xsd"),
		},
	).Return(&xmlvalidate.Result{}, nil).Once().NotBefore(downloadExport)
	metsName := "METS." + aipUUID.String() + ".xml"
	metsPath := filepath.Join(s.workingDir, s.dip.UUID.String(), metsName)
	fetchMETS := s.env.OnActivity(
		amss.FetchActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&amss.FetchActivityParams{
			AIPUUID:      aipUUID,
			RelativePath: "test-" + aipUUID.String() + "/data/" + metsName,
			Destination:  metsPath,
		},
	).Return(&amss.FetchActivityResult{}, nil).Once().NotBefore(validateExport)
	parseMetadata := s.env.OnActivity(
		activities.ParseMetadataName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.ParseMetadataParams{
			MetadataPath: downloadParams.MetadataPath,
			AIPs: []activities.AIP{
				{UUID: aipUUID, DirName: "test-" + aipUUID.String(), METSPath: metsPath},
			},
		},
	).Return(&activities.ParseMetadataResult{Files: []*datatypes.File{{
		DateiID: "_file1", DIPPath: "content/file1.jp2", Checksum: "checksum", ChecksumAlgorithm: "MD5",
		AIPUUID: aipUUID, AIPPath: "test-" + aipUUID.String() + "/data/objects/file1.jp2",
	}}}, nil).Once().NotBefore(fetchMETS)
	prepareDIP := s.env.OnActivity(
		activities.PrepareDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.PrepareDIPParams{
			DIPPath:      filepath.Join(s.workingDir, s.dip.UUID.String(), "DIP_"+s.dip.UUID.String()),
			MetadataPath: downloadParams.MetadataPath,
			XSDDir:       s.xsdDir,
		},
	).Return(&activities.PrepareDIPResult{}, nil).Once().NotBefore(parseMetadata)
	fetchContent := s.env.OnActivity(
		amss.FetchActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&amss.FetchActivityParams{
			AIPUUID:      aipUUID,
			RelativePath: "test-" + aipUUID.String() + "/data/objects/file1.jp2",
			Destination: filepath.Join(
				s.workingDir,
				s.dip.UUID.String(),
				"DIP_"+s.dip.UUID.String(),
				"content",
				"file1.jp2",
			),
		},
	).Return(&amss.FetchActivityResult{}, nil).Once().NotBefore(prepareDIP)
	dipPath := filepath.Join(s.workingDir, s.dip.UUID.String(), "DIP_"+s.dip.UUID.String())
	archiveDIP := s.env.OnActivity(
		archivezip.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&archivezip.Params{SourceDir: dipPath},
	).Return(&archivezip.Result{Path: dipPath + ".zip"}, nil).Once().NotBefore(fetchContent)
	s.env.OnActivity(
		bucketupload.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&bucketupload.Params{Path: dipPath + ".zip"},
	).Return(&bucketupload.Result{Key: filepath.Base(dipPath) + ".zip"}, nil).Once().NotBefore(archiveDIP)
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
	s.T().Helper()

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
	).Return(&actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")}}, nil).Once()
	s.env.OnActivity(
		amss.GetAIPPathActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&amss.GetAIPPathActivityParams{AIPUUID: uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")},
	).Return(&amss.GetAIPPathActivityResult{Path: "aip.7z"}, nil).Once()
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

func (s *CreateDIPTestSuite) TestDocumentContainsNoAIPs() {
	for _, tt := range []struct {
		name     string
		aipUUIDs []uuid.UUID
	}{
		{name: "Nil AIP list"},
		{name: "Empty AIP list", aipUUIDs: []uuid.UUID{}},
	} {
		s.Run(tt.name, func() {
			s.SetupTest()

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
			).Return(&actapro.GetDocumentResult{AIPUUIDs: tt.aipUUIDs}, nil).Once().NotBefore(initialUpdate)

			wDIP.Status = enums.DIPStatusFailed
			wDIP.CompletedAt = createDIPTestTime
			wDIP.ErrorMessage = "ACTApro document contains no AIPs."
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once().NotBefore(getDocument)

			s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			var applicationErr *temporalsdk_temporal.ApplicationError
			s.Require().True(errors.As(s.env.GetWorkflowError(), &applicationErr))
			s.Equal(wDIP.ErrorMessage, applicationErr.Message())
			s.env.AssertNotCalled(s.T(), amss.GetAIPPathActivityName, mock.Anything, mock.Anything)
			s.env.AssertNotCalled(s.T(), actapro.CreateExportActivityName, mock.Anything, mock.Anything)
		})
	}
}

func (s *CreateDIPTestSuite) TestGetAIPPathsFails() {
	aipUUIDs := []uuid.UUID{
		uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6"),
		uuid.MustParse("1231e569-a94e-4ac1-873e-65e1f524b1c8"),
		uuid.MustParse("a38c3e43-c6c3-42f6-a7a0-8227c378deab"),
	}
	for _, tt := range []struct {
		name         string
		errs         []error
		attempts     int
		errorMessage string
	}{
		{
			name: "Collects paths after a lookup fails",
			errs: []error{
				temporalsdk_temporal.NewNonRetryableApplicationError("package not found", "", nil),
				nil,
				nil,
			},
			attempts: 1,
			errorMessage: "AMSS AIP path retrieval failed:\n" +
				"AIP 28c1a3e2-abd3-4b9b-9214-ae851c87b1a6: package not found",
		},
		{
			name: "Includes all lookup errors alongside successful paths",
			errs: []error{
				temporalsdk_temporal.NewNonRetryableApplicationError("package not found", "", nil),
				nil,
				temporalsdk_temporal.NewNonRetryableApplicationError("current path missing", "", nil),
			},
			attempts: 1,
			errorMessage: "AMSS AIP path retrieval failed:\n" +
				"AIP 28c1a3e2-abd3-4b9b-9214-ae851c87b1a6: package not found\n" +
				"AIP a38c3e43-c6c3-42f6-a7a0-8227c378deab: current path missing",
		},
		{
			name: "Retries transient lookup errors before collecting them",
			errs: []error{
				errors.New("AMSS unavailable"),
				nil,
				errors.New("AMSS request failed"),
			},
			attempts: 3,
			errorMessage: "AMSS AIP path retrieval failed:\n" +
				"AIP 28c1a3e2-abd3-4b9b-9214-ae851c87b1a6: AMSS unavailable\n" +
				"AIP a38c3e43-c6c3-42f6-a7a0-8227c378deab: AMSS request failed",
		},
	} {
		s.Run(tt.name, func() {
			s.SetupTest()

			wDIP := s.dip
			wDIP.Status = enums.DIPStatusInProgress
			wDIP.StartedAt = createDIPTestTime
			initialUpdate := s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once()
			previous := s.env.OnActivity(
				actapro.GetDocumentActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&actapro.GetDocumentParams{DocKey: s.dip.DocKey},
			).Return(&actapro.GetDocumentResult{AIPUUIDs: aipUUIDs}, nil).Once().NotBefore(initialUpdate)

			var retryDelay time.Duration
			for i, aipUUID := range aipUUIDs {
				lookup := s.env.OnActivity(
					amss.GetAIPPathActivityName,
					mock.AnythingOfType("*context.timerCtx"),
					&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
				).NotBefore(previous)
				if tt.errs[i] != nil {
					lookup.Return(nil, tt.errs[i]).Times(tt.attempts)
					retryDelay += time.Duration(tt.attempts-1) * 5 * time.Second
				} else {
					lookup.Return(&amss.GetAIPPathActivityResult{Path: aipUUID.String() + ".7z"}, nil).Once()
				}
				previous = lookup
			}

			wDIP.Status = enums.DIPStatusFailed
			wDIP.CompletedAt = createDIPTestTime.Add(retryDelay)
			wDIP.ErrorMessage = tt.errorMessage
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once().NotBefore(previous)

			s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			var applicationErr *temporalsdk_temporal.ApplicationError
			s.Require().True(errors.As(s.env.GetWorkflowError(), &applicationErr))
			s.Equal(tt.errorMessage, applicationErr.Message())
			s.env.AssertNotCalled(s.T(), actapro.CreateExportActivityName, mock.Anything, mock.Anything)
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
			).Return(&actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")}}, nil).Once()
			s.env.OnActivity(
				amss.GetAIPPathActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&amss.GetAIPPathActivityParams{AIPUUID: uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")},
			).Return(&amss.GetAIPPathActivityResult{Path: "aip.7z"}, nil).Once()
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

func (s *CreateDIPTestSuite) TestDownloadAIPMETSFails() {
	aipUUIDs := []uuid.UUID{
		uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6"),
		uuid.MustParse("1231e569-a94e-4ac1-873e-65e1f524b1c8"),
		uuid.MustParse("a38c3e43-c6c3-42f6-a7a0-8227c378deab"),
	}
	for _, tt := range []struct {
		name         string
		errs         []error
		attempts     int
		errorMessage string
	}{
		{
			name: "Continues downloading after a failure",
			errs: []error{
				temporalsdk_temporal.NewNonRetryableApplicationError("METS not found", "", nil),
				nil,
				nil,
			},
			attempts: 1,
			errorMessage: "AMSS AIP METS download failed:\n" +
				"AIP 28c1a3e2-abd3-4b9b-9214-ae851c87b1a6: METS not found",
		},
		{
			name: "Collects every error alongside successful downloads",
			errs: []error{
				temporalsdk_temporal.NewNonRetryableApplicationError("METS not found", "", nil),
				nil,
				temporalsdk_temporal.NewNonRetryableApplicationError("permission denied", "", nil),
			},
			attempts: 1,
			errorMessage: "AMSS AIP METS download failed:\n" +
				"AIP 28c1a3e2-abd3-4b9b-9214-ae851c87b1a6: METS not found\n" +
				"AIP a38c3e43-c6c3-42f6-a7a0-8227c378deab: permission denied",
		},
		{
			name: "Retries transient errors before collecting them",
			errs: []error{
				errors.New("AMSS unavailable"),
				nil,
				errors.New("download interrupted"),
			},
			attempts: 3,
			errorMessage: "AMSS AIP METS download failed:\n" +
				"AIP 28c1a3e2-abd3-4b9b-9214-ae851c87b1a6: AMSS unavailable\n" +
				"AIP a38c3e43-c6c3-42f6-a7a0-8227c378deab: download interrupted",
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
			).Return(&actapro.GetDocumentResult{AIPUUIDs: aipUUIDs}, nil).Once()
			for _, aipUUID := range aipUUIDs {
				s.env.OnActivity(
					amss.GetAIPPathActivityName,
					mock.AnythingOfType("*context.timerCtx"),
					&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
				).Return(&amss.GetAIPPathActivityResult{Path: "aa/bb/test-" + aipUUID.String() + ".7z"}, nil).Once()
			}
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
			dipWorkingDir := filepath.Join(s.workingDir, s.dip.UUID.String())
			previous := s.env.OnActivity(
				actapro.DownloadExportActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&actapro.DownloadExportParams{
					ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
					MetadataPath: filepath.Join(dipWorkingDir, "metadata.xml"),
				},
			).Return(&actapro.DownloadExportResult{}, nil).Once()
			previous = s.env.OnActivity(
				xmlvalidate.Name,
				mock.AnythingOfType("*context.timerCtx"),
				&xmlvalidate.Params{
					XMLPath: filepath.Join(dipWorkingDir, "metadata.xml"),
					XSDPath: filepath.Join(s.xsdDir, "arelda.xsd"),
				},
			).Return(&xmlvalidate.Result{}, nil).Once().NotBefore(previous)

			var retryDelay time.Duration
			for i, aipUUID := range aipUUIDs {
				metsName := "METS." + aipUUID.String() + ".xml"
				fetch := s.env.OnActivity(
					amss.FetchActivityName,
					mock.AnythingOfType("*context.timerCtx"),
					&amss.FetchActivityParams{
						AIPUUID:      aipUUID,
						RelativePath: "test-" + aipUUID.String() + "/data/" + metsName,
						Destination:  filepath.Join(dipWorkingDir, metsName),
					},
				).NotBefore(previous)
				if tt.errs[i] != nil {
					fetch.Return(nil, tt.errs[i]).Times(tt.attempts)
					retryDelay += time.Duration(tt.attempts-1) * 5 * time.Second
				} else {
					fetch.Return(&amss.FetchActivityResult{}, nil).Once()
				}
				previous = fetch
			}
			cleanup := s.env.OnActivity(
				removepaths.Name,
				mock.AnythingOfType("*context.timerCtx"),
				&removepaths.Params{Paths: []string{dipWorkingDir}},
			).Return(&removepaths.Result{}, nil).Once().NotBefore(previous)

			wDIP.Status = enums.DIPStatusFailed
			wDIP.CompletedAt = createDIPTestTime.Add(retryDelay)
			wDIP.ErrorMessage = tt.errorMessage
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once().NotBefore(cleanup)

			s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			var applicationErr *temporalsdk_temporal.ApplicationError
			s.Require().True(errors.As(s.env.GetWorkflowError(), &applicationErr))
			s.Equal(tt.errorMessage, applicationErr.Message())
		})
	}
}

func (s *CreateDIPTestSuite) TestExportValidationFails() {
	aipUUID := uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")
	for _, tt := range []struct {
		name         string
		result       *xmlvalidate.Result
		err          error
		errorMessage string
	}{
		{
			name:         "Rejects XML that does not match the schema",
			result:       &xmlvalidate.Result{Failures: []string{"metadata.xml fails to validate"}},
			errorMessage: "ACTApro export validation failed:\nmetadata.xml fails to validate",
		},
		{
			name: "Includes all validation failures",
			result: &xmlvalidate.Result{Failures: []string{
				"metadata.xml: invalid schemaVersion",
				"metadata.xml: missing required element",
			}},
			errorMessage: "ACTApro export validation failed:\n" +
				"metadata.xml: invalid schemaVersion\nmetadata.xml: missing required element",
		},
		{
			name:         "Returns activity errors without retrying validation",
			err:          errors.New("xmlvalidate: xmllint not found"),
			errorMessage: "ACTApro export validation failed: xmlvalidate: xmllint not found",
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
			).Return(&actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{aipUUID}}, nil).Once()
			s.env.OnActivity(
				amss.GetAIPPathActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
			).Return(&amss.GetAIPPathActivityResult{Path: "aa/bb/test-" + aipUUID.String() + ".7z"}, nil).Once()
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
			metadataPath := filepath.Join(s.workingDir, s.dip.UUID.String(), "metadata.xml")
			downloadExport := s.env.OnActivity(
				actapro.DownloadExportActivityName,
				mock.AnythingOfType("*context.timerCtx"),
				&actapro.DownloadExportParams{
					ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
					MetadataPath: metadataPath,
				},
			).Return(&actapro.DownloadExportResult{}, nil).Once()
			validateExport := s.env.OnActivity(
				xmlvalidate.Name,
				mock.AnythingOfType("*context.timerCtx"),
				&xmlvalidate.Params{
					XMLPath: metadataPath,
					XSDPath: filepath.Join(s.xsdDir, "arelda.xsd"),
				},
			).Return(tt.result, tt.err).Once().NotBefore(downloadExport)
			cleanup := s.env.OnActivity(
				removepaths.Name,
				mock.AnythingOfType("*context.timerCtx"),
				&removepaths.Params{Paths: []string{filepath.Join(s.workingDir, s.dip.UUID.String())}},
			).Return(&removepaths.Result{}, nil).Once().NotBefore(validateExport)

			wDIP.Status = enums.DIPStatusFailed
			wDIP.CompletedAt = createDIPTestTime
			wDIP.ErrorMessage = tt.errorMessage
			s.env.OnActivity(
				activities.UpdateDIPName,
				mock.AnythingOfType("*context.timerCtx"),
				&activities.UpdateDIPParams{DIP: wDIP},
			).Return(&activities.UpdateDIPResult{}, nil).Once().NotBefore(cleanup)

			s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

			s.True(s.env.IsWorkflowCompleted())
			s.env.AssertExpectations(s.T())
			s.env.AssertNotCalled(s.T(), amss.FetchActivityName, mock.Anything, mock.Anything)
			if tt.err != nil {
				s.ErrorContains(s.env.GetWorkflowError(), tt.err.Error())
			} else {
				var applicationErr *temporalsdk_temporal.ApplicationError
				s.Require().True(errors.As(s.env.GetWorkflowError(), &applicationErr))
				s.Equal(tt.errorMessage, applicationErr.Message())
			}
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
	aipUUID := uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")
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
			wantErrorMessage:      "AMSS AIP path retrieval failed:\nAIP " + aipUUID.String() + ": canceled",
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
				).Return(&actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{aipUUID}}, nil).
					After(tt.activityDelay).Once()
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
			} else if tt.getDocument && tt.cancelAfterCompletion {
				// AIP path lookup errors are collected into an application error.
				s.ErrorContains(s.env.GetWorkflowError(), tt.wantErrorMessage)
			} else {
				s.True(temporalsdk_temporal.IsCanceledError(s.env.GetWorkflowError()))
			}
			s.Equal(createDIPTestTime.Add(3*time.Second), s.env.Now().UTC())
		})
	}
}
