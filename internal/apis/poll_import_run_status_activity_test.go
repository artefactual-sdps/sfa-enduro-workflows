package apis_test

import (
	"errors"
	"testing"
	"time"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	fake_apis "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/fake"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

func TestPollImportRunStatusActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  apis.PollImportRunStatusParams
		expect  func(*fake_apis.MockClientMockRecorder)
		want    apis.PollImportRunStatusResult
		wantErr string
	}{
		{
			name:   "polls until import is complete",
			params: apis.PollImportRunStatusParams{TaskID: "task-000001"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000001"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusAnalysiert},
					nil,
				)
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000001"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusWirdImportiert},
					nil,
				)
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000001"},
				).Return(
					&apisgen.ImportTaskStatusResponse{
						Status:       apisgen.ImportTaskStatusImportiert,
						ImportResult: apisgen.NewOptNilImportResult(apisgen.ImportResultErfolgreich),
					},
					nil,
				)
			},
			want: apis.PollImportRunStatusResult{ImportResult: apisgen.ImportResultErfolgreich},
		},
		{
			name:   "returns import error result",
			params: apis.PollImportRunStatusParams{TaskID: "task-000002"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000002"},
				).Return(
					&apisgen.ImportTaskStatusResponse{
						Status:       apisgen.ImportTaskStatusImportiert,
						ImportResult: apisgen.NewOptNilImportResult(apisgen.ImportResultFehler),
					},
					nil,
				)
			},
			want: apis.PollImportRunStatusResult{ImportResult: apisgen.ImportResultFehler},
		},
		{
			name:   "returns polling error",
			params: apis.PollImportRunStatusParams{TaskID: "task-000003"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000003"},
				).Return(nil, errors.New("status boom"))
			},
			wantErr: "status boom",
		},
		{
			name:   "returns unauthorized error response",
			params: apis.PollImportRunStatusParams{TaskID: "task-000004"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000004"},
				).Return(
					&apisgen.APIImporttasksIDStatusGetUnauthorized{
						Detail: apisgen.NewOptNilString("unauthorized"),
					},
					nil,
				)
			},
			wantErr: "poll APIS import run status: unauthorized",
		},
		{
			name:   "returns not found error response",
			params: apis.PollImportRunStatusParams{TaskID: "task-000005"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000005"},
				).Return(
					&apisgen.APIImporttasksIDStatusGetNotFound{
						Detail: apisgen.NewOptNilString("import task does not exist"),
					},
					nil,
				)
			},
			wantErr: "poll APIS import run status: task not found: import task does not exist",
		},
		{
			name:   "returns internal server error response",
			params: apis.PollImportRunStatusParams{TaskID: "task-000006"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000006"},
				).Return(
					&apisgen.APIImporttasksIDStatusGetInternalServerError{
						Detail: apisgen.NewOptNilString("status backend failed"),
					},
					nil,
				)
			},
			wantErr: "poll APIS import run status: server error: status backend failed",
		},
		{
			name:   "returns error on unexpected status",
			params: apis.PollImportRunStatusParams{TaskID: "task-000007"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000007"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusInAnalyse},
					nil,
				)
			},
			wantErr: "unexpected APIS import task status during import: InAnalyse",
		},
		{
			name:   "returns error when import result is missing",
			params: apis.PollImportRunStatusParams{TaskID: "task-000008"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000008"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusImportiert},
					nil,
				)
			},
			wantErr: "missing import result",
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
				apis.NewPollImportRunStatusActivity(client, time.Millisecond).Execute,
				temporalsdk_activity.RegisterOptions{Name: apis.PollImportRunStatusActivityName},
			)

			future, err := env.ExecuteActivity(apis.PollImportRunStatusActivityName, &tt.params)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var result apis.PollImportRunStatusResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
