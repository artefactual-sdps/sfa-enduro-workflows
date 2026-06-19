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
	"github.com/artefactual-sdps/temporal-activities/archiveextract"
	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/bagvalidate"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.artefactual.dev/tools/fsutil"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/localact"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/pips"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/workflows"
)

const (
	sipName                   = "SIP_20240606_dept.zip"
	apisTaskID                = "task-000001"
	preprocessingWorkflowUser = "sfa-enduro"

	// The relPath reflects an actual SFA ZIP path passed from Enduro to
	// sfa-enduro-workflows — it seems that ingest prepends "SIP_" to the original
	// file name and appends a UUID.
	relPath = "8fdfaea1-06ed-4cf6-8bdf-d15d80420f35/SIP_SIP_20240606_dept_8fdfaea1-06ed-4cf6-8bdf-d15d80420f35.zip"

	manifest = `
<?xml version="1.0" encoding="UTF-8"?>
<paket
	xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xmlns:xip="http://www.tessella.com/XIP/v4"
	xmlns="http://bar.admin.ch/arelda/v4"
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:submissionTests="http://bar.admin.ch/submissionTestResult" xsi:type="paketAIP" schemaVersion="5.0">
	<paketTyp>AIP</paketTyp>
	<globaleAIPId>909c56e9-e334-4c0a-9736-f92c732149d9</globaleAIPId>
	<lokaleAIPId>fa5fb285-fa45-44e4-8d85-77ec1d774403</lokaleAIPId>
	<version>1</version>
	<inhaltsverzeichnis>
		<ordner>
			<name>header</name>
			<ordner>
				<name>old</name>
				<ordner>
					<name>SIP_20240606_123456</name>
					<datei id="OLD_SIP">
						<name>metadata.xml</name>
						<originalName>metadata.xml</originalName>
						<pruefalgorithmus>MD5</pruefalgorithmus>
						<pruefsumme>43c533d499c572fca699e77e06295ba3</pruefsumme>
					</datei>
				</ordner>
			</ordner>
			<ordner>
				<name>xsd</name>
				<datei id="_xAlSBc3dYcypUMvN8HzeN5">
					<name>arelda.xsd</name>
					<originalName>arelda.xsd</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>f8454632e1ebf97e0aa8d9527ce2641f</pruefsumme>
				</datei>
			</ordner>
		</ordner>
		<ordner>
			<name>content</name>
			<ordner>
				<name>d_0000001</name>
				<datei id="_SRpeVgb4xGImymb23OH1od">
					<name>00000001_PREMIS.xml</name>
					<originalName>00000001_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>1428a269ff4e5b4894793b68646984b7</pruefsumme>
				</datei>
				<datei id="_MKhAIC639MxzyOn8ji3tN5">
					<name>00000002_PREMIS.xml</name>
					<originalName>00000002_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>f338f61911d2620972b0ac668dcc37ec</pruefsumme>
				</datei>
				<datei id="_fZzi3dX2jvrwakvY6jeJS8">
					<name>Prozess_Digitalisierung_PREMIS.xml</name>
					<originalName>Prozess_Digitalisierung_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>8067daaa900eba6dace69572eea8f8f3</pruefsumme>
				</datei>
				<datei id="_miEf29GTkFR7ymi91IV4fO">
					<name>00000001.jp2</name>
					<originalName>00000001.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>f7dc1f76a55cbdca0ae4a6dc8ae64644</pruefsumme>
				</datei>
				<datei id="_mOXw3hINt3zY6WvKQOfYmk">
					<name>00000002.jp2</name>
					<originalName>00000002.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>954d06be4a70c188b6b2e5fe4309fb2c</pruefsumme>
				</datei>
			</ordner>
		</ordner>
	</inhaltsverzeichnis>
</paket>
`
)

