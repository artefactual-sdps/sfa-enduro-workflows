package activities_test

import (
	pseudorand "math/rand"
	"os"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
)

func TestAddPREMISObjects(t *testing.T) {
	t.Parallel()

	// Normally populated files (for execution expected to work).
	contentFilesNormal := fs.NewDir(t, "",
		fs.WithDir("digitized_Vecteur_SIP",
			fs.WithDir("header",
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", ""),
				),
				fs.WithFile("metadata.xml", digitizedSIPMetadata),
			),
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
					fs.WithFile("00000002.jp2", ""),
					fs.WithFile("00000002_PREMIS.xml", ""),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			),
		),
	)

	premisFilePathNormal := contentFilesNormal.Join("digitized_Vecteur_SIP", "metadata", "premis.xml")

	tests := []struct {
		name       string
		params     activities.AddPREMISObjectsParams
		result     activities.AddPREMISObjectsResult
		wantPREMIS string
		wantErr    string
	}{
		{
			name: "Add PREMIS objects for normal content",
			params: activities.AddPREMISObjectsParams{
				SIP:            testSIP(t, contentFilesNormal.Join("digitized_Vecteur_SIP")),
				PREMISFilePath: premisFilePathNormal,
			},
			result: activities.AddPREMISObjectsResult{},
			wantPREMIS: `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>52fdfc07-2182-454f-963f-5f0f9a621d72</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/digitized_Vecteur_SIP/content/d_0000001/00000001.jp2</premis:originalName>
  </premis:object>
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>9566c74d-1003-4c4d-bbbb-0407d1e2c649</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/digitized_Vecteur_SIP/content/d_0000001/00000001_PREMIS.xml</premis:originalName>
  </premis:object>
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>81855ad8-681d-4d86-91e9-1e00167939cb</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/digitized_Vecteur_SIP/content/d_0000001/00000002.jp2</premis:originalName>
  </premis:object>
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>6694d2c4-22ac-4208-a007-2939487f6999</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/digitized_Vecteur_SIP/content/d_0000001/00000002_PREMIS.xml</premis:originalName>
  </premis:object>
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>eb9d18a4-4784-445d-87f3-c67cf22746e9</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/metadata/Prozess_Digitalisierung_PREMIS.xml</premis:originalName>
  </premis:object>
</premis:premis>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			rng := pseudorand.New(pseudorand.NewSource(1)) // #nosec G404
			env.RegisterActivityWithOptions(
				activities.NewAddPREMISObjects(rng).Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISObjectsName},
			)

			var res activities.AddPREMISObjectsResult
			future, err := env.ExecuteActivity(activities.AddPREMISObjectsName, tt.params)
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

			b, err := os.ReadFile(tt.params.PREMISFilePath)
			assert.NilError(t, err)
			assert.Equal(t, string(b), tt.wantPREMIS)
		})
	}
}
