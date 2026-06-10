package apis_test

import (
	"context"
	"errors"
	"io"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	fake_apis "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/fake"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func TestCreateImportTaskActivity(t *testing.T) {
	t.Parallel()

	sipValue, err := sip.New(fs.NewDir(t, "",
		fs.WithDir("content",
			fs.WithDir("d_0000001",
				fs.WithFile("content.txt", "payload"),
			),
		),
		fs.WithDir("header",
			fs.WithFile("metadata.xml", "metadata-body"),
			fs.WithDir("xsd",
				fs.WithFile("arelda.xsd", ""),
			),
		),
	).Path())
	assert.NilError(t, err)

	tests := []struct {
		name    string
		params  apis.CreateImportTaskParams
		expect  func(*fake_apis.MockClientMockRecorder)
		want    apis.CreateImportTaskResult
		wantErr string
	}{
		{
			name: "creates import task",
			params: apis.CreateImportTaskParams{
				SIP:      sipValue,
				Username: "archivist@example.com",
			},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksPost(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req apisgen.OptAPIImporttasksPostReq) (apisgen.APIImporttasksPostRes, error) {
						payload, ok := req.Get()
						assert.Assert(t, ok)
						assert.Equal(t, payload.Username, "archivist@example.com")
						assert.Equal(t, payload.SipType, apisgen.SipTypeBornDigitalSIP)
						assert.Equal(t, payload.File.Name, "metadata.xml")

						body, err := io.ReadAll(payload.File.File)
						assert.NilError(t, err)
						assert.Equal(t, string(body), "metadata-body")

						res := apisgen.CreateImportTaskResponse{
							ImportTaskId: "task-000001",
						}
						return &res, nil
					},
				)
			},
			want: apis.CreateImportTaskResult{TaskID: "task-000001"},
		},
		{
			name: "passes empty username through",
			params: apis.CreateImportTaskParams{
				SIP: sipValue,
			},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksPost(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req apisgen.OptAPIImporttasksPostReq) (apisgen.APIImporttasksPostRes, error) {
						payload, ok := req.Get()
						assert.Assert(t, ok)
						assert.Equal(t, payload.Username, "")

						res := apisgen.CreateImportTaskResponse{
							ImportTaskId: "task-000002",
						}
						return &res, nil
					},
				)
			},
			want: apis.CreateImportTaskResult{TaskID: "task-000002"},
		},
		{
			name: "returns bad request error",
			params: apis.CreateImportTaskParams{
				SIP: sipValue,
			},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksPost(gomock.Any(), gomock.Any()).Return(
					&apisgen.APIImporttasksPostBadRequest{
						Detail: apisgen.NewOptNilString("invalid payload"),
					},
					nil,
				)
			},
			wantErr: "create APIS import task: bad request: invalid payload",
		},
		{
			name: "returns unauthorized error",
			params: apis.CreateImportTaskParams{
				SIP: sipValue,
			},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksPost(gomock.Any(), gomock.Any()).Return(
					&apisgen.APIImporttasksPostUnauthorized{Detail: apisgen.NewOptNilString("unauthorized")},
					nil,
				)
			},
			wantErr: "create APIS import task: unauthorized",
		},
		{
			name: "returns client error",
			params: apis.CreateImportTaskParams{
				SIP: sipValue,
			},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksPost(gomock.Any(), gomock.Any()).Return(nil, errors.New("error from client"))
			},
			wantErr: "create APIS import task: error from client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := fake_apis.NewMockClient(ctrl)
			if tt.expect != nil {
				tt.expect(client.EXPECT())
			}

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				apis.NewCreateImportTaskActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: apis.CreateImportTaskActivityName},
			)

			future, err := env.ExecuteActivity(apis.CreateImportTaskActivityName, &tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				assert.ErrorContains(t, err, tt.wantErr)

				return
			}
			assert.NilError(t, err)

			var result apis.CreateImportTaskResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