var (
	testTime = time.Date(2024, 6, 6, 15, 8, 39, 0, time.UTC)
	sipUUID  = uuid.MustParse("8fdfaea1-06ed-4cf6-8bdf-d15d80420f35")

	preAPISEvents = []*childwf.Task{
		{
			Name:        "Calculate SIP checksum",
			Message:     "SIP checksum calculated using SHA-256",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Check for duplicate SIP",
			Message:     "SIP is not a duplicate",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Extract SIP",
			Message:     "SIP extracted",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Validate Bag",
			Message:     "Bag successfully validated",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Unbag SIP",
			Message:     "SIP unbagged",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Identify SIP structure",
			Message:     "SIP structure identified: DigitizedAIP",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Validate SIP structure",
			Message:     "SIP structure matches validation criteria",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Validate SIP name",
			Message:     "SIP name matches expected naming convention for the identified structure type",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Verify SIP manifest",
			Message:     "SIP contents match manifest",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Verify SIP checksums",
			Message:     "SIP checksums match file contents",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Validate SIP file formats",
			Message:     "No invalid files found",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name: "Validate SIP metadata",
			Message: `Metadata validation successful on the following file(s):

- UpdatedAreldaMetadata.xml`,
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Validate logical metadata",
			Message:     "Logical metadata validation successful",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
	}

	postAPISEvents = []*childwf.Task{
		{
			Name:        "Create premis.xml",
			Message:     "Created a premis.xml file and stored it in the metadata directory",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Restructure SIP",
			Message:     "SIP has been restructured for preservation processing",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Create identifier.json",
			Message:     "Created an identifier.json file and stored it in the metadata directory",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		{
			Name:        "Bag SIP",
			Message:     "SIP has been bagged",
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
	}
)

type PreprocessingTestSuite struct {
	suite.Suite
	temporalsdk_testsuite.WorkflowTestSuite

	env      *temporalsdk_testsuite.TestWorkflowEnvironment
	workflow *workflows.Preprocessing
	testDir  string
	sipPath  string
}

func (s *PreprocessingTestSuite) SetupTest(cfg *config.Config) {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetStartTime(testTime)
	s.env.SetWorkerOptions(temporalsdk_worker.Options{EnableSessionWorker: true})
	s.testDir = s.T().TempDir()
	cfg.Preprocessing.SharedPath = s.testDir

	sp := filepath.Join(s.testDir, relPath)
	if err := os.MkdirAll(sp, os.FileMode(0o700)); err != nil {
		s.T().Fatalf("create sip dir: %v", err)
	}
	s.sipPath = sp

	// Register activities.
	s.env.RegisterActivityWithOptions(
		archiveextract.New(archiveextract.Config{}).Execute,
		temporalsdk_activity.RegisterOptions{Name: archiveextract.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewChecksumSIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ChecksumSIPName},
	)
	s.env.RegisterActivityWithOptions(
		bagvalidate.New(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: bagvalidate.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewUnbag().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.UnbagName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewIdentifySIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.IdentifySIPName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateStructure().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateStructureName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateSIPName().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateSIPNameName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewVerifyManifest().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.VerifyManifestName},
	)
	s.env.RegisterActivityWithOptions(
		ffvalidate.New(ffvalidate.Config{}).Execute,
		temporalsdk_activity.RegisterOptions{Name: ffvalidate.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateFiles(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateFilesName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISObjects(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISObjectsName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISEvent().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISEventName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISValidationEvent(nil, nil, nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISValidationEventName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISAgent().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISAgentName},
	)
	s.env.RegisterActivityWithOptions(
		xmlvalidate.New(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: xmlvalidate.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidatePREMIS(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidatePREMISName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewTransformSIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.TransformSIPName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewWriteIdentifierFile().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.WriteIdentifierFileName},
	)
	s.env.RegisterActivityWithOptions(
		apis.NewCreateImportTaskActivity(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: apis.CreateImportTaskActivityName},
	)
	s.env.RegisterActivityWithOptions(
		apis.NewPollImportTaskStatusActivity(nil, 0).Execute,
		temporalsdk_activity.RegisterOptions{Name: apis.PollImportTaskStatusActivityName},
	)
	s.env.RegisterActivityWithOptions(
		bagcreate.New(cfg.Preprocessing.BagCreate).Execute,
		temporalsdk_activity.RegisterOptions{Name: bagcreate.Name},
	)

	s.workflow = workflows.NewPreprocessing(nil, cfg.Preprocessing, cfg.APIS.Enabled)
	s.env.RegisterWorkflow(s.workflow.Execute)
}

func (s *PreprocessingTestSuite) digitizedAIP(path string) sip.SIP {
	fs.Apply(
		s.T(),
		fs.DirFromPath(s.T(), path),
		fs.WithDir("additional",
			fs.WithFile("UpdatedAreldaMetadata.xml", manifest),
		),
		fs.WithDir("content",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
					fs.WithFile("00000002.jp2", ""),
					fs.WithFile("00000002_PREMIS.xml", ""),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			),
			fs.WithDir("header",
				fs.WithDir("old",
					fs.WithDir("SIP_20240606_123456",
						fs.WithFile("metadata.xml", ""),
					),
				),
			),
		),
	)

	r, err := sip.New(path)
	if err != nil {
		s.T().Fatalf("Couldn't create SIP: %v", err)
	}

	return r
}

func (s *PreprocessingTestSuite) bornDigitalSIP(path string) sip.SIP {
	fs.Apply(
		s.T(),
		fs.DirFromPath(s.T(), path),
		fs.WithDir("content",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
					fs.WithFile("00000002.jp2", ""),
					fs.WithFile("00000002_PREMIS.xml", ""),
				),
			),
			fs.WithDir("header",
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", ""),
				),
				fs.WithFile("metadata.xml", ""),
			),
		),
	)

	r, err := sip.New(path)
	if err != nil {
		s.T().Fatalf("Couldn't create SIP: %v", err)
	}

	return r
}

func (s *PreprocessingTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func TestPreprocessingWorkflow(t *testing.T) {
	suite.Run(t, new(PreprocessingTestSuite))
}

func (s *PreprocessingTestSuite) writeBagitTxt(path string) {
	if err := os.WriteFile(
		filepath.Join(path, "bagit.txt"),
		[]byte(`
BagIt-Version: 0.97
Tag-File-Character-Encoding: UTF-8
`),
		os.FileMode(0o640),
	); err != nil {
		s.T().Fatalf("write bagit.txt: %v", err)
	}
}

func (s *PreprocessingTestSuite) preAPISActivities(ar apisgen.AnalysisResult) (sip.SIP, string, string) {
	extractPath := filepath.Join(filepath.Dir(s.sipPath), fsutil.BaseNoExt(filepath.Base(sipName)))
	expectedSIP := s.digitizedAIP(extractPath)
	ctx := mock.AnythingOfType("*context.valueCtx")
	sessionCtx := mock.AnythingOfType("*context.timerCtx")
	checksum := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	s.env.OnActivity(
		activities.ChecksumSIPName,
		sessionCtx,
		&activities.ChecksumSIPParams{Path: s.sipPath},
	).Return(
		&activities.ChecksumSIPResult{Algo: "SHA-256", Hash: checksum}, nil,
	)
	s.env.OnActivity(
		localact.CheckDuplicate,
		ctx,
		nil,
		&localact.CheckDuplicateParams{
			Name:     filepath.Base(s.sipPath),
			Checksum: checksum,
		},
	).Return(
		&localact.CheckDuplicateResult{}, nil,
	)
	s.env.OnActivity(
		archiveextract.Name,
		sessionCtx,
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(
		&archiveextract.Result{ExtractPath: extractPath}, nil,
	)
	s.env.OnActivity(
		localact.IsBag,
		ctx,
		&localact.IsBagParams{Path: extractPath},
	).Return(
		&localact.IsBagResult{IsBag: true}, nil,
	)
	s.env.OnActivity(
		bagvalidate.Name,
		sessionCtx,
		&bagvalidate.Params{Path: extractPath},
	).Return(
		&bagvalidate.Result{Valid: true}, nil,
	)
	s.env.OnActivity(
		activities.UnbagName,
		sessionCtx,
		&activities.UnbagParams{Path: extractPath},
	).Return(
		&activities.UnbagResult{Path: extractPath}, nil,
	)
	s.env.OnActivity(
		activities.IdentifySIPName,
		sessionCtx,
		&activities.IdentifySIPParams{Path: extractPath},
	).Return(
		&activities.IdentifySIPResult{SIP: expectedSIP}, nil,
	)
	s.env.OnActivity(
		activities.ValidateStructureName,
		sessionCtx,
		&activities.ValidateStructureParams{SIP: expectedSIP},
	).Return(
		&activities.ValidateStructureResult{}, nil,
	)
	s.env.OnActivity(
		activities.ValidateSIPNameName,
		sessionCtx,
		&activities.ValidateSIPNameParams{SIP: expectedSIP},
	).Return(
		&activities.ValidateSIPNameResult{}, nil,
	)
	s.env.OnActivity(
		activities.VerifyManifestName,
		sessionCtx,
		&activities.VerifyManifestParams{SIP: expectedSIP},
	).Return(
		&activities.VerifyManifestResult{}, nil,
	)
	s.env.OnActivity(
		activities.ValidateFilesName,
		sessionCtx,
		&activities.ValidateFilesParams{SIP: expectedSIP},
	).Return(
		&activities.ValidateFilesResult{}, nil,
	)
	s.env.OnActivity(
		xmlvalidate.Name,
		sessionCtx,
		&xmlvalidate.Params{
			XMLPath: expectedSIP.ManifestPath,
			XSDPath: expectedSIP.XSDPath,
		},
	).Return(
		&xmlvalidate.Result{}, nil,
	)
	s.env.OnActivity(
		activities.ValidatePREMISName,
		sessionCtx,
		&activities.ValidatePREMISParams{Path: expectedSIP.LogicalMDPath},
	).Return(
		&activities.ValidatePREMISResult{}, nil,
	)
	s.env.OnActivity(
		apis.CreateImportTaskActivityName,
		sessionCtx,
		&apis.CreateImportTaskParams{
			SIP:      expectedSIP,
			Username: preprocessingWorkflowUser,
		},
	).Return(
		&apis.CreateImportTaskResult{TaskID: apisTaskID}, nil,
	)
	s.env.OnActivity(
		apis.PollImportTaskStatusActivityName,
		sessionCtx,
		&apis.PollImportTaskStatusParams{TaskID: apisTaskID},
	).Return(
		&apis.PollImportTaskStatusResult{AnalysisResult: ar}, nil,
	)

	return expectedSIP, extractPath, apisTaskID
}

func (s *PreprocessingTestSuite) postAPISActivities(expectedSIP sip.SIP) {
	sessionCtx := mock.AnythingOfType("*context.timerCtx")
	premisFilePath := filepath.Join(expectedSIP.Path, "metadata", "premis.xml")
	expectedPIP := pips.NewFromSIP(expectedSIP)

	s.env.OnActivity(
		activities.AddPREMISObjectsName,
		sessionCtx,
		&activities.AddPREMISObjectsParams{
			SIP:            expectedSIP,
			PREMISFilePath: premisFilePath,
		},
	).Return(
		&activities.AddPREMISObjectsResult{}, nil,
	)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		sessionCtx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP structure\"",
			OutcomeDetail:  "SIP structure identified: DigitizedAIP. SIP structure matches validation criteria.",
			Failures:       nil,
		},
	).Return(
		&activities.AddPREMISEventResult{}, nil,
	)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		sessionCtx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP name\"",
			OutcomeDetail: fmt.Sprintf(
				"SIP name %q matches validation criteria.", expectedSIP.Name(),
			),
			Failures: nil,
		},
	).Return(
		&activities.AddPREMISEventResult{}, nil,
	)
	s.env.OnActivity(
		activities.AddPREMISValidationEventName,
		sessionCtx,
		&activities.AddPREMISValidationEventParams{
			SIP:            expectedSIP,
			PREMISFilePath: premisFilePath,
			Summary: premis.EventSummary{
				Type:          "validation",
				Detail:        "name=\"Validate SIP file formats\"",
				Outcome:       "valid",
				OutcomeDetail: "File format complies with specification",
			},
		},
	).Return(
		&activities.AddPREMISValidationEventResult{}, nil,
	)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		sessionCtx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP metadata\"",
			OutcomeDetail:  "Metadata validation successful",
			Failures:       nil,
		},
	).Return(
		&activities.AddPREMISEventResult{}, nil,
	)
	s.env.OnActivity(
		activities.AddPREMISAgentName,
		sessionCtx,
		&activities.AddPREMISAgentParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
		},
	).Return(
		&activities.AddPREMISAgentResult{}, nil,
	)
	s.env.OnActivity(
		activities.TransformSIPName,
		sessionCtx,
		&activities.TransformSIPParams{SIP: expectedSIP},
	).Return(
		&activities.TransformSIPResult{PIP: expectedPIP}, nil,
	)
	s.env.OnActivity(
		activities.WriteIdentifierFileName,
		sessionCtx,
		&activities.WriteIdentifierFileParams{PIP: expectedPIP},
	).Return(
		&activities.WriteIdentifierFileResult{
			Path: filepath.Join(s.sipPath, "metadata", "identifiers.json"),
		}, nil,
	)
	s.env.OnActivity(
		bagcreate.Name,
		sessionCtx,
		&bagcreate.Params{SourcePath: expectedSIP.Path},
	).Return(
		&bagcreate.Result{}, nil,
	)
}

