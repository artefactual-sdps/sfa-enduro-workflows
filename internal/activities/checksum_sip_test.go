package activities_test

import (
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
)

func TestChecksumPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		want    activities.ChecksumSIPResult
		wantErr string
	}{
		{
			name: "Succeeds",
			path: filepath.Join(
				fs.NewDir(t, "", fs.WithFile("test.zip", "")).Path(),
				"test.zip",
			),
			want: activities.ChecksumSIPResult{
				Algo: "SHA-256",
				Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
		{
			name:    "Fails with missing SIP",
			path:    filepath.Join(fs.NewDir(t, "").Path(), "test.zip"),
			wantErr: "ChecksumSIP: open SIP:",
		},
		{
			name:    "Fails with a directory",
			path:    fs.NewDir(t, "").Path(),
			wantErr: "ChecksumSIP: calculate checksum:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewChecksumSIP().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ChecksumSIPName},
			)

			future, err := env.ExecuteActivity(
				activities.ChecksumSIPName,
				&activities.ChecksumSIPParams{Path: tt.path},
			)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}

			assert.NilError(t, err)
			var res activities.ChecksumSIPResult
			future.Get(&res)
			assert.DeepEqual(t, res, tt.want)
		})
	}
}
