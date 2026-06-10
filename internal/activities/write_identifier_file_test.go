package activities_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/pips"
)

func TestWriteIdentifierFile(t *testing.T) {
	t.Parallel()

	pipDigitizedSIP := pips.New(
		fs.NewDir(t, "",
			fs.WithDir("Test_Digitized_SIP",
				fs.WithDir("metadata"),
				fs.WithDir("objects",
					fs.WithDir("Test_Digitized_SIP",
						fs.WithDir("header",
							fs.WithFile("metadata.xml", digitizedSIPMetadata),
						),
					),
				),
			),
		).Join("Test_Digitized_SIP"),
		enums.SIPTypeDigitizedSIP,
	)

	pipNoManifest := pips.New(fs.NewDir(t, "").Path(), enums.SIPTypeBornDigitalSIP)

	pipEmptyManifest := pips.New(
		fs.NewDir(t, "",
			fs.WithDir("Test_digitized_SIP",
				fs.WithDir("metadata"),
				fs.WithDir("objects",
					fs.WithDir("Test_digitized_SIP",
						fs.WithDir("header",
							fs.WithFile("metadata.xml", ""),
						),
					),
				),
			),
		).Join("Test_digitized_SIP"),
		enums.SIPTypeDigitizedSIP,
	)

	pipReadOnly := pips.New(
		fs.NewDir(t, "",
			fs.WithDir("Test_Digitized_SIP",
				fs.WithDir("metadata", fs.WithMode(0o400)),
				fs.WithDir("objects",
					fs.WithDir("Test_Digitized_SIP",
						fs.WithDir("header",
							fs.WithFile("metadata.xml", digitizedSIPMetadata),
						),
					),
				),
			),
		).Join("Test_Digitized_SIP"),
		enums.SIPTypeDigitizedSIP,
	)

	tests := []struct {
		name     string
		params   activities.WriteIdentifierFileParams
		wantJSON string
		wantErr  string
	}{
		{
			name: "Writes a digitized SIP identifier file",
			params: activities.WriteIdentifierFileParams{
				PIP: pipDigitizedSIP,
			},
			wantJSON: `[
    {
        "file": "metadata/Prozess_Digitalisierung_PREMIS.xml",
        "identifiers": [
            {
                "identifier": "_cQ6sm5CChWVqtqmrWvne0W",
                "identifierType": "local"
            }
        ]
    },
    {
        "file": "objects/Test_Digitized_SIP/content/d_0000001/00000001.jp2",
        "identifiers": [
            {
                "identifier": "_zodSTSD0nv05CpOp6JoV3X",
                "identifierType": "local"
            }
        ]
    },
    {
        "file": "objects/Test_Digitized_SIP/content/d_0000001/00000001_PREMIS.xml",
        "identifiers": [
            {
                "identifier": "_WuDmXAs5UDwKTGVLsCcZxa",
                "identifierType": "local"
            }
        ]
    },
    {
        "file": "objects/Test_Digitized_SIP/content/d_0000001/00000002.jp2",
        "identifiers": [
            {
                "identifier": "_rlPKJX9ZcAl4ooc4IfoIkM",
                "identifierType": "local"
            }
        ]
    },
    {
        "file": "objects/Test_Digitized_SIP/content/d_0000001/00000002_PREMIS.xml",
        "identifiers": [
            {
                "identifier": "_Ohk77y2DJa82RXqsWG4S90",
                "identifierType": "local"
            }
        ]
    }
]`,
		},
		{
			name: "Errors when manifest is not readable",
			params: activities.WriteIdentifierFileParams{
				PIP: pipNoManifest,
			},
			wantErr: fmt.Sprintf(
				"write identifier file: open manifest: open %s: no such file or directory",
				pipNoManifest.ManifestPath,
			),
		},
		{
			name: "Errors when manifest is invalid",
			params: activities.WriteIdentifierFileParams{
				PIP: pipEmptyManifest,
			},
			wantErr: "write identifier file: get manifest identifiers: no files in manifest",
		},
		{
			name: "Errors when metadata path is not writable",
			params: activities.WriteIdentifierFileParams{
				PIP: pipReadOnly,
			},
			wantErr: fmt.Sprintf(
				"write identifier file: write identifiers.json: open %s: permission denied",
				pipReadOnly.Path+"/metadata/identifiers.json",
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewWriteIdentifierFile().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.WriteIdentifierFileName},
			)

			future, err := env.ExecuteActivity(activities.WriteIdentifierFileName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.Error(
						t,
						err,
						"activity error (type: write-identifier-file, scheduledEventID: 0, startedEventID: 0, identity: ): "+tt.wantErr,
					)
				}

				return
			}
			assert.NilError(t, err)

			var res activities.WriteIdentifierFileResult
			future.Get(&res)
			p := filepath.Join(tt.params.PIP.Path, "metadata", "identifiers.json")
			assert.DeepEqual(t, res, activities.WriteIdentifierFileResult{Path: p})

			b, err := os.ReadFile(p)
			assert.NilError(t, err)
			assert.Equal(t, string(b), tt.wantJSON)
		})
	}
}