func apisTasks(
	taskID string,
	waitMessage string,
	waitOutcome childwf.TaskOutcome,
	includePostAPIS bool,
) []*childwf.Task {
	events := append([]*childwf.Task{}, preAPISEvents...)
	events = append(events,
		&childwf.Task{
			Name:        "Submit metadata to APIS",
			Message:     fmt.Sprintf(`Submitted metadata to APIS with import task ID %q`, taskID),
			Outcome:     childwf.TaskOutcomeSuccess,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
		&childwf.Task{
			Name:        "Wait for APIS analysis",
			Message:     waitMessage,
			Outcome:     waitOutcome,
			StartedAt:   testTime,
			CompletedAt: testTime,
		},
	)
	if includePostAPIS {
		events = append(events, postAPISEvents...)
	}

	return events
}

func apisCustomMetadata(taskID, decision string) childwf.CustomMetadata {
	return childwf.CustomMetadata{
		apis.CustomMetadataKey: json.RawMessage(fmt.Sprintf(
			`{"importTaskId":%q,"decision":%q}`,
			taskID,
			decision,
		)),
	}
}

func (s *PreprocessingTestSuite) executeAsChildWithHumanReview(
	params *childwf.PreprocessingParams,
	decision childwf.DecisionResponse,
) *childwf.PreprocessingResult {
	parentWorkflow := func(
		ctx temporalsdk_workflow.Context,
		childParams *childwf.PreprocessingParams,
	) (*childwf.PreprocessingResult, error) {
		childFuture := temporalsdk_workflow.ExecuteChildWorkflow(ctx, s.workflow.Execute, childParams)

		var request childwf.DecisionRequest
		temporalsdk_workflow.GetSignalChannel(ctx, childwf.DecisionRequestSignalName).Receive(ctx, &request)
		s.Equal(
			childwf.DecisionRequest{
				Message: fmt.Sprintf(
					"APIS detected metadata conflicts for import task ID %q. Review the APIS task and choose how ingest should continue.",
					apisTaskID,
				),
				Options: []string{
					apis.DecisionOptionCancelIngest,
					apis.DecisionOptionContinueOverwrite,
					apis.DecisionOptionContinueAppend,
				},
			},
			request,
		)

		err := childFuture.SignalChildWorkflow(ctx, childwf.DecisionResponseSignalName, decision).Get(ctx, nil)
		if err != nil {
			return nil, err
		}

		var childResult childwf.PreprocessingResult
		if err := childFuture.Get(ctx, &childResult); err != nil {
			return nil, err
		}

		return &childResult, nil
	}

	s.env.ExecuteWorkflow(parentWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())

	var result childwf.PreprocessingResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)

	return &result
}

