package actapro_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	fake_actapro "github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/fake"
	actaprogen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

func TestPollExportStatusActivityErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		response     actaprogen.GetMassOperationInfoRes
		clientErr    error
		wantErr      string
		nonRetryable bool
	}{
		{
			name: "returns bad request error",
			response: &actaprogen.GetMassOperationInfoBadRequest{
				Message: actaprogen.NewOptString("status unavailable"),
			},
			wantErr:      "poll ACTApro export status: bad request: status unavailable",
			nonRetryable: true,
		},
		{
			name: "returns forbidden error",
			response: &actaprogen.GetMassOperationInfoForbidden{
				Message: actaprogen.NewOptString("status unavailable"),
			},
			wantErr:      "poll ACTApro export status: forbidden: status unavailable",
			nonRetryable: true,
		},
		{
			name: "returns not found error",
			response: &actaprogen.GetMassOperationInfoNotFound{
				Message: actaprogen.NewOptString("status unavailable"),
			},
			wantErr:      "poll ACTApro export status: not found: status unavailable",
			nonRetryable: true,
		},
		{
			name: "returns conflict error",
			response: &actaprogen.GetMassOperationInfoConflict{
				Message: actaprogen.NewOptString("status unavailable"),
			},
			wantErr:      "poll ACTApro export status: conflict: status unavailable",
			nonRetryable: false,
		},
		{
			name: "returns locked error",
			response: &actaprogen.GetMassOperationInfoLocked{
				Message: actaprogen.NewOptString("status unavailable"),
			},
			wantErr:      "poll ACTApro export status: locked: status unavailable",
			nonRetryable: false,
		},
		{
			name: "returns server error",
			response: &actaprogen.GetMassOperationInfoInternalServerError{
				Message: actaprogen.NewOptString("status unavailable"),
			},
			wantErr:      "poll ACTApro export status: server error: status unavailable",
			nonRetryable: false,
		},
		{
			name:         "returns retryable client error",
			clientErr:    errors.New("status unavailable"),
			wantErr:      "poll ACTApro export status: status unavailable",
			nonRetryable: false,
		},
		{
			name:         "returns retryable unexpected response error",
			wantErr:      "poll ACTApro export status: unexpected response",
			nonRetryable: false,
		},
		{
			name:         "rejects missing status",
			response:     &actaprogen.MassOperationInfoDTO{},
			wantErr:      "poll ACTApro export status: missing status",
			nonRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			client.EXPECT().GetMassOperationInfo(gomock.Any(), actaprogen.GetMassOperationInfoParams{
				ID: uuid.MustParse("1ef33301-83c4-407c-9895-18d16a1f10b9"),
			}).Return(tt.response, tt.clientErr)

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewPollExportStatusActivity(client, time.Millisecond).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
			)
			_, err := env.ExecuteActivity(actapro.PollExportStatusActivityName, &actapro.PollExportStatusParams{
				ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9",
			})
			assert.ErrorContains(t, err, tt.wantErr)
			var applicationErr *temporalsdk_temporal.ApplicationError
			assert.Assert(t, errors.As(err, &applicationErr))
			assert.Equal(t, applicationErr.NonRetryable(), tt.nonRetryable)
		})
	}
}

func TestPollExportStatusActivityInvalidExportID(t *testing.T) {
	t.Parallel()

	client := fake_actapro.NewMockClient(gomock.NewController(t))
	suite := temporalsdk_testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivityWithOptions(
		actapro.NewPollExportStatusActivity(client, time.Millisecond).Execute,
		temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
	)
	_, err := env.ExecuteActivity(actapro.PollExportStatusActivityName, &actapro.PollExportStatusParams{
		ExportID: "invalid",
	})
	assert.ErrorContains(t, err, "poll ACTApro export status: invalid export ID")
	var applicationErr *temporalsdk_temporal.ApplicationError
	assert.Assert(t, errors.As(err, &applicationErr))
	assert.Assert(t, applicationErr.NonRetryable())
}

