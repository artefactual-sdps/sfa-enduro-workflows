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
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
)

func TestPrepareDIP(t *testing.T) {
	t.Parallel()

	schemas := fs.NewDir(t, "schemas",
		fs.WithFile("arelda.xsd", "main schema"),
		fs.WithFile("base.xsd", "other schema"),
		fs.WithDir("nested", fs.WithFile("base.xsd", "base schema")),
		fs.WithFile("datei.xsd", "file schema"),
		fs.WithDir("ignored.xsd"),
	)
	workingDir := fs.NewDir(t, "dip-working",
		fs.WithFile("export.xml", "<paket>exported metadata</paket>", fs.WithMode(0o600)),
	)
	params := &activities.PrepareDIPParams{
		DIPPath:      workingDir.Join("DIP_9390594f-84c2-457d-bd6a-618f21f7c954"),
		MetadataPath: workingDir.Join("export.xml"),
		XSDDir:       schemas.Path(),
		HeaderFiles: []*datatypes.File{
			{
				DIPPath:           "header/xsd/arelda.xsd",
				Checksum:          "8DCD6DD15CC9B5CC9F31F84531E05CBC",
				ChecksumAlgorithm: "MD5",
			},
			{
				DIPPath:           "header/xsd/nested/base.xsd",
				Checksum:          "d2dc225da465e15756668e26fba00073ea73b6313d6ab7f82a910005b8c7bee7",
				ChecksumAlgorithm: "SHA-256",
			},
			{
				DIPPath:           "header/xsd/datei.xsd",
				Checksum:          "20c31b8bae0fc28e0614ef3df269f52e",
				ChecksumAlgorithm: "MD5",
			},
		},
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
				fs.WithFile("datei.xsd", "file schema", fs.WithMode(0o600)),
				fs.WithDir("nested",
					fs.WithMode(0o700),
					fs.WithFile("base.xsd", "base schema", fs.WithMode(0o600)),
				),
			),
		),
	)))

	// Preparing the DIP moves the export and preserves the source schemas.
	_, err = os.Stat(params.MetadataPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, name := range []string{"arelda.xsd", "base.xsd", "nested/base.xsd", "datei.xsd"} {
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
			name: "Cannot create nested schema directory",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Put a file at the nested directory path so the schema's parent cannot be created.
				xsdPath := filepath.Join(params.DIPPath, "header", "xsd")
				assert.NilError(t, os.MkdirAll(xsdPath, 0o700))
				assert.NilError(t, os.WriteFile(filepath.Join(xsdPath, "nested"), nil, 0o600))
				params.HeaderFiles[0] = &datatypes.File{
					DIPPath:           "header/xsd/nested/base.xsd",
					Checksum:          "d2dc225da465e15756668e26fba00073ea73b6313d6ab7f82a910005b8c7bee7",
					ChecksumAlgorithm: "SHA-256",
				}
			},
			wantErr: `copy DIP schema "header/xsd/nested/base.xsd": mkdir`,
		},
		{
			name: "Missing schema directory",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Point to a missing schema directory so opening the schema file fails.
				params.XSDDir = filepath.Join(t.TempDir(), "missing")
			},
			wantErr: `validate DIP schema checksum "header/xsd/arelda.xsd": open file:`,
		},
		{
			name: "Missing schema file",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				params.HeaderFiles[0].DIPPath = "header/xsd/missing.xsd"
			},
			wantErr: `validate DIP schema checksum "header/xsd/missing.xsd": open file:`,
		},
		{
			name: "Missing nested schema even when its basename exists at the root",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				params.HeaderFiles[0].DIPPath = "header/xsd/missing/arelda.xsd"
			},
			wantErr: `validate DIP schema checksum "header/xsd/missing/arelda.xsd": open file:`,
		},
		{
			name: "Reports checksum failures and unexpected files after checking all schemas",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				params.HeaderFiles = []*datatypes.File{
					{
						DIPPath: "header/extra.txt",
					},
					{
						DIPPath:           "header/xsd/arelda.xsd",
						Checksum:          "incorrect",
						ChecksumAlgorithm: "MD5",
					},
					{
						DIPPath:           "header/xsd/base.xsd",
						Checksum:          "incorrect",
						ChecksumAlgorithm: "MD5",
					},
					{
						DIPPath:           "header/xsd/datei.xsd",
						Checksum:          "checksum",
						ChecksumAlgorithm: "unsupported",
					},
					{
						DIPPath:           "header/xsd/nested/base.xsd",
						Checksum:          "d2dc225da465e15756668e26fba00073ea73b6313d6ab7f82a910005b8c7bee7",
						ChecksumAlgorithm: "SHA-256",
					},
				}
			},
			wantErr: `validate DIP schema checksum "header/xsd/arelda.xsd": checksum mismatch ` +
				`(expected: "incorrect", got: "8dcd6dd15cc9b5cc9f31f84531e05cbc")` + "\n" +
				`validate DIP schema checksum "header/xsd/base.xsd": checksum mismatch ` +
				`(expected: "incorrect", got: "84ccbf66da5d6ad38104e4ec900b06c1")` + "\n" +
				`validate DIP schema checksum "header/xsd/datei.xsd": hash algorithm "unsupported" is not supported` + "\n" +
				"unexpected header files:\nheader/extra.txt",
		},
		{
			name: "Reports every unexpected header file",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				params.HeaderFiles = []*datatypes.File{
					{
						DIPPath: "header/extra.xml",
					},
					{
						DIPPath:           "header/old/base.xsd",
						Checksum:          "84ccbf66da5d6ad38104e4ec900b06c1",
						ChecksumAlgorithm: "MD5",
					},
					{
						DIPPath:           "header/xsd-other/datei.xsd",
						Checksum:          "20c31b8bae0fc28e0614ef3df269f52e",
						ChecksumAlgorithm: "MD5",
					},
				}
			},
			wantErr: "unexpected header files:\n" +
				"header/extra.xml\nheader/old/base.xsd\nheader/xsd-other/datei.xsd",
		},
		{
			name: "Reports unexpected header files when there are no schemas",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				params.HeaderFiles = []*datatypes.File{
					{DIPPath: "header/extra.xml"},
					{DIPPath: "header/old/extra.xml"},
				}
			},
			wantErr: "unexpected header files:\nheader/extra.xml\nheader/old/extra.xml",
		},
		{
			name: "Cannot copy schema",
			setup: func(t *testing.T, params *activities.PrepareDIPParams) {
				// Put a directory at the schema destination so it cannot be opened as a file.
				assert.NilError(t, os.MkdirAll(filepath.Join(params.DIPPath, "header", "xsd", "arelda.xsd"), 0o700))
			},
			wantErr: `copy DIP schema "header/xsd/arelda.xsd":`,
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

			schemas := fs.NewDir(t, "schemas",
				fs.WithFile("arelda.xsd", "main schema"),
				fs.WithFile("base.xsd", "other schema"),
				fs.WithDir("nested", fs.WithFile("base.xsd", "base schema")),
				fs.WithFile("datei.xsd", "file schema"),
			)
			workingDir := fs.NewDir(t, "dip-working", fs.WithFile("metadata.xml", "<paket/>"))
			params := &activities.PrepareDIPParams{
				DIPPath:      workingDir.Join("DIP_9390594f-84c2-457d-bd6a-618f21f7c954"),
				MetadataPath: workingDir.Join("metadata.xml"),
				XSDDir:       schemas.Path(),
				HeaderFiles: []*datatypes.File{{
					DIPPath:           "header/xsd/arelda.xsd",
					Checksum:          "8DCD6DD15CC9B5CC9F31F84531E05CBC",
					ChecksumAlgorithm: "MD5",
				}},
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
