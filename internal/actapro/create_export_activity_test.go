package actapro_test

import (
	"errors"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	fake_actapro "github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/fake"
	actaprogen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

func TestCreateExportActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		response     actaprogen.CreateMassOperationExportRes
		clientErr    error
		want         actapro.CreateExportResult
		wantErr      string
		nonRetryable bool
	}{
		{
			name: "returns the export ID",
			response: &actaprogen.MassOperationInfoDTO{
				ID: actaprogen.NewOptString("1ef33301-83c4-407c-9895-18d16a1f10b9"),
			},
			want: actapro.CreateExportResult{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"},
		},
		{
			name:         "rejects a missing export ID",
			response:     &actaprogen.MassOperationInfoDTO{},
			wantErr:      "create ACTApro export: missing export ID in created response",
			nonRetryable: true,
		},
		{
			name:         "rejects an empty export ID",
			response:     &actaprogen.MassOperationInfoDTO{ID: actaprogen.NewOptString("")},
			wantErr:      "create ACTApro export: missing export ID in created response",
			nonRetryable: true,
		},
		{
			name: "returns bad request error",
			response: &actaprogen.CreateMassOperationExportBadRequest{
				Message: actaprogen.NewOptString("invalid document key"),
			},
			wantErr:      "create ACTApro export: bad request: invalid document key",
			nonRetryable: true,
		},
		{
			name: "returns forbidden error",
			response: &actaprogen.CreateMassOperationExportForbidden{
				Message: actaprogen.NewOptString("access denied"),
			},
			wantErr:      "create ACTApro export: forbidden: access denied",
			nonRetryable: true,
		},
		{
			name: "returns not found error",
			response: &actaprogen.CreateMassOperationExportNotFound{
				Message: actaprogen.NewOptString("unknown script"),
			},
			wantErr:      "create ACTApro export: not found: unknown script",
			nonRetryable: true,
		},
		{
			name: "returns retryable conflict error",
			response: &actaprogen.CreateMassOperationExportConflict{
				Message: actaprogen.NewOptString("export in progress"),
			},
			wantErr:      "create ACTApro export: conflict: export in progress",
			nonRetryable: false,
		},
		{
			name: "returns retryable locked error",
			response: &actaprogen.CreateMassOperationExportLocked{
				Message: actaprogen.NewOptString("document is locked"),
			},
			wantErr:      "create ACTApro export: locked: document is locked",
			nonRetryable: false,
		},
		{
			name: "returns retryable server error",
			response: &actaprogen.CreateMassOperationExportInternalServerError{
				Message: actaprogen.NewOptString("service unavailable"),
			},
			wantErr:      "create ACTApro export: server error: service unavailable",
			nonRetryable: false,
		},
		{
			name:         "returns retryable client error",
			clientErr:    errors.New("error from client"),
			wantErr:      "create ACTApro export: error from client",
			nonRetryable: false,
		},
		{
			name:         "returns retryable unexpected response error",
			wantErr:      "create ACTApro export: unexpected response",
			nonRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			client.EXPECT().CreateMassOperationExport(gomock.Any(), &actaprogen.ExportParamsDTO{
				ScriptId:      "fb104fbe-de3f-419c-82e9-4a0767ea5d48",
				KeysRecursive: actaprogen.NewOptBool(true),
				ExportDocKeys: []string{"CH-000001"},
				ExportScriptOptions: actaprogen.ExportScriptOptionsDTO{
					ScriptOptions: actaprogen.NewOptExportScriptOptionsDTOScriptOptions(
						actaprogen.ExportScriptOptionsDTOScriptOptions{},
					),
				},
			}).Return(tt.response, tt.clientErr)

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewCreateExportActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.CreateExportActivityName},
			)
			future, err := env.ExecuteActivity(actapro.CreateExportActivityName, &actapro.CreateExportParams{
				DocKey: "CH-000001",
			})
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				var applicationErr *temporalsdk_temporal.ApplicationError
				assert.Assert(t, errors.As(err, &applicationErr))
				assert.Equal(t, applicationErr.NonRetryable(), tt.nonRetryable)
				return
			}
			assert.NilError(t, err)

			var result actapro.CreateExportResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
