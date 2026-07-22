package activities_test

import (
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func digitizedAIP(t *testing.T, content ...fs.PathOp) sip.SIP {
	t.Helper()

	path := fs.NewDir(t, "extract-",
		fs.WithDir("7537ab2c-4e6b-4820-95bf-bd2c577351c3",
			fs.WithDir("additional",
				fs.WithFile("7537ab2c-4e6b-4820-95bf-bd2c577351c3-premis.xml", ""),
				fs.WithFile("UpdatedAreldaMetadata.xml", ""),
			),
			fs.WithDir("content",
				fs.WithDir("content", content...),
				fs.WithDir("header",
					fs.WithDir("old",
						fs.WithDir("SIP",
							fs.WithFile("metadata.xml", ""),
						),
					),
					fs.WithDir("xsd",
						fs.WithFile("arelda.xsd", ""),
					),
				),
			),
		),
	).Join("7537ab2c-4e6b-4820-95bf-bd2c577351c3")

	return testSIP(t, path)
}

func digitizedSIP(t *testing.T, hasExtraDossier bool) sip.SIP {
	t.Helper()

	path := fs.NewDir(t, "extract-",
		fs.WithDir("SIP_202200915_dept",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
				func() fs.PathOp {
					if hasExtraDossier {
						return fs.WithDir("d_0000002",
							fs.WithFile("00000002.jp2", ""),
							fs.WithFile("00000002_PREMIS.xml", ""),
							fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
						)
					}

					// Don't modify path (no-op).
					return func(path fs.Path) error { return nil }
				}(),
			),
			fs.WithDir("header",
				fs.WithFile("metadata.xml", ""),
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", ""),
				),
			),
		),
	).Join("SIP_202200915_dept")

	return testSIP(t, path)
}

// extraTLDSIP returns a born-digital SIP with an extra top-level directory.
func extraTLDSIP(t *testing.T) sip.SIP {
	t.Helper()

	path := fs.NewDir(t, "extract-",
		fs.WithDir("SIP_202200915_dept",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
				),
				fs.WithFile("00000002.jp2", ""),
				fs.WithFile("00000002_PREMIS.xml", ""),
			),
			fs.WithDir("header",
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", ""),
				),
				fs.WithFile("metadata.xml", ""),
			),
			fs.WithDir("extra",
				fs.WithFile("data.txt", ""),
			),
		),
	).Join("SIP_202200915_dept")

	return testSIP(t, path)
}

func badNamesSIP(t *testing.T) sip.SIP {
	t.Helper()

	path := fs.NewDir(t, "extract-",
		fs.WithDir("SIP_20251015_Vecteur_987654",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001$.jp2", ""),
					fs.WithFile("00000001$_PREMIS.xml", ""),
				),
			),
			fs.WithDir("header",
				fs.WithFile("content!.txt", ""),
				fs.WithFile("metadata.xml", ""),
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", ""),
				),
			),
		),
	).Join("SIP_20251015_Vecteur_987654")

	return testSIP(t, path)
}

func TestValidateStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  activities.ValidateStructureParams
		want    activities.ValidateStructureResult
		wantErr string
	}{
		{
			name: "Returns only one failure when SIP is empty",
			params: activities.ValidateStructureParams{
				SIP: testSIP(t, fs.NewDir(t, "extract-",
					fs.WithDir("AIP-1234"),
				).Join("AIP-1234")),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"The SIP is empty",
				},
			},
		},
		{
			name: "Returns multiple failures when a SIP has a single file",
			params: activities.ValidateStructureParams{
				SIP: testSIP(t, fs.NewDir(t, "extract-",
					fs.WithDir("AIP-1234",
						fs.WithFile("file.txt", ""),
					),
				).Join("AIP-1234")),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"Content folder is missing",
					"XSD folder is missing",
					"metadata.xml is missing",
				},
			},
		},
		{
			name:   "Returns a failure when a SIP has an extra top-level directory",
			params: activities.ValidateStructureParams{SIP: extraTLDSIP(t)},
			want: activities.ValidateStructureResult{
				Failures: []string{`Unexpected directory: "extra"`},
			},
		},
		{
			name: "Returns failures when a SIP is missing files or directories",
			params: activities.ValidateStructureParams{
				SIP: testSIP(t, fs.NewDir(t, "extract-",
					fs.WithDir("SIP_202200915_dept",
						fs.WithDir("header",
							fs.WithFile("metadata.xml", ""),
						),
					),
				).Join("SIP_202200915_dept")),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"Content folder is missing",
					"XSD folder is missing",
				},
			},
		},
		{
			name:   "Validates a digitized SIP",
			params: activities.ValidateStructureParams{SIP: digitizedSIP(t, false)},
		},
		{
			name:   "Returns a failure when a digitized SIP has more than one dossier",
			params: activities.ValidateStructureParams{SIP: digitizedSIP(t, true)},
			want: activities.ValidateStructureResult{
				Failures: []string{"More than one dossier in the content directory"},
			},
		},
		{
			name:   "Returns a failure when a digitized SIP has bad file or directory names",
			params: activities.ValidateStructureParams{SIP: badNamesSIP(t)},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"Name \"content/d_0000001/00000001$.jp2\" contains invalid character(s)",
					"Name \"content/d_0000001/00000001$_PREMIS.xml\" contains invalid character(s)",
					"Name \"header/content!.txt\" contains invalid character(s)",
				},
			},
		},
		{
			name: "Validates a digitized AIP",
			params: activities.ValidateStructureParams{
				SIP: digitizedAIP(t,
					fs.WithDir("d_0000001",
						fs.WithFile("00000001.jp2", ""),
						fs.WithFile("00000001_PREMIS.xml", ""),
						fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
					),
				),
			},
		},
		{
			name: "Validates a digitized AIP without dossiers",
			params: activities.ValidateStructureParams{
				SIP: digitizedAIP(t,
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			},
		},
		{
			name: "Returns a failure when a digitized AIP has an empty directory",
			params: activities.ValidateStructureParams{
				SIP: digitizedAIP(t, fs.WithDir("d_0000001")),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"An empty directory has been found - content/content/d_0000001",
					"Please remove the empty directories and update the metadata manifest accordingly",
				},
			},
		},
		{
			name: "Returns failures when a digitized AIP is missing files and directories",
			params: activities.ValidateStructureParams{
				SIP: testSIP(t,
					fs.NewDir(t, "extract-",
						fs.WithDir("AIP-1234",
							fs.WithDir("additional",
								fs.WithFile("content.txt", ""),
							),
							fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
						),
					).Join("AIP-1234"),
				),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"Content folder is missing",
					"XSD folder is missing",
					"metadata.xml is missing",
					"UpdatedAreldaMetadata.xml is missing",
					"AIP-1234-premis.xml is missing",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewValidateStructure().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateStructureName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateStructureName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var result activities.ValidateStructureResult
			_ = enc.Get(&result)
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