func (s *PreprocessingTestSuite) TestSuccess() {
	s.SetupTest(&config.Config{
		APIS: apis.Config{Enabled: true},
		Preprocessing: config.PreprocessingConfig{
			CheckDuplicates: true,
		},
	})
	s.writeBagitTxt(s.sipPath)

	expectedSIP, extractPath, apisTaskID := s.preAPISActivities(apisgen.AnalysisResultAlleNeu)
	s.postAPISActivities(expectedSIP)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
	)
	s.True(s.env.IsWorkflowCompleted())

	// Update the relative path to the extracted SIP path.
	relPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	var result childwf.PreprocessingResult
	err = s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:        childwf.OutcomeSuccess,
			CustomMetadata: apisCustomMetadata(apisTaskID, ""),
			RelativePath:   relPath,
			Tasks: apisTasks(
				apisTaskID,
				fmt.Sprintf(
					`APIS analysis completed for import task ID %q with result %q`,
					apisTaskID,
					apisgen.AnalysisResultAlleNeu,
				),
				childwf.TaskOutcomeSuccess,
				true,
			),
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestIdentifySIPFailure() {
	s.SetupTest(&config.Config{})

	extractPath := filepath.Join(filepath.Dir(s.sipPath), fsutil.BaseNoExt(filepath.Base(sipName)))
	sessionCtx := mock.AnythingOfType("*context.timerCtx")

	// Mock activities.
	s.env.OnActivity(
		archiveextract.Name,
		sessionCtx,
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(
		&archiveextract.Result{ExtractPath: extractPath}, nil,
	)
	s.env.OnActivity(
		activities.IdentifySIPName,
		sessionCtx,
		&activities.IdentifySIPParams{Path: extractPath},
	).Return(
		nil, fmt.Errorf("IdentifySIP: NewSIP: stat : no such file or directory"),
	)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
	)

	s.True(s.env.IsWorkflowCompleted())

	relPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	var result childwf.PreprocessingResult
	err = s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeContentError,
			RelativePath: relPath,
			Tasks: []*childwf.Task{
				{
					Name:        "Extract SIP",
					Message:     "SIP extracted",
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Identify SIP structure",
					Message: `Content error: SIP identification has failed.

Enduro could not identify the package type. Please ensure that your SIP matches one of the supported package structures.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestValidationError() {
	s.SetupTest(&config.Config{})

	extractPath := filepath.Join(filepath.Dir(s.sipPath), fsutil.BaseNoExt(filepath.Base(sipName)))
	expectedSIP := s.bornDigitalSIP(extractPath)

	// Mock activities.
	sessionCtx := mock.AnythingOfType("*context.timerCtx")
	s.env.OnActivity(
		archiveextract.Name,
		sessionCtx,
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(
		&archiveextract.Result{ExtractPath: extractPath}, nil,
	)
	s.env.OnActivity(
		activities.IdentifySIPName,
		sessionCtx,
		&activities.IdentifySIPParams{Path: extractPath},
	).Return(
		&activities.IdentifySIPResult{SIP: expectedSIP}, nil,
	)
	s.env.OnActivity(
		activities.ValidateStructureName,
		sessionCtx,
		&activities.ValidateStructureParams{SIP: expectedSIP},
	).Return(
		&activities.ValidateStructureResult{
			Failures: []string{
				"XSD folder is missing",
				"metadata.xml is missing",
			},
		},
		nil,
	)
	s.env.OnActivity(
		activities.ValidateSIPNameName,
		sessionCtx,
		&activities.ValidateSIPNameParams{SIP: expectedSIP},
	).Return(
		&activities.ValidateSIPNameResult{
			Failures: []string{"SIP name \"sip\" violates naming standard"},
		},
		nil,
	)
	s.env.OnActivity(
		activities.VerifyManifestName,
		sessionCtx,
		&activities.VerifyManifestParams{SIP: expectedSIP},
	).Return(
		&activities.VerifyManifestResult{
			ChecksumFailures: []string{
				`Checksum mismatch for "content/content/d_0000001/00000001.jp2" (expected: "827ccb0eea8a706c4c34a16891f84e7b", got: "2714364e3a0ac68e8bf9b898b31ff303")`,
			},
			ManifestFailures: []string{"Unsupported schema version: 5.1"},
			MissingFiles:     []string{"Missing file: d_0000001/00000001.jp2"},
			UnexpectedFiles:  []string{"Unexpected file: d_0000001/extra_file.txt"},
		},
		nil,
	)
	s.env.OnActivity(
		ffvalidate.Name,
		sessionCtx,
		&ffvalidate.Params{Path: expectedSIP.ContentPath},
	).Return(
		&ffvalidate.Result{Failures: []string{
			`file format fmt/11 not allowed: "content/content/d_0000001/00000010.png"`,
			`file format fmt/11 not allowed: "content/content/d_0000001/00000011.png"`,
		}},
		nil,
	)
	s.env.OnActivity(
		activities.ValidateFilesName,
		sessionCtx,
		&activities.ValidateFilesParams{SIP: expectedSIP},
	).Return(
		&activities.ValidateFilesResult{
			Failures: []string{`One or more PDF/A files are invalid`},
		},
		nil,
	)
	s.env.OnActivity(
		xmlvalidate.Name,
		sessionCtx,
		&xmlvalidate.Params{
			XMLPath: expectedSIP.ManifestPath,
			XSDPath: expectedSIP.XSDPath,
		},
	).Return(
		&xmlvalidate.Result{
			Failures: []string{
				`metadata.xml does not match expected metadata requirements`,
			},
		}, nil,
	)

	// Execute workflow.
	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
	)

	s.True(s.env.IsWorkflowCompleted())

	relPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	var result childwf.PreprocessingResult
	err = s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeContentError,
			RelativePath: relPath,
			Tasks: []*childwf.Task{
				{
					Name:        "Extract SIP",
					Message:     "SIP extracted",
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name:        "Identify SIP structure",
					Message:     "SIP structure identified: BornDigitalSIP",
					Outcome:     childwf.TaskOutcomeSuccess,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Validate SIP structure",
					Message: `Content error: SIP structure validation has failed.

- XSD folder is missing
- metadata.xml is missing

Please review the SIP and ensure that its structure matches the BornDigitalSIP specifications.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Validate SIP name",
					Message: `Content error: SIP name validation has failed.

The name used for the package does not match the expected convention for the "BornDigitalSIP" type.

- SIP name "sip" violates naming standard

Please review the naming conventions specified for this type of SIP.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Verify SIP manifest",
					Message: `Content error: "metadata.xml" manifest could not be verified against the contents of the SIP.

- Unsupported schema version: 5.1
- Missing file: d_0000001/00000001.jp2
- Unexpected file: d_0000001/extra_file.txt

Please review the SIP and ensure that its contents match those listed in the metadata manifest.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Verify SIP checksums",
					Message: `Content error: SIP checksums do not match file contents.

- Checksum mismatch for "content/content/d_0000001/00000001.jp2" (expected: "827ccb0eea8a706c4c34a16891f84e7b", got: "2714364e3a0ac68e8bf9b898b31ff303")

Please review the SIP and ensure that the metadata checksums match those of the files.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Check for disallowed file formats",
					Message: `Content error: file format check has failed.

One or more file formats are not allowed:

- file format fmt/11 not allowed: "content/content/d_0000001/00000010.png"
- file format fmt/11 not allowed: "content/content/d_0000001/00000011.png"

Please review the SIP and remove or replace all disallowed file formats.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Validate SIP file formats",
					Message: `Content error: file format validation has failed.

- One or more PDF/A files are invalid

Please ensure all files are well-formed.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Validate SIP metadata",
					Message: `Content error: metadata validation has failed.

- metadata.xml does not match expected metadata requirements

Please ensure all metadata files are present and well-formed.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestSystemError() {
	s.SetupTest(&config.Config{})

	// Mock activities.
	s.env.OnActivity(
		archiveextract.Name,
		mock.AnythingOfType("*context.timerCtx"),
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(
		nil, errors.New("Not a file"),
	)

	// Execute workflow.
	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
	)

	s.True(s.env.IsWorkflowCompleted())

	var result childwf.PreprocessingResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeSystemError,
			RelativePath: relPath,
			Tasks: []*childwf.Task{
				{
					Name: "Extract SIP",
					Message: fmt.Sprintf(
						`System error: SIP extraction has failed.

%q could not be successfully extracted. Please try again, or ask a system administrator to investigate.`,
						filepath.Base(relPath),
					),
					Outcome:     childwf.TaskOutcomeSystemFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestExtractionError() {
	cfg := &config.Config{}
	s.SetupTest(cfg)

	sessionCtx := mock.AnythingOfType("*context.timerCtx")
	extractPath := filepath.Join(cfg.Preprocessing.SharedPath, "extract-123456")

	// Mock activities.
	s.env.OnActivity(
		archiveextract.Name,
		sessionCtx,
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(
		&archiveextract.Result{
			ExtractPath: extractPath,
		},
		nil,
	)

	s.env.OnActivity(
		activities.IdentifySIPName,
		sessionCtx,
		&activities.IdentifySIPParams{Path: extractPath},
	).Return(
		nil,
		fmt.Errorf("IdentifySIP: NewSIP: stat : no such file or directory"),
	)

	// Execute workflow.
	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
	)

	s.True(s.env.IsWorkflowCompleted())

	var result childwf.PreprocessingResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeContentError,
			RelativePath: relPath,
			Tasks: []*childwf.Task{
				{
					Name: "Extract SIP",
					Message: fmt.Sprintf(
						`Content error: SIP extraction has failed.

The extracted SIP is missing the top-level %q folder.

Please ensure that the SIP is well-formed and try again.`,
						fsutil.BaseNoExt(sipName),
					),
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
				{
					Name: "Identify SIP structure",
					Message: `Content error: SIP identification has failed.

Enduro could not identify the package type. Please ensure that your SIP matches one of the supported package structures.`,
					Outcome:     childwf.TaskOutcomeValidationFailure,
					StartedAt:   testTime,
					CompletedAt: testTime,
				},
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestHumanReviewContinueAndOverwrite() {
	s.SetupTest(&config.Config{
		APIS: apis.Config{Enabled: true},
		Preprocessing: config.PreprocessingConfig{
			CheckDuplicates: true,
		},
	})
	s.writeBagitTxt(s.sipPath)

	expectedSIP, extractPath, apisTaskID := s.preAPISActivities(apisgen.AnalysisResultKonflikte)
	s.postAPISActivities(expectedSIP)

	result := s.executeAsChildWithHumanReview(
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
		childwf.DecisionResponse{
			Option: apis.DecisionOptionContinueOverwrite,
		},
	)

	updatedRelPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:        childwf.OutcomeSuccess,
			CustomMetadata: apisCustomMetadata(apisTaskID, apis.DecisionOptionContinueOverwrite),
			RelativePath:   updatedRelPath,
			Tasks: apisTasks(
				apisTaskID,
				fmt.Sprintf(
					"APIS detected metadata conflicts for import task ID %q but ingest was continued with user decision %q.",
					apisTaskID,
					apis.DecisionOptionContinueOverwrite,
				),
				childwf.TaskOutcomeSuccess,
				true,
			),
		},
		result,
	)
}

func (s *PreprocessingTestSuite) TestHumanReviewContinueAndAppend() {
	s.SetupTest(&config.Config{
		APIS: apis.Config{Enabled: true},
		Preprocessing: config.PreprocessingConfig{
			CheckDuplicates: true,
		},
	})
	s.writeBagitTxt(s.sipPath)

	expectedSIP, extractPath, apisTaskID := s.preAPISActivities(apisgen.AnalysisResultKonflikte)
	s.postAPISActivities(expectedSIP)

	result := s.executeAsChildWithHumanReview(
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
		childwf.DecisionResponse{
			Option: apis.DecisionOptionContinueAppend,
		},
	)

	updatedRelPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:        childwf.OutcomeSuccess,
			CustomMetadata: apisCustomMetadata(apisTaskID, apis.DecisionOptionContinueAppend),
			RelativePath:   updatedRelPath,
			Tasks: apisTasks(
				apisTaskID,
				fmt.Sprintf(
					"APIS detected metadata conflicts for import task ID %q but ingest was continued with user decision %q.",
					apisTaskID,
					apis.DecisionOptionContinueAppend,
				),
				childwf.TaskOutcomeSuccess,
				true,
			),
		},
		result,
	)
}

func (s *PreprocessingTestSuite) TestHumanReviewCancelIngest() {
	s.SetupTest(&config.Config{
		APIS: apis.Config{Enabled: true},
		Preprocessing: config.PreprocessingConfig{
			CheckDuplicates: true,
		},
	})
	s.writeBagitTxt(s.sipPath)

	_, extractPath, apisTaskID := s.preAPISActivities(apisgen.AnalysisResultKonflikte)

	result := s.executeAsChildWithHumanReview(
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
		childwf.DecisionResponse{
			Option: apis.DecisionOptionCancelIngest,
		},
	)

	updatedRelPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeContentError,
			RelativePath: updatedRelPath,
			Tasks: apisTasks(
				apisTaskID,
				fmt.Sprintf(
					`Content error: ingest was canceled after APIS metadata conflict review.

APIS detected metadata conflicts for import task ID %q and ingest was canceled by user decision.`,
					apisTaskID,
				),
				childwf.TaskOutcomeValidationFailure,
				false,
			),
		},
		result,
	)
}

func (s *PreprocessingTestSuite) TestHumanReviewInvalidOption() {
	s.SetupTest(&config.Config{
		APIS: apis.Config{Enabled: true},
		Preprocessing: config.PreprocessingConfig{
			CheckDuplicates: true,
		},
	})
	s.writeBagitTxt(s.sipPath)

	_, extractPath, apisTaskID := s.preAPISActivities(apisgen.AnalysisResultKonflikte)

	result := s.executeAsChildWithHumanReview(
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPID:        sipUUID,
			SIPName:      sipName,
		},
		childwf.DecisionResponse{
			Option: "Unexpected option",
		},
	)

	updatedRelPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeSystemError,
			RelativePath: updatedRelPath,
			Tasks: apisTasks(
				apisTaskID,
				`System error: submission to APIS has failed.

Received unsupported user decision "Unexpected option" while resolving APIS metadata conflicts. Please ask a system administrator to investigate.`,
				childwf.TaskOutcomeSystemFailure,
				false,
			),
		},
		result,
	)
}
