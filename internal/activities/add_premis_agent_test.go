package activities_test

import (
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
)

func TestAddPREMISAgent(t *testing.T) {
	t.Parallel()

	// Normally populated files (for execution expected to work).
	ContentFilesNormal := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)

	PREMISFilePathNormal := ContentFilesNormal.Join("metadata", "premis.xml")

	// No files (for execution expected to work).
	ContentNoFiles := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)

	PREMISFilePathNoFiles := ContentNoFiles.Join("metadata", "premis.xml")

	// Non-existent paths (for execution expected to fail).
	ContentNonExistent := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)

	PREMISFilePathNonExistent := ContentNonExistent.Join("metadata", "premis.xml")

	ContentNonExistent.Remove()

	tests := []struct {
		name    string
		params  activities.AddPREMISAgentParams
		result  activities.AddPREMISAgentResult
		wantErr string
	}{
		{
			name: "Add PREMIS agent for normal content",
			params: activities.AddPREMISAgentParams{
				PREMISFilePath: PREMISFilePathNormal,
				Agent:          premis.AgentDefault(),
			},
			result: activities.AddPREMISAgentResult{},
		},
		{
			name: "Add PREMIS agent for no content",
			params: activities.AddPREMISAgentParams{
				PREMISFilePath: PREMISFilePathNoFiles,
				Agent:          premis.AgentDefault(),
			},
			result: activities.AddPREMISAgentResult{},
		},
		{
			name: "Add PREMIS agent for bad path",
			params: activities.AddPREMISAgentParams{
				PREMISFilePath: PREMISFilePathNonExistent,
				Agent:          premis.AgentDefault(),
			},
			result:  activities.AddPREMISAgentResult{},
			wantErr: "no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewAddPREMISAgent().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISAgentName},
			)

			var res activities.AddPREMISAgentResult
			future, err := env.ExecuteActivity(activities.AddPREMISAgentName, tt.params)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}

			assert.NilError(t, err)

			future.Get(&res)
			assert.NilError(t, err)
			assert.DeepEqual(t, res, tt.result)

			_, err = premis.ParseFile(tt.params.PREMISFilePath)
			assert.NilError(t, err)
		})
	}
}
