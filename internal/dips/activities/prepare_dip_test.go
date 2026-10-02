package activities_test

import (
	"os"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
)

func TestPrepareDIP(t *testing.T) {
	t.Parallel()

	schemas := fs.NewDir(t, "schemas",
		fs.WithFile("arelda.xsd", "main schema"),
		fs.WithFile("base.xsd", "base schema"),
		fs.WithFile("datei.xsd", "file schema"),
		fs.WithFile("README.txt", "not a schema"),
		fs.WithDir("ignored.xsd"),
	)
	workingDir := fs.NewDir(t, "dip-working",
		fs.WithFile("export.xml", "<paket>exported metadata</paket>", fs.WithMode(0o600)),
	)
	params := &activities.PrepareDIPParams{
		DIPPath:      workingDir.Join("DIP_9390594f-84c2-457d-bd6a-618f21f7c954"),
		MetadataPath: workingDir.Join("export.xml"),
		XSDDir:       schemas.Path(),
	}
	ts := &temporalsdk_testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	env.RegisterActivityWithOptions(
		activities.NewPrepareDIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.PrepareDIPName},
	)

	enc, err := env.ExecuteActivity(activities.PrepareDIPName, params)
	assert.NilError(t, err)
	var result activities.PrepareDIPResult
	assert.NilError(t, enc.Get(&result))

	assert.Assert(t, fs.Equal(params.DIPPath, fs.Expected(t,
		fs.WithDir("header",
			fs.WithMode(0o700),
			fs.WithFile("metadata.xml", "<paket>exported metadata</paket>", fs.WithMode(0o600)),
			fs.WithDir("xsd",
				fs.WithMode(0o700),
				fs.WithFile("arelda.xsd", "main schema", fs.WithMode(0o600)),
				fs.WithFile("base.xsd", "base schema", fs.WithMode(0o600)),
				fs.WithFile("datei.xsd", "file schema", fs.WithMode(0o600)),
			),
		),
	)))

	// Preparing the DIP moves the export and preserves the source schemas.
	_, err = os.Stat(params.MetadataPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, name := range []string{"arelda.xsd", "base.xsd", "datei.xsd"} {
		_, err := os.Stat(schemas.Join(name))
		assert.NilError(t, err)
	}
}

func TestPrepareDIPFails(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		setup   func(*testing.T, *activities.PrepareDIPParams)
		wantErr string
	}{
		{
			name: "Cannot create header",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Put a file at the DIP path so the header directory cannot be created.
				assert.NilError(t, os.WriteFile(params.DIPPath, nil, 0o600))
			},
			wantErr: "create DIP header:",
		},
		{
			name: "Missing schema directory",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Point to a missing schema directory so reading its entries fails.
				params.XSDDir = filepath.Join(t.TempDir(), "missing")
			},
			wantErr: "read DIP schemas:",
		},
		{
			name: "Cannot copy schema",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Put a directory at the schema destination so it cannot be opened as a file.
				assert.NilError(t, os.MkdirAll(filepath.Join(params.DIPPath, "header", "xsd", "arelda.xsd"), 0o700))
			},
			wantErr: "copy DIP schema arelda.xsd:",
		},
		{
			name: "Missing export",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Point to a missing export so the move cannot find its source.
				params.MetadataPath = filepath.Join(t.TempDir(), "missing.xml")
			},
			wantErr: "move DIP metadata:",
		},
		{
			name: "Cannot move export",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Create the metadata destination so fsutil.Move rejects the existing path.
				assert.NilError(t, os.MkdirAll(filepath.Join(params.DIPPath, "header", "metadata.xml"), 0o700))
			},
			wantErr: "move DIP metadata:",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schemas := fs.NewDir(t, "schemas", fs.WithFile("arelda.xsd", "schema"))
			workingDir := fs.NewDir(t, "dip-working", fs.WithFile("metadata.xml", "<paket/>"))
			params := &activities.PrepareDIPParams{
				DIPPath:      workingDir.Join("DIP_9390594f-84c2-457d-bd6a-618f21f7c954"),
				MetadataPath: workingDir.Join("metadata.xml"),
				XSDDir:       schemas.Path(),
			}
			tt.setup(t, params)

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewPrepareDIP().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.PrepareDIPName},
			)
			_, err := env.ExecuteActivity(activities.PrepareDIPName, params)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
