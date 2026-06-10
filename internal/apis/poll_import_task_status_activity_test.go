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

func TestPollImportTaskStatusActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  apis.PollImportTaskStatusParams
		expect  func(*fake_apis.MockClientMockRecorder)
		want    apis.PollImportTaskStatusResult
		wantErr string
	}{
		{
			name:   "polls until analysis is complete",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000001"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000001"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusNeu},
					nil,
				)
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000001"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusInAnalyse},
					nil,
				)
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000001"},
				).Return(
					&apisgen.ImportTaskStatusResponse{
						Status:         apisgen.ImportTaskStatusAnalysiert,
						AnalysisResult: apisgen.NewOptNilAnalysisResult(apisgen.AnalysisResultAlleNeu),
					},
					nil,
				)
			},
			want: apis.PollImportTaskStatusResult{AnalysisResult: apisgen.AnalysisResultAlleNeu},
		},
		{
			name:   "returns conflict analysis result",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000002"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000002"},
				).Return(
					&apisgen.ImportTaskStatusResponse{
						Status:         apisgen.ImportTaskStatusAnalysiert,
						AnalysisResult: apisgen.NewOptNilAnalysisResult(apisgen.AnalysisResultKonflikte),
					},
					nil,
				)
			},
			want: apis.PollImportTaskStatusResult{AnalysisResult: apisgen.AnalysisResultKonflikte},
		},
		{
			name:   "returns analysis error result",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000003"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000003"},
				).Return(
					&apisgen.ImportTaskStatusResponse{
						Status:         apisgen.ImportTaskStatusAnalysiert,
						AnalysisResult: apisgen.NewOptNilAnalysisResult(apisgen.AnalysisResultFehler),
					},
					nil,
				)
			},
			want: apis.PollImportTaskStatusResult{AnalysisResult: apisgen.AnalysisResultFehler},
		},
		{
			name:   "returns polling error",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000004"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000004"},
				).Return(nil, errors.New("status boom"))
			},
			wantErr: "status boom",
		},
		{
			name:   "returns unauthorized error response",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000005"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000005"},
				).Return(
					&apisgen.APIImporttasksIDStatusGetUnauthorized{
						Detail: apisgen.NewOptNilString("unauthorized"),
					},
					nil,
				)
			},
			wantErr: "poll APIS import task status: unauthorized",
		},
		{
			name:   "returns not found error response",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000006"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000006"},
				).Return(
					&apisgen.APIImporttasksIDStatusGetNotFound{
						Detail: apisgen.NewOptNilString("import task does not exist"),
					},
					nil,
				)
			},
			wantErr: "poll APIS import task status: task not found: import task does not exist",
		},
		{
			name:   "returns internal server error response",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000007"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000007"},
				).Return(
					&apisgen.APIImporttasksIDStatusGetInternalServerError{
						Detail: apisgen.NewOptNilString("status backend failed"),
					},
					nil,
				)
			},
			wantErr: "poll APIS import task status: server error: status backend failed",
		},
		{
			name:   "returns error on unexpected analysis status",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000008"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000008"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusImportiert},
					nil,
				)
			},
			wantErr: "unexpected APIS import task status: Importiert",
		},
		{
			name:   "returns error when analysis result is missing",
			params: apis.PollImportTaskStatusParams{TaskID: "task-000009"},
			expect: func(m *fake_apis.MockClientMockRecorder) {
				m.APIImporttasksIDStatusGet(
					gomock.Any(),
					apisgen.APIImporttasksIDStatusGetParams{ID: "task-000009"},
				).Return(
					&apisgen.ImportTaskStatusResponse{Status: apisgen.ImportTaskStatusAnalysiert},
					nil,
				)
			},
			wantErr: "missing analysis result",
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
				apis.NewPollImportTaskStatusActivity(client, time.Millisecond).Execute,
				temporalsdk_activity.RegisterOptions{Name: apis.PollImportTaskStatusActivityName},
			)

			future, err := env.ExecuteActivity(apis.PollImportTaskStatusActivityName, &tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				assert.ErrorContains(t, err, tt.wantErr)

				return
			}
			assert.NilError(t, err)

			var result apis.PollImportTaskStatusResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
