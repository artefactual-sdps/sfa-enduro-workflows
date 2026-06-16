package workflows_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/archivezip"
	"github.com/artefactual-sdps/temporal-activities/bucketupload"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/cantons"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/workflows"
)

type CantonsPoststorageTestSuite struct {
	suite.Suite
	temporalsdk_testsuite.WorkflowTestSuite

	env      *temporalsdk_testsuite.TestWorkflowEnvironment
	workflow *workflows.PoststorageCantons
	testDir  string
}

func (s *CantonsPoststorageTestSuite) setup(cfg *config.PoststorageConfig) {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetWorkerOptions(temporalsdk_worker.Options{EnableSessionWorker: true})
	s.testDir = s.T().TempDir()
	cfg.WorkingDir = s.testDir

	s.env.RegisterActivityWithOptions(
		amss.NewGetAIPPathActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.GetAIPPathActivityName},
	)
	s.env.RegisterActivityWithOptions(
		amss.NewFetchActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.FetchActivityName},
	)
	s.env.RegisterActivityWithOptions(
		cantons.NewParseActivity().Execute,
		temporalsdk_activity.RegisterOptions{Name: cantons.ParseActivityName},
	)
	s.env.RegisterActivityWithOptions(
		cantons.NewCombineMDActivity().Execute,
		temporalsdk_activity.RegisterOptions{Name: cantons.CombineMDActivityName},
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
		removepaths.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: removepaths.Name},
	)

	s.workflow = workflows.NewPoststorageCantons(*cfg)
}

func TestCantonsPoststorage(t *testing.T) {
	suite.Run(t, new(CantonsPoststorageTestSuite))
}

func (s *CantonsPoststorageTestSuite) TestWorkflowSuccess() {
	aipUUID := uuid.MustParse("9390594f-84c2-457d-bd6a-618f21f7c954")

	s.setup(&config.PoststorageConfig{})
	s.mockCantonsActivitiesSuccess(aipUUID)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PostStorageParams{AIPUUID: aipUUID.String()},
	)

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())

	var result childwf.PostStorageResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)

	s.Equal(childwf.PostStorageResult{}, result)
}

func (s *CantonsPoststorageTestSuite) mockCantonsActivitiesSuccess(aipUUID uuid.UUID) {
	aipUUIDString := aipUUID.String()
	aipName := "test-" + aipUUIDString
	bundleName := fmt.Sprintf("search-md_%s", aipName)
	localDir := filepath.Join(s.testDir, bundleName)

	s.env.OnActivity(
		amss.GetAIPPathActivityName,
		mock.AnythingOfType("*context.timerCtx"),
		&amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
	).Return(
		&amss.GetAIPPathActivityResult{
			Path: "9390/594f/84c2/457d/bd6a/618f/21f7/c954/" + aipName + ".zip",
		}, nil,
	)

	sessionCtx := mock.AnythingOfType("*context.timerCtx")
	metsName := fmt.Sprintf("METS.%s.xml", aipUUIDString)
	metsPath := filepath.Join(localDir, metsName)
	s.env.OnActivity(
		amss.FetchActivityName,
		sessionCtx,
		&amss.FetchActivityParams{
			AIPUUID:      aipUUID,
			RelativePath: fmt.Sprintf("%s/data/%s", aipName, metsName),
			Destination:  metsPath,
		},
	).Return(
		&amss.FetchActivityResult{}, nil,
	)

	mdRelPath := "objects/header/metadata.xml"
	s.env.OnActivity(
		cantons.ParseActivityName,
		sessionCtx,
		&cantons.ParseActivityParams{METSPath: metsPath},
	).Return(
		&cantons.ParseActivityResult{MetadataRelPath: mdRelPath}, nil,
	)

	areldaPath := filepath.Join(localDir, "metadata.xml")
	s.env.OnActivity(
		amss.FetchActivityName,
		sessionCtx,
		&amss.FetchActivityParams{
			AIPUUID:      aipUUID,
			RelativePath: fmt.Sprintf("%s/data/%s", aipName, mdRelPath),
			Destination:  areldaPath,
		},
	).Return(
		&amss.FetchActivityResult{}, nil,
	)

	s.env.OnActivity(
		cantons.CombineMDActivityName,
		sessionCtx,
		cantons.CombineMDActivityParams{
			AreldaPath: areldaPath,
			METSPath:   metsPath,
			LocalDir:   localDir,
		},
	).Return(
		&cantons.CombineMDActivityResult{Path: filepath.Join(localDir, "AIS_1000_893_3251903")}, nil,
	)

	zipPath := localDir + ".zip"
	s.env.OnActivity(
		archivezip.Name,
		sessionCtx,
		&archivezip.Params{SourceDir: localDir},
	).Return(
		&archivezip.Result{Path: zipPath}, nil,
	)

	s.env.OnActivity(
		bucketupload.Name,
		sessionCtx,
		&bucketupload.Params{Path: zipPath},
	).Return(
		&bucketupload.Result{Key: bundleName + ".zip"}, nil,
	)
}
