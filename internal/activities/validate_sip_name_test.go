package activities_test

import (
	"fmt"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func TestValidateSIPName(t *testing.T) {
	t.Parallel()

	bornDigitalSIPName := "SIP_20010106_someoffice_someref"
	bornDigitalSIP, err := sip.New(filepath.Join(bornDigitalSIPTempDir(t, bornDigitalSIPName), bornDigitalSIPName))
	assert.NilError(t, err)

	bornDigitalSIPUnderscoreRefName := "SIP_20010106_someoffice_some_ref"
	bornDigitalSIPUnderscoreRef, err := sip.New(
		filepath.Join(bornDigitalSIPTempDir(t, bornDigitalSIPUnderscoreRefName), bornDigitalSIPUnderscoreRefName),
	)
	assert.NilError(t, err)

	bornDigitalSIPNoRefName := "SIP_20010106_someoffice"
	bornDigitalSIPNoRef, err := sip.New(
		filepath.Join(bornDigitalSIPTempDir(t, bornDigitalSIPNoRefName), bornDigitalSIPNoRefName),
	)
	assert.NilError(t, err)

	bornDigitalSIPBadName := "somref"
	bornDigitalSIPBad, err := sip.New(
		filepath.Join(bornDigitalSIPTempDir(t, bornDigitalSIPBadName), bornDigitalSIPBadName),
	)
	assert.NilError(t, err)

	digitizedSIPName := "SIP_20010106_Vecteur_someref"
	digitizedSIP, err := sip.New(
		filepath.Join(digitizedSIPTempDir(t, digitizedSIPName), digitizedSIPName),
	)
	assert.NilError(t, err)

	digitizedSIPUnderscoreRefName := "SIP_20010106_Vecteur_some_ref"
	digitizedSIPUnderscoreRef, err := sip.New(
		filepath.Join(digitizedSIPTempDir(t, digitizedSIPUnderscoreRefName), digitizedSIPUnderscoreRefName),
	)
	assert.NilError(t, err)

	digitizedSIPBadName := "someref"
	digitizedSIPBad, err := sip.New(
		filepath.Join(digitizedSIPTempDir(t, digitizedSIPBadName), digitizedSIPBadName),
	)
	assert.NilError(t, err)

	tests := []struct {
		name    string
		params  activities.ValidateSIPNameParams
		want    activities.ValidateSIPNameResult
		wantErr string
	}{
		{
			name:   "Validates the name of a born digital SIP",
			params: activities.ValidateSIPNameParams{SIP: bornDigitalSIP},
		},
		{
			name:   "Validates the name of a born digital SIP with an underscore in the reference number",
			params: activities.ValidateSIPNameParams{SIP: bornDigitalSIPUnderscoreRef},
		},
		{
			name:   "Validates the name of a born digital SIP with no reference number",
			params: activities.ValidateSIPNameParams{SIP: bornDigitalSIPNoRef},
		},
		{
			name:   "Validates the name of a born digital SIP with a bad name",
			params: activities.ValidateSIPNameParams{SIP: bornDigitalSIPBad},
			want: activities.ValidateSIPNameResult{
				Failures: []string{
					fmt.Sprintf(
						"SIP name %q violates naming standard",
						bornDigitalSIPBadName,
					),
				},
			},
		},
		{
			name:   "Validates the name of a digitized SIP",
			params: activities.ValidateSIPNameParams{SIP: digitizedSIP},
		},
		{
			name:   "Validates the name of a digitized SIP with an underscore in the reference number",
			params: activities.ValidateSIPNameParams{SIP: digitizedSIPUnderscoreRef},
		},
		{
			name:   "Validates the name of a digitized SIP with a bad name",
			params: activities.ValidateSIPNameParams{SIP: digitizedSIPBad},
			want: activities.ValidateSIPNameResult{
				Failures: []string{
					fmt.Sprintf(
						"SIP name %q violates naming standard",
						digitizedSIPBadName,
					),
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
				activities.NewValidateSIPName().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateSIPNameName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateSIPNameName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var result activities.ValidateSIPNameResult
			assert.NilError(t, enc.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}

func bornDigitalSIPTempDir(t *testing.T, sipName string) string {
	return fs.NewDir(t, "",
		fs.WithDir(sipName,
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
				),
			),
			fs.WithDir("header",
				fs.WithFile("metadata.xml", ""),
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", ""),
				),
			),
		),
	).Path()
}

func digitizedSIPTempDir(t *testing.T, sipName string) string {
	return fs.NewDir(t, "",
		fs.WithDir(sipName,
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
		),
	).Path()
}
