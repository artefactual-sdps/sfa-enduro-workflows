package activities_test

import (
	pseudorand "math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	fake_fvalidate "github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate/fake"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const premisObjectContent = `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>uuid</premis:objectIdentifierType>
      <premis:objectIdentifierValue>c74a85b7-919b-409e-8209-9c7ebe0e7945</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/content/file.json</premis:originalName>
  </premis:object>
</premis:premis>
`

func TestAddPREMISValidationEvent(t *testing.T) {
	t.Parallel()

	// Define PREMIS event summary to use in all tests.
	eventSummary := premis.EventSummary{
		Type:          "validation",
		Detail:        "name=\"Validate SIP file formats\"",
		Outcome:       "valid",
		OutcomeDetail: "File format complies with specification",
	}

	// Create test directory, and corresponding SIP object, with non-PREMIS XML.
	badXMLDir := fs.NewDir(t, "",
		fs.WithFile("premis.xml", "<xml></xml>"),
	)

	badXMLFilePath := filepath.Join(badXMLDir.Path(), "premis.xml")

	badXMLSIP := sip.SIP{
		Type:        enums.SIPTypeBornDigitalAIP,
		ContentPath: badXMLDir.Path(),
	}

	// Create test directory, and corresponding SIP object, with empty PREMIS file.
	emptyPREMISTestDir := fs.NewDir(t, "",
		fs.WithFile("premis.xml", premis.EmptyXML),
		fs.WithFile("file.json", "{}"),
	)

	emptyPREMISPath := filepath.Join(emptyPREMISTestDir.Path(), "premis.xml")

	testSIP := sip.SIP{
		Type:        enums.SIPTypeBornDigitalAIP,
		ContentPath: emptyPREMISTestDir.Path(),
	}

	// Create test directory, and corresponding SIP object, with populated PREMIS file.
	normalTestDir := fs.NewDir(t, "",
		fs.WithDir("test_transfer",
			fs.WithFile("file.json", "{}"),
			fs.WithFile("premis.xml", premisObjectContent),
		),
	)

	normalPREMISPath := filepath.Join(normalTestDir.Path(), "test_transfer", "premis.xml")

	normalTestSIP := sip.SIP{
		Type:        enums.SIPTypeBornDigitalAIP,
		ContentPath: filepath.Join(normalTestDir.Path(), "test_transfer"),
	}

	// Creation of PREMIS file in non-existing directory (for execution expected to fail).
	ContentNonExistent := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)

	PREMISFilePathNonExistent := ContentNonExistent.Join("metadata", "premis.xml")

	ContentNonExistent.Remove()

	tests := []struct {
		name        string
		params      activities.AddPREMISValidationEventParams
		result      activities.AddPREMISValidationEventResult
		expectVld   func(*fake_fvalidate.MockValidatorMockRecorder)
		wantContent []byte
		wantErr     string
	}{
		{
			name: "Add PREMIS event and agent",
			params: activities.AddPREMISValidationEventParams{
				SIP:            normalTestSIP,
				PREMISFilePath: normalPREMISPath,
				Summary:        eventSummary,
			},
			result: activities.AddPREMISValidationEventResult{},
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.FormatIDs().Return([]string{"fmt/354", "fmt/817"})
				m.PREMISAgent().Return(premis.Agent{
					Type:    "software",
					Name:    "veraPDF v1.2.3",
					IdType:  "url",
					IdValue: "https://verapdf.org",
				})
			},
			wantContent: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>uuid</premis:objectIdentifierType>
      <premis:objectIdentifierValue>c74a85b7-919b-409e-8209-9c7ebe0e7945</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/content/file.json</premis:originalName>
    <premis:linkingEventIdentifier>
      <premis:linkingEventIdentifierType>UUID</premis:linkingEventIdentifierType>
      <premis:linkingEventIdentifierValue>52fdfc07-2182-454f-963f-5f0f9a621d72</premis:linkingEventIdentifierValue>
    </premis:linkingEventIdentifier>
  </premis:object>
  <premis:event>
    <premis:eventIdentifier>
      <premis:eventIdentifierType>UUID</premis:eventIdentifierType>
      <premis:eventIdentifierValue>52fdfc07-2182-454f-963f-5f0f9a621d72</premis:eventIdentifierValue>
    </premis:eventIdentifier>
    <premis:eventType>validation</premis:eventType>
    <premis:eventDateTime>2025-06-06T09:57:16Z</premis:eventDateTime>
    <premis:eventDetailInformation>
      <premis:eventDetail>name=&quot;Validate SIP file formats&quot;</premis:eventDetail>
    </premis:eventDetailInformation>
    <premis:eventOutcomeInformation>
      <premis:eventOutcome>valid</premis:eventOutcome>
      <premis:eventOutcomeDetail>
        <premis:eventOutcomeDetailNote>File format complies with specification</premis:eventOutcomeDetailNote>
      </premis:eventOutcomeDetail>
    </premis:eventOutcomeInformation>
    <premis:linkingAgentIdentifier>
      <premis:linkingAgentIdentifierType valueURI="http://id.loc.gov/vocabulary/identifiers/local">url</premis:linkingAgentIdentifierType>
      <premis:linkingAgentIdentifierValue>https://verapdf.org</premis:linkingAgentIdentifierValue>
    </premis:linkingAgentIdentifier>
  </premis:event>
  <premis:agent>
    <premis:agentIdentifier>
      <premis:agentIdentifierType valueURI="http://id.loc.gov/vocabulary/identifiers/local">url</premis:agentIdentifierType>
      <premis:agentIdentifierValue>https://verapdf.org</premis:agentIdentifierValue>
    </premis:agentIdentifier>
    <premis:agentName>veraPDF v1.2.3</premis:agentName>
    <premis:agentType>software</premis:agentType>
  </premis:agent>
</premis:premis>
`),
		},
		{
			name: "Error adding PREMIS event to bad XML",
			params: activities.AddPREMISValidationEventParams{
				SIP:            badXMLSIP,
				PREMISFilePath: badXMLFilePath,
				Summary:        eventSummary,
			},
			result:  activities.AddPREMISValidationEventResult{},
			wantErr: "no root premis element found in document",
		},
		{
			name: "Error adding PREMIS event for PREMIS object not yet in the XML",
			params: activities.AddPREMISValidationEventParams{
				SIP:            testSIP,
				PREMISFilePath: emptyPREMISPath,
				Summary:        eventSummary,
			},
			result: activities.AddPREMISValidationEventResult{},
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.FormatIDs().Return([]string{"fmt/354", "fmt/817"})
				m.PREMISAgent().Return(premis.Agent{
					Type:    "software",
					Name:    "veraPDF v1.2.3",
					IdType:  "url",
					IdValue: "https://verapdf.org",
				})
			},
			wantErr: "element not found",
		},
		{
			name: "Error adding PREMIS event to bad PREMIS file path",
			params: activities.AddPREMISValidationEventParams{
				SIP:            testSIP,
				PREMISFilePath: PREMISFilePathNonExistent,
				Summary:        eventSummary,
			},
			result:  activities.AddPREMISValidationEventResult{},
			wantErr: "PREMIS file path does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			mockVdr := fake_fvalidate.NewMockValidator(ctrl)
			if tt.expectVld != nil {
				tt.expectVld(mockVdr.EXPECT())
			}

			act := activities.NewAddPREMISValidationEvent(
				clockwork.NewFakeClockAt(time.Date(2025, 6, 6, 9, 57, 16, 0, time.UTC)),
				pseudorand.New(pseudorand.NewSource(1)), // #nosec G404
				mockVdr,
			)

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				act.Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISValidationEventName},
			)

			var res activities.AddPREMISValidationEventResult
			future, err := env.ExecuteActivity(activities.AddPREMISValidationEventName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			assert.NilError(t, future.Get(&res))
			assert.DeepEqual(t, res, tt.result)

			if tt.wantContent != nil {
				b, err := os.ReadFile(tt.params.PREMISFilePath)
				assert.NilError(t, err)
				assert.DeepEqual(t, b, tt.wantContent)
			}
		})
	}
}
