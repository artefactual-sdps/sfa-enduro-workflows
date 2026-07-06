package activities_test

import (
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
)

func TestUnbag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		params  func(string) activities.UnbagParams
		result  func(string) activities.UnbagResult
		wantFS  fs.Manifest
		wantErr string
	}{
		{
			name: "Unbags a bag",
			path: fs.NewDir(t, "enduro-test",
				fs.WithDir("data",
					fs.WithDir("d_0000001",
						fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
					),
					fs.WithDir("additional"),
				),
				fs.WithFile("bagit.txt", ""),
				fs.WithFile("manifest-md5.txt", ""),
			).Path(),
			params: func(path string) activities.UnbagParams {
				return activities.UnbagParams{Path: path}
			},
			result: func(path string) activities.UnbagResult {
				return activities.UnbagResult{Path: path}
			},
			wantFS: fs.Expected(t,
				fs.WithDir("d_0000001",
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
				fs.WithDir("additional"),
			),
		},
		{
			name: "Does nothing when path is not a bag",
			path: fs.NewDir(t, "enduro-test",
				fs.WithDir("d_0000001",
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
				fs.WithDir("additional"),
			).Path(),
			params: func(path string) activities.UnbagParams {
				return activities.UnbagParams{Path: path}
			},
			result: func(path string) activities.UnbagResult {
				return activities.UnbagResult{Path: path}
			},
			wantFS: fs.Expected(t,
				fs.WithDir("d_0000001",
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
				fs.WithDir("additional"),
			),
		},
		{
			name: "Errors when bag is missing data dir",
			path: fs.NewDir(t, "enduro-test",
				fs.WithDir("content",
					fs.WithDir("d_0000001",
						fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
					),
					fs.WithDir("additional"),
				),
				fs.WithFile("bagit.txt", ""),
			).Path(),
			params: func(path string) activities.UnbagParams {
				return activities.UnbagParams{Path: path}
			},
			wantErr: "activity error (type: unbag, scheduledEventID: 0, startedEventID: 0, identity: ): missing data directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewUnbag().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.UnbagName},
			)

			var res activities.UnbagResult
			future, err := env.ExecuteActivity(activities.UnbagName, tt.params(tt.path))
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
			assert.DeepEqual(t, res, tt.result(tt.path))
		})
	}
}
