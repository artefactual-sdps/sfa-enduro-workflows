package activities_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"go.artefactual.dev/tools/mockutil"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
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
	now := time.Now().UTC()
	dip := datatypes.DIP{
		DBID:      1,
		UUID:      dipUUID,
		DocKey:    "CH-000001",
		Status:    enums.DIPStatusInProgress,
		CreatedAt: createdAt,
		StartedAt: createdAt.Add(time.Second),
	}

	for _, tt := range []struct {
		name         string
		params       *activities.UpdateDIPParams
		current      *datatypes.DIP
		wantDIP      datatypes.DIP
		err          error
		wantErr      string
		nonRetryable bool
	}{
		{
			name: "Updates a DIP",
			params: &activities.UpdateDIPParams{
				UUID:      dipUUID,
				Status:    enums.DIPStatusStored,
				ObjectKey: "archive.zip",
				Retention: time.Hour,
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: now,
				ExpiresAt:   now.Add(time.Hour),
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Marks the DIP stored without setting an expiry",
			params: &activities.UpdateDIPParams{
				UUID:      dipUUID,
				Status:    enums.DIPStatusStored,
				ObjectKey: "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: now,
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Marks a DIP failed",
			params: &activities.UpdateDIPParams{
				UUID:         dipUUID,
				Status:       enums.DIPStatusFailed,
				ErrorMessage: "DIP creation failed",
				ObjectKey:    "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:         1,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusFailed,
				CreatedAt:    createdAt,
				StartedAt:    createdAt.Add(time.Second),
				CompletedAt:  now,
				ErrorMessage: "DIP creation failed",
				ObjectKey:    "archive.zip",
			},
		},
		{
			name: "Updates the start time",
			params: &activities.UpdateDIPParams{
				WorkflowID: "create-dip-test",
				UUID:       dipUUID,
				Status:     enums.DIPStatusInProgress,
			},
			wantDIP: datatypes.DIP{
				DBID:       1,
				UUID:       dipUUID,
				DocKey:     "CH-000001",
				Status:     enums.DIPStatusInProgress,
				CreatedAt:  createdAt,
				StartedAt:  now,
				WorkflowID: "create-dip-test",
			},
		},
		{
			name: "Sets the start time",
			params: &activities.UpdateDIPParams{
				WorkflowID: "create-dip-test",
				UUID:       dipUUID,
				Status:     enums.DIPStatusInProgress,
			},
			current: &datatypes.DIP{
				DBID:      1,
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Status:    enums.DIPStatusQueued,
				CreatedAt: createdAt,
				StartedAt: time.Time{},
			},
			wantDIP: datatypes.DIP{
				DBID:       1,
				UUID:       dipUUID,
				DocKey:     "CH-000001",
				Status:     enums.DIPStatusInProgress,
				CreatedAt:  createdAt,
				StartedAt:  now,
				WorkflowID: "create-dip-test",
			},
		},
		{
			name: "Retrying completion preserves the renewed expiry",
			params: &activities.UpdateDIPParams{
				UUID:         dipUUID,
				Status:       enums.DIPStatusStored,
				ObjectKey:    "replacement.zip",
				Retention:    time.Hour,
				ErrorMessage: "retry failed",
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(3 * time.Hour),
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(3 * time.Hour),
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Retrying completion preserves indefinite retention",
			params: &activities.UpdateDIPParams{
				UUID:      dipUUID,
				Status:    enums.DIPStatusStored,
				ObjectKey: "replacement.zip",
				Retention: time.Hour,
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   time.Time{},
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   time.Time{},
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Doesn't update stored DIP to failed",
			params: &activities.UpdateDIPParams{
				UUID:         dipUUID,
				Status:       enums.DIPStatusFailed,
				ObjectKey:    "replacement.zip",
				Retention:    time.Hour,
				ErrorMessage: "retry failed",
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(time.Hour),
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(time.Hour),
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Doesn't update stored DIP to in progress",
			params: &activities.UpdateDIPParams{
				UUID:      dipUUID,
				Status:    enums.DIPStatusInProgress,
				ObjectKey: "replacement.zip",
				Retention: time.Hour,
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   time.Time{},
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   time.Time{},
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Doesn't update failed DIP state",
			params: &activities.UpdateDIPParams{
				UUID:         dipUUID,
				Status:       enums.DIPStatusStored,
				ObjectKey:    "replacement.zip",
				Retention:    time.Hour,
				ErrorMessage: "retry failed",
			},
			current: &datatypes.DIP{
				DBID:         1,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusFailed,
				CreatedAt:    createdAt,
				StartedAt:    createdAt.Add(time.Second),
				CompletedAt:  createdAt.Add(time.Minute),
				ExpiresAt:    time.Time{},
				ErrorMessage: "generation failed",
			},
			wantDIP: datatypes.DIP{
				DBID:         1,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusFailed,
				CreatedAt:    createdAt,
				StartedAt:    createdAt.Add(time.Second),
				CompletedAt:  createdAt.Add(time.Minute),
				ExpiresAt:    time.Time{},
				ErrorMessage: "generation failed",
			},
		},
		{
			name: "Doesn't update deleted DIP status",
			params: &activities.UpdateDIPParams{
				UUID:         dipUUID,
				Status:       enums.DIPStatusStored,
				ObjectKey:    "replacement.zip",
				Retention:    time.Hour,
				ErrorMessage: "retry failed",
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusDeleted,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(-time.Hour),
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusDeleted,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(-time.Hour),
			},
		},
		{
			name: "Preserves deletion after failure",
			params: &activities.UpdateDIPParams{
				UUID:         dipUUID,
				Status:       enums.DIPStatusFailed,
				ObjectKey:    "replacement.zip",
				Retention:    time.Hour,
				ErrorMessage: "retry failed",
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusDeleted,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(-time.Hour),
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusDeleted,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(-time.Hour),
			},
		},
		{
			name: "Deletes an expired DIP",
			params: &activities.UpdateDIPParams{
				UUID:   dipUUID,
				Status: enums.DIPStatusDeleted,
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(-time.Hour),
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusDeleted,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(-time.Hour),
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Ignores deletion when DIP expiration is in the future",
			params: &activities.UpdateDIPParams{
				UUID:   dipUUID,
				Status: enums.DIPStatusDeleted,
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(time.Hour),
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   now.Add(time.Hour),
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Never deletes a DIP with an indefinite expiration",
			params: &activities.UpdateDIPParams{
				UUID:   dipUUID,
				Status: enums.DIPStatusDeleted,
			},
			current: &datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   time.Time{},
				ObjectKey:   "archive.zip",
			},
			wantDIP: datatypes.DIP{
				DBID:        1,
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				Status:      enums.DIPStatusStored,
				CreatedAt:   createdAt,
				StartedAt:   createdAt.Add(time.Second),
				CompletedAt: createdAt.Add(time.Minute),
				ExpiresAt:   time.Time{},
				ObjectKey:   "archive.zip",
			},
		},
		{
			name: "Clears the object key after cleanup of a failed DIP",
			params: &activities.UpdateDIPParams{
				UUID:           dipUUID,
				ClearObjectKey: true,
			},
			current: &datatypes.DIP{
				UUID:        dipUUID,
				Status:      enums.DIPStatusFailed,
				ObjectKey:   "archive.zip",
				CompletedAt: createdAt,
			},
			wantDIP: datatypes.DIP{
				UUID:        dipUUID,
				Status:      enums.DIPStatusFailed,
				ObjectKey:   "",
				CompletedAt: createdAt,
			},
		},
		{
			name: "Clears the object key after cleanup of a deleted DIP",
			params: &activities.UpdateDIPParams{
				UUID:           dipUUID,
				ClearObjectKey: true,
			},
			current: &datatypes.DIP{
				UUID:        dipUUID,
				Status:      enums.DIPStatusDeleted,
				ObjectKey:   "archive.zip",
				CompletedAt: createdAt,
			},
			wantDIP: datatypes.DIP{
				UUID:        dipUUID,
				Status:      enums.DIPStatusDeleted,
				ObjectKey:   "",
				CompletedAt: createdAt,
			},
		},
		{
			name: "Does not clear the object key of a stored DIP",
			params: &activities.UpdateDIPParams{
				UUID:           dipUUID,
				ClearObjectKey: true,
			},
			current: &datatypes.DIP{
				UUID:        dipUUID,
				Status:      enums.DIPStatusStored,
				ObjectKey:   "archive.zip",
				CompletedAt: createdAt,
			},
			wantDIP: datatypes.DIP{
				UUID:        dipUUID,
				Status:      enums.DIPStatusStored,
				ObjectKey:   "archive.zip",
				CompletedAt: createdAt,
			},
		},
		{
			name: "Fails if there is a persistence error",
			params: &activities.UpdateDIPParams{
				UUID:   dipUUID,
				Status: enums.DIPStatusInProgress,
			},
			wantDIP: datatypes.DIP{
				DBID:      1,
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Status:    enums.DIPStatusInProgress,
				CreatedAt: createdAt,
				StartedAt: now,
			},
			err:     errors.New("persistence error"),
			wantErr: "persistence error",
		},
		{
			name: "Fails if the DIP is missing",
			params: &activities.UpdateDIPParams{
				UUID:   dipUUID,
				Status: enums.DIPStatusInProgress,
			},
			wantDIP: datatypes.DIP{
				DBID:      1,
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Status:    enums.DIPStatusInProgress,
				CreatedAt: createdAt,
				StartedAt: now,
			},
			err:          persistence.ErrNotFound,
			wantErr:      "update DIP: not found error",
			nonRetryable: true,
		},
		{
			name: "Fails if completion has no object key",
			params: &activities.UpdateDIPParams{
				UUID:   dipUUID,
				Status: enums.DIPStatusStored,
			},
			wantErr:      `invalid data error: update DIP status to "stored": ObjectKey is required`,
			nonRetryable: true,
		},
		{
			name: "Fails if completion has negative retention",
			params: &activities.UpdateDIPParams{
				UUID:      dipUUID,
				Status:    enums.DIPStatusStored,
				ObjectKey: "archive.zip",
				Retention: -time.Hour,
			},
			wantErr:      `invalid data error: update DIP status to "stored": Retention must not be negative (got -1h0m0s)`,
			nonRetryable: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := persistencefake.NewMockService(gomock.NewController(t))
			svc.EXPECT().
				UpdateDIP(mockutil.Context(), dipUUID, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ uuid.UUID, updater persistence.DIPUpdater) (*datatypes.DIP, error) {
					d := dip
					if tt.current != nil {
						d = *tt.current
					}
					got, err := updater(&d)
					if err != nil {
						return nil, err
					}
					assert.DeepEqual(t, got, &tt.wantDIP, cmpopts.EquateApproxTime(time.Second))
					return &tt.wantDIP, tt.err
				})

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewUpdateDIP(svc).Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.UpdateDIPName},
			)
			enc, err := env.ExecuteActivity(activities.UpdateDIPName, tt.params)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				appErr, ok := errors.AsType[*temporalsdk_temporal.ApplicationError](err)
				assert.Assert(t, ok)
				assert.Equal(t, appErr.NonRetryable(), tt.nonRetryable)
				return
			}
			assert.NilError(t, err)

			var res activities.UpdateDIPResult
			assert.NilError(t, enc.Get(&res))
			assert.DeepEqual(t, res.DIP, tt.wantDIP)
		})
	}
}