func TestPollExportStatusActivityCancellation(t *testing.T) {
	t.Parallel()

	client := fake_actapro.NewMockClient(gomock.NewController(t))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	suite := temporalsdk_testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.SetWorkerOptions(temporalsdk_worker.Options{BackgroundActivityContext: ctx})
	env.RegisterActivityWithOptions(
		actapro.NewPollExportStatusActivity(client, time.Hour).Execute,
		temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
	)
	_, err := env.ExecuteActivity(actapro.PollExportStatusActivityName, &actapro.PollExportStatusParams{
		ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9",
	})
	assert.ErrorContains(t, err, context.Canceled.Error())
}

func TestPollExportStatusActivity(t *testing.T) {
	t.Parallel()

	logs := actaprogen.GetMassOperationLogsOKApplicationJSON{
		{
			OperationId: actaprogen.NewOptString("1ef33301-83c4-407c-9895-18d16a1f10b9"),
			DocKey:      actaprogen.NewOptNilString("CH-000001"),
			Status:      actaprogen.NewOptMassOperationLogDTOStatus(actaprogen.MassOperationLogDTOStatusERROR),
			Message:     actaprogen.NewOptNilString("Export script failed"),
		},
		{
			Status:  actaprogen.NewOptMassOperationLogDTOStatus(actaprogen.MassOperationLogDTOStatusWARNING),
			Message: actaprogen.NewOptNilString("Document has no files"),
		},
	}
	tests := []struct {
		name     string
		statuses []string
		logs     *actaprogen.GetMassOperationLogsOKApplicationJSON
		want     actapro.PollExportStatusResult
	}{
		{
			name:     "polls through REQUESTSTART until completed",
			statuses: []string{"REQUESTSTART", "REQUESTSTART", "COMPLETED"},
			want:     actapro.PollExportStatusResult{Status: "COMPLETED"},
		},
		{
			name:     "keeps polling other nonterminal statuses",
			statuses: []string{"UNKNOWN", "COMPLETED"},
			want:     actapro.PollExportStatusResult{Status: "COMPLETED"},
		},
		{
			name:     "returns completed status without requesting logs",
			statuses: []string{"COMPLETED"},
			want:     actapro.PollExportStatusResult{Status: "COMPLETED"},
		},
		{
			name:     "recognizes lowercase completed status",
			statuses: []string{"requeststart", "completed"},
			want:     actapro.PollExportStatusResult{Status: "COMPLETED"},
		},
		{
			name:     "recognizes mixed case completed status",
			statuses: []string{"Completed"},
			want:     actapro.PollExportStatusResult{Status: "COMPLETED"},
		},
		{
			name:     "returns failed status and formatted logs without an error",
			statuses: []string{"REQUESTSTART", "FAILED"},
			logs:     &logs,
			want: actapro.PollExportStatusResult{
				Status: "FAILED",
				Logs:   "- [ERROR] CH-000001: Export script failed\n- [WARNING]: Document has no files",
			},
		},
		{
			name:     "returns failed status with an empty logs response",
			statuses: []string{"FAILED"},
			logs:     &actaprogen.GetMassOperationLogsOKApplicationJSON{},
			want:     actapro.PollExportStatusResult{Status: "FAILED"},
		},
		{
			name:     "recognizes lowercase failed status",
			statuses: []string{"failed"},
			logs:     &logs,
			want: actapro.PollExportStatusResult{
				Status: "FAILED",
				Logs:   "- [ERROR] CH-000001: Export script failed\n- [WARNING]: Document has no files",
			},
		},
		{
			name:     "recognizes mixed case failed status",
			statuses: []string{"Failed"},
			logs:     &actaprogen.GetMassOperationLogsOKApplicationJSON{},
			want:     actapro.PollExportStatusResult{Status: "FAILED"},
		},
		{
			name:     "returns canceled status and formatted logs without an error",
			statuses: []string{"REQUESTSTART", "CANCELED"},
			logs:     &logs,
			want: actapro.PollExportStatusResult{
				Status: "CANCELED",
				Logs:   "- [ERROR] CH-000001: Export script failed\n- [WARNING]: Document has no files",
			},
		},
		{
			name:     "recognizes lowercase canceled status",
			statuses: []string{"canceled"},
			logs:     &actaprogen.GetMassOperationLogsOKApplicationJSON{},
			want:     actapro.PollExportStatusResult{Status: "CANCELED"},
		},
		{
			name:     "recognizes mixed case canceled status",
			statuses: []string{"Canceled"},
			logs:     &actaprogen.GetMassOperationLogsOKApplicationJSON{},
			want:     actapro.PollExportStatusResult{Status: "CANCELED"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			id := uuid.MustParse("1ef33301-83c4-407c-9895-18d16a1f10b9")
			var calls []any
			for _, status := range tt.statuses {
				calls = append(calls, client.EXPECT().GetMassOperationInfo(
					gomock.Any(), actaprogen.GetMassOperationInfoParams{ID: id},
				).Return(&actaprogen.MassOperationInfoDTO{Status: actaprogen.NewOptString(status)}, nil))
			}
			if tt.logs != nil {
				calls = append(calls, client.EXPECT().GetMassOperationLogs(
					gomock.Any(), actaprogen.GetMassOperationLogsParams{ID: id},
				).Return(tt.logs, nil))
			}
			gomock.InOrder(calls...)

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewPollExportStatusActivity(client, time.Millisecond).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
			)
			future, err := env.ExecuteActivity(actapro.PollExportStatusActivityName, &actapro.PollExportStatusParams{
				ExportID: id.String(),
			})
			assert.NilError(t, err)

			var result actapro.PollExportStatusResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}

func TestPollExportStatusActivityFormatsLogs(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		logs []actaprogen.MassOperationLogDTO
		want string
	}{
		{
			name: "Formats document errors and preserves CRLF line endings and Windows paths",
			logs: []actaprogen.MassOperationLogDTO{
				{
					OperationId: actaprogen.NewOptString("e873e374-607e-410a-a382-5ae06ddd28a7"),
					DocKey:      actaprogen.NewOptNilString("CH-000001"),
					Status:      actaprogen.NewOptMassOperationLogDTOStatus(actaprogen.MassOperationLogDTOStatusERROR),
					Message: actaprogen.NewOptNilString("Problem in xconv run. See logfile for more details.\r\n" +
						`See more details in file: C:\ProgramData\startext\Logs\_EventLog_20260911144044972.log`),
				},
			},
			want: "- [ERROR] CH-000001: Problem in xconv run. See logfile for more details.\r\n" +
				`See more details in file: C:\ProgramData\startext\Logs\_EventLog_20260911144044972.log`,
		},
		{
			name: "Prefixes each entry as a list item in response order",
			logs: []actaprogen.MassOperationLogDTO{
				{
					DocKey:  actaprogen.NewOptNilString("CH-000001"),
					Status:  actaprogen.NewOptMassOperationLogDTOStatus(actaprogen.MassOperationLogDTOStatusWARNING),
					Message: actaprogen.NewOptNilString("Missing file"),
				},
				{
					DocKey:  actaprogen.NewOptNilString("CH-000002"),
					Status:  actaprogen.NewOptMassOperationLogDTOStatus(actaprogen.MassOperationLogDTOStatusERROR),
					Message: actaprogen.NewOptNilString("Export script failed"),
				},
			},
			want: "- [WARNING] CH-000001: Missing file\n- [ERROR] CH-000002: Export script failed",
		},
		{
			name: "Handles optional fields and skips empty entries",
			logs: []actaprogen.MassOperationLogDTO{
				{},
				{Message: actaprogen.NewOptNilString("  Export script failed\r\n")},
				{
					DocKey:  actaprogen.NewOptNilString("CH-000001"),
					Message: actaprogen.NewOptNilString("Missing file"),
				},
				{Status: actaprogen.NewOptMassOperationLogDTOStatus(actaprogen.MassOperationLogDTOStatusERROR)},
			},
			want: "-   Export script failed\r\n\n- CH-000001: Missing file\n- [ERROR]",
		},
		{
			name: "Handles no logs",
			want: "",
		},
		{
			name: "Preserves whitespace-only messages and document keys",
			logs: []actaprogen.MassOperationLogDTO{
				{},
				{Message: actaprogen.NewOptNilString(" \r\n ")},
				{DocKey: actaprogen.NewOptNilString(" ")},
			},
			want: "-  \r\n \n-  ",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			id := uuid.MustParse("1ef33301-83c4-407c-9895-18d16a1f10b9")
			logs := actaprogen.GetMassOperationLogsOKApplicationJSON(tt.logs)
			gomock.InOrder(
				client.EXPECT().GetMassOperationInfo(
					gomock.Any(), actaprogen.GetMassOperationInfoParams{ID: id},
				).Return(&actaprogen.MassOperationInfoDTO{Status: actaprogen.NewOptString("FAILED")}, nil),
				client.EXPECT().GetMassOperationLogs(
					gomock.Any(), actaprogen.GetMassOperationLogsParams{ID: id},
				).Return(&logs, nil),
			)
			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewPollExportStatusActivity(client, time.Millisecond).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
			)
			future, err := env.ExecuteActivity(actapro.PollExportStatusActivityName, &actapro.PollExportStatusParams{
				ExportID: id.String(),
			})
			assert.NilError(t, err)

			var result actapro.PollExportStatusResult
			assert.NilError(t, future.Get(&result))
			assert.Equal(t, result.Status, "FAILED")
			assert.Equal(t, result.Logs, tt.want)
		})
	}
}

func TestPollExportStatusActivityLogsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		response     actaprogen.GetMassOperationLogsRes
		clientErr    error
		wantErr      string
		nonRetryable bool
	}{
		{
			name: "returns bad request error",
			response: &actaprogen.GetMassOperationLogsBadRequest{
				Message: actaprogen.NewOptString("logs unavailable"),
			},
			wantErr:      "get ACTApro export logs: bad request: logs unavailable",
			nonRetryable: true,
		},
		{
			name: "returns forbidden error",
			response: &actaprogen.GetMassOperationLogsForbidden{
				Message: actaprogen.NewOptString("logs unavailable"),
			},
			wantErr:      "get ACTApro export logs: forbidden: logs unavailable",
			nonRetryable: true,
		},
		{
			name: "returns not found error",
			response: &actaprogen.GetMassOperationLogsNotFound{
				Message: actaprogen.NewOptString("logs unavailable"),
			},
			wantErr:      "get ACTApro export logs: not found: logs unavailable",
			nonRetryable: true,
		},
		{
			name: "returns conflict error",
			response: &actaprogen.GetMassOperationLogsConflict{
				Message: actaprogen.NewOptString("logs unavailable"),
			},
			wantErr:      "get ACTApro export logs: conflict: logs unavailable",
			nonRetryable: false,
		},
		{
			name: "returns locked error",
			response: &actaprogen.GetMassOperationLogsLocked{
				Message: actaprogen.NewOptString("logs unavailable"),
			},
			wantErr:      "get ACTApro export logs: locked: logs unavailable",
			nonRetryable: false,
		},
		{
			name: "returns server error error",
			response: &actaprogen.GetMassOperationLogsInternalServerError{
				Message: actaprogen.NewOptString("logs unavailable"),
			},
			wantErr:      "get ACTApro export logs: server error: logs unavailable",
			nonRetryable: false,
		},
		{
			name:      "returns retryable client error",
			clientErr: errors.New("logs unavailable"),
			wantErr:   "get ACTApro export logs: logs unavailable",
		},
		{
			name:    "returns retryable unexpected response error",
			wantErr: "get ACTApro export logs: unexpected response",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			id := uuid.MustParse("1ef33301-83c4-407c-9895-18d16a1f10b9")
			gomock.InOrder(
				client.EXPECT().GetMassOperationInfo(
					gomock.Any(), actaprogen.GetMassOperationInfoParams{ID: id},
				).Return(&actaprogen.MassOperationInfoDTO{Status: actaprogen.NewOptString("FAILED")}, nil),
				client.EXPECT().GetMassOperationLogs(
					gomock.Any(), actaprogen.GetMassOperationLogsParams{ID: id},
				).Return(tt.response, tt.clientErr),
			)
			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewPollExportStatusActivity(client, time.Millisecond).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.PollExportStatusActivityName},
			)
			_, err := env.ExecuteActivity(actapro.PollExportStatusActivityName, &actapro.PollExportStatusParams{
				ExportID: id.String(),
			})
			assert.ErrorContains(t, err, tt.wantErr)
			var applicationErr *temporalsdk_temporal.ApplicationError
			assert.Assert(t, errors.As(err, &applicationErr))
			assert.Equal(t, applicationErr.NonRetryable(), tt.nonRetryable)
		})
	}
}
