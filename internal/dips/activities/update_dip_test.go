package activities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.artefactual.dev/tools/mockutil"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	persistencefake "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/fake"
)

func TestUpdateDIP(t *testing.T) {
	t.Parallel()

	dipUUID := uuid.New()
	createdAt := time.Date(2024, 6, 13, 17, 50, 12, 0, time.UTC)
	startedAt := createdAt.Add(time.Minute)
	completedAt := startedAt.Add(time.Minute)
	dip := datatypes.DIP{
		DBID:         1,
		UUID:         dipUUID,
		DocKey:       "CH-000001",
		Status:       enums.DIPStatusInProgress,
		ErrorMessage: "previous error",
		CreatedAt:    createdAt,
		StartedAt:    createdAt.Add(time.Second),
		CompletedAt:  createdAt.Add(2 * time.Second),
		ObjectKey:    "previous-object-key",
	}

	for _, tt := range []struct {
		name    string
		params  *activities.UpdateDIPParams
		wantDIP datatypes.DIP
		err     error
		wantErr string
	}{
		{
			name: "Updates a DIP",
			params: &activities.UpdateDIPParams{DIP: datatypes.DIP{
				UUID:         dipUUID,
				Status:       enums.DIPStatusFailed,
				StartedAt:    startedAt,
				CompletedAt:  completedAt,
				ErrorMessage: "DIP creation failed",
				ObjectKey:    "updated-object-key",
			}},
			wantDIP: datatypes.DIP{
				DBID:         1,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusFailed,
				ErrorMessage: "DIP creation failed",
				CreatedAt:    createdAt,
				StartedAt:    startedAt,
				CompletedAt:  completedAt,
				ObjectKey:    "updated-object-key",
			},
		},
		{
			name: "Does not overwrite fields with zero values",
			params: &activities.UpdateDIPParams{DIP: datatypes.DIP{
				UUID: dipUUID,
			}},
			wantDIP: dip,
		},
		{
			name: "Does not overwrite status with an invalid value",
			params: &activities.UpdateDIPParams{DIP: datatypes.DIP{
				UUID:   dipUUID,
				Status: "invalid",
			}},
			wantDIP: dip,
		},
		{
			name: "Fails if there is a persistence error",
			params: &activities.UpdateDIPParams{DIP: datatypes.DIP{
				UUID: dipUUID,
			}},
			wantDIP: dip,
			err:     errors.New("persistence error"),
			wantErr: "failed to update DIP: persistence error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := persistencefake.NewMockService(gomock.NewController(t))
			svc.EXPECT().
				UpdateDIP(
					mockutil.Context(),
					dipUUID,
					mockutil.Func(
						"should update DIP",
						func(updater persistence.DIPUpdater) error {
							d := dip
							got, err := updater(&d)
							assert.NilError(t, err)
							assert.DeepEqual(t, got, &tt.wantDIP)
							return nil
						},
					),
				).
				Return(nil, tt.err)

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewUpdateDIP(svc).Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.UpdateDIPName},
			)
			enc, err := env.ExecuteActivity(activities.UpdateDIPName, tt.params)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var res activities.UpdateDIPResult
			assert.NilError(t, enc.Get(&res))
			assert.DeepEqual(t, res, activities.UpdateDIPResult{})
		})
	}
}
