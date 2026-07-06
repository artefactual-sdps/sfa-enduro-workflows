package activities_test

import (
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func TestIdentifySIP(t *testing.T) {
	t.Parallel()

	path := fs.NewDir(t, "",
		fs.WithDir("Digitized-AIP",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			),
			fs.WithDir("additional"),
		),
	).Join("Digitized-AIP")

	tests := []struct {
		name    string
		params  activities.IdentifySIPParams
		result  activities.IdentifySIPResult
		wantErr string
	}{
		{
			name:   "Identifies a SIP",
			params: activities.IdentifySIPParams{Path: path},
			result: activities.IdentifySIPResult{
				SIP: sip.SIP{
					Type:                enums.SIPTypeDigitizedAIP,
					Path:                path,
					ContentPath:         filepath.Join(path, "content", "content"),
					LogicalMDPath:       filepath.Join(path, "additional", "Digitized-AIP-premis.xml"),
					ManifestPath:        filepath.Join(path, "additional", "UpdatedAreldaMetadata.xml"),
					MetadataPath:        filepath.Join(path, "content", "header", "old", "SIP", "metadata.xml"),
					UpdatedAreldaMDPath: filepath.Join(path, "additional", "UpdatedAreldaMetadata.xml"),
					XSDPath:             filepath.Join(path, "content", "header", "xsd", "arelda.xsd"),
					TopLevelPaths:       []string{filepath.Join(path, "content"), filepath.Join(path, "additional")},
				},
			},
		},
		{
			name:    "Fails to identify a non existing path",
			wantErr: "IdentifySIP: SIP: New: stat : no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewIdentifySIP().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.IdentifySIPName},
			)

			var res activities.IdentifySIPResult
			future, err := env.ExecuteActivity(activities.IdentifySIPName, tt.params)
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
		})
	}
}
