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
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func TestTransformSIP(t *testing.T) {
	t.Parallel()

	var (
		dmode = os.FileMode(0o700)
		fmode = os.FileMode(0o600)
	)

	digitizedAIPPath := fs.NewDir(t, "",
		fs.WithDir("Vecteur_Digitized_AIP",
			fs.WithDir("additional",
				fs.WithFile("UpdatedAreldaMetadata.xml", ""),
				fs.WithFile("Vecteur_Digitized_AIP-premis.xml", ""),
			),
			fs.WithDir("content",
				fs.WithDir("content",
					fs.WithDir("d_0000001",
						fs.WithFile("00000001.jp2", ""),
						fs.WithFile("00000001_PREMIS.xml", ""),
						fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
					),
				),
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
	).Join("Vecteur_Digitized_AIP")

	digitizedSIPPath := fs.NewDir(t, "Vecteur_Digitized_SIP",
		fs.WithDir("content",
			fs.WithDir("d_0000001",
				fs.WithFile("00000001.jp2", ""),
				fs.WithFile("00000001_PREMIS.xml", ""),
				fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
			),
		),
		fs.WithDir("header",
			fs.WithFile("metadata.xml", ""),
			fs.WithDir("xsd",
				fs.WithFile("arelda.xsd", ""),
			),
		),
	).Path()

	digitizedAIP, err := sip.New(digitizedAIPPath)
	assert.NilError(t, err)
	digitizedSIP, err := sip.New(digitizedSIPPath)
	assert.NilError(t, err)

	expectedDigitizedAIP := fs.Expected(t,
		fs.WithDir("objects", fs.WithMode(dmode),
			fs.WithDir(filepath.Base(digitizedAIPPath), fs.WithMode(dmode),
				fs.WithDir("content", fs.WithMode(dmode),
					fs.WithDir("d_0000001", fs.WithMode(dmode),
						fs.WithFile("00000001.jp2", "", fs.WithMode(fmode)),
						fs.WithFile("00000001_PREMIS.xml", "", fs.WithMode(fmode)),
					),
				),
				fs.WithDir("header", fs.WithMode(dmode),
					fs.WithFile("metadata.xml", "", fs.WithMode(fmode)),
				),
			),
		),
		fs.WithDir("metadata", fs.WithMode(dmode),
			fs.WithFile("UpdatedAreldaMetadata.xml", "", fs.WithMode(fmode)),
			fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", "", fs.WithMode(fmode)),
			fs.WithFile("Vecteur_Digitized_AIP-premis.xml", "", fs.WithMode(fmode)),
		),
	)

	expectedDigitizedSIP := fs.Expected(t,
		fs.WithDir("objects", fs.WithMode(dmode),
			fs.WithDir(filepath.Base(digitizedSIPPath), fs.WithMode(dmode),
				fs.WithDir("content", fs.WithMode(dmode),
					fs.WithDir("d_0000001", fs.WithMode(dmode),
						fs.WithFile("00000001.jp2", "", fs.WithMode(fmode)),
						fs.WithFile("00000001_PREMIS.xml", "", fs.WithMode(fmode)),
					),
				),
				fs.WithDir("header", fs.WithMode(dmode),
					fs.WithFile("metadata.xml", "", fs.WithMode(fmode)),
				),
			),
		),
		fs.WithDir("metadata", fs.WithMode(dmode),
			fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", "", fs.WithMode(fmode)),
		),
	)

	missingMetadataSIP, err := sip.New(fs.NewDir(t, "MissingMD_Vecteur_SIP",
		fs.WithDir("content",
			fs.WithDir("d_0000001",
				fs.WithFile("00000001.jp2", ""),
				fs.WithFile("00000001_PREMIS.xml", ""),
				fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
			),
		),
	).Path())
	assert.NilError(t, err)

	missingContentSIP, err := sip.New(fs.NewDir(t, "Missing_Content_SIP",
		fs.WithDir("header",
			fs.WithFile("metadata.xml", ""),
		),
	).Path())
	assert.NilError(t, err)

	tests := []struct {
		name    string
		params  activities.TransformSIPParams
		wantSIP fs.Manifest
		wantErr string
	}{
		{
			name:    "Transforms a digitized AIP",
			params:  activities.TransformSIPParams{SIP: digitizedAIP},
			wantSIP: expectedDigitizedAIP,
		},
		{
			name:    "Transforms a digitized SIP",
			params:  activities.TransformSIPParams{SIP: digitizedSIP},
			wantSIP: expectedDigitizedSIP,
		},
		{
			name:   "Fails when the metadata file is missing",
			params: activities.TransformSIPParams{SIP: missingMetadataSIP},
			wantErr: fmt.Sprintf(
				"rename %s/header/metadata.xml %s/objects/%s/header/metadata.xml: no such file or directory",
				missingMetadataSIP.Path,
				missingMetadataSIP.Path,
				filepath.Base(missingMetadataSIP.Path),
			),
		},
		{
			name:   "Fails when the content directory is missing",
			params: activities.TransformSIPParams{SIP: missingContentSIP},
			wantErr: fmt.Sprintf(
				"rename %s/content %s/objects/%s/content: no such file or directory (type: LinkError, retryable: true): no such file or directory",
				missingContentSIP.Path,
				missingContentSIP.Path,
				filepath.Base(missingContentSIP.Path),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewTransformSIP().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.TransformSIPName},
			)

			_, err := env.ExecuteActivity(activities.TransformSIPName, tt.params)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}

			assert.NilError(t, err)
			assert.Assert(t, fs.Equal(tt.params.SIP.Path, tt.wantSIP))
		})
	}
}
