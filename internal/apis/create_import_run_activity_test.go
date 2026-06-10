package apis_test

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	fake_apis "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/fake"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

func TestCreateImportRunActivity(t *testing.T) {
	t.Parallel()

	metsPath := fs.NewDir(t, "", fs.WithFile("METS.uuid.xml", "mets-body")).Join("METS.uuid.xml")

	tests := []struct {
		name    string
		params  apis.CreateImportRunParams
		expect  func(*testing.T, *fake_apis.MockClientMockRecorder)
		want    apis.CreateImportRunResult
		wantErr string
	}{
		{
			name: "creates import run",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000001",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeOverwriteAndAppend,
				Username:        "archivist@example.com",
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(
					gomock.Any(),
					gomock.Any(),
					apisgen.APIImporttasksIDImportrunsPostParams{ID: "task-000001"},
				).DoAndReturn(
					func(
						_ context.Context,
						req apisgen.OptAPIImporttasksIDImportrunsPostReq,
						_ apisgen.APIImporttasksIDImportrunsPostParams,
					) (apisgen.APIImporttasksIDImportrunsPostRes, error) {
						payload, ok := req.Get()
						assert.Assert(t, ok)
						assert.Equal(t, payload.Username, "archivist@example.com")
						assert.Equal(t, payload.File.Name, filepath.Base(metsPath))
						assert.DeepEqual(
							t,
							payload.ImportBehaviour,
							apisgen.NewOptImportBehaviourType(apisgen.ImportBehaviourTypeOverwriteAndAppend),
						)

						body, err := io.ReadAll(payload.File.File)
						assert.NilError(t, err)
						assert.Equal(t, string(body), "mets-body")

						return &apisgen.CreateImportRunResponse{ImportRunId: "run-000001"}, nil
					},
				)
			},
			want: apis.CreateImportRunResult{RunID: "run-000001"},
		},
		{
			name: "returns bad request error",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000003",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&apisgen.APIImporttasksIDImportrunsPostBadRequest{
						Detail: apisgen.NewOptNilString("cannot create import run"),
					},
					nil,
				)
			},
			wantErr: "create APIS import run: bad request: cannot create import run",
		},
		{
			name: "returns not found error",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000004",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&apisgen.APIImporttasksIDImportrunsPostNotFound{
						Detail: apisgen.NewOptNilString("import task does not exist"),
					},
					nil,
				)
			},
			wantErr: "create APIS import run: task not found: import task does not exist",
		},
		{
			name: "returns unauthorized error",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000005",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&apisgen.APIImporttasksIDImportrunsPostUnauthorized{
						Detail: apisgen.NewOptNilString("unauthorized"),
					},
					nil,
				)
			},
			wantErr: "create APIS import run: unauthorized",
		},
		{
			name: "returns unsupported media type error",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000006",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&apisgen.APIImporttasksIDImportrunsPostUnsupportedMediaType{
						Detail: apisgen.NewOptNilString("not xml"),
					},
					nil,
				)
			},
			wantErr: "create APIS import run: unsupported media type: not xml",
		},
		{
			name: "returns client error",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000007",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("error from client"))
			},
			wantErr: "create APIS import run: error from client",
		},
		{
			name: "returns missing run ID error",
			params: apis.CreateImportRunParams{
				TaskID:          "task-000008",
				METSPath:        metsPath,
				ImportBehaviour: apisgen.ImportBehaviourTypeAppendOnly,
			},
			expect: func(t *testing.T, m *fake_apis.MockClientMockRecorder) {
				t.Helper()
				m.APIImporttasksIDImportrunsPost(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&apisgen.CreateImportRunResponse{},
					nil,
				)
			},
			wantErr: "missing run ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := fake_apis.NewMockClient(ctrl)
			if tt.expect != nil {
				tt.expect(t, client.EXPECT())
			}

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				apis.NewCreateImportRunActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: apis.CreateImportRunActivityName},
			)

			future, err := env.ExecuteActivity(apis.CreateImportRunActivityName, &tt.params)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var result apis.CreateImportRunResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
