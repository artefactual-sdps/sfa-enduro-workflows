package client_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

func TestCreateDIP(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, time.August, 31, 8, 30, 0, 0, time.UTC)
	completed := started.Add(time.Minute)

	tests := []struct {
		name    string
		dip     *datatypes.DIP
		want    *datatypes.DIP
		wantErr string
	}{
		{
			name: "Creates a new DIP in the DB",
			dip: &datatypes.DIP{
				DBID:         100,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusInProgress,
				ErrorMessage: "an earlier error",
				CreatedAt:    time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC),
				StartedAt:    started,
				CompletedAt:  completed,
				ObjectKey:    "dips/CH-000001.zip",
			},
			want: &datatypes.DIP{
				DBID:         1,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusInProgress,
				ErrorMessage: "an earlier error",
				CreatedAt:    time.Now(),
				StartedAt:    started,
				CompletedAt:  completed,
				ObjectKey:    "dips/CH-000001.zip",
			},
		},
		{
			name: "Creates a DIP with missing optional fields",
			dip: &datatypes.DIP{
				UUID:   dipUUID,
				DocKey: "CH-000002",
				Status: enums.DIPStatusQueued,
			},
			want: &datatypes.DIP{
				DBID:      1,
				UUID:      dipUUID,
				DocKey:    "CH-000002",
				Status:    enums.DIPStatusQueued,
				CreatedAt: time.Now(),
			},
		},
		{
			name:    "Required field error for missing UUID",
			dip:     &datatypes.DIP{},
			wantErr: "invalid data error: field \"UUID\" is required",
		},
		{
			name:    "Required field error for missing document key",
			dip:     &datatypes.DIP{UUID: dipUUID},
			wantErr: "invalid data error: field \"DocKey\" is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, svc := setUpClient(t, logr.Discard())
			dip := *tt.dip

			err := svc.CreateDIP(t.Context(), &dip)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, &dip, tt.want, cmpopts.EquateApproxTime(time.Second))
		})
	}
}

func TestCreateDIPRejectsInvalidData(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		dips []*datatypes.DIP
	}{
		{
			name: "Invalid status",
			dips: []*datatypes.DIP{{
				UUID:   dipUUID,
				DocKey: "CH-000001",
				Status: enums.DIPStatus("invalid"),
			}},
		},
		{
			name: "Duplicate UUID",
			dips: []*datatypes.DIP{
				{UUID: dipUUID, DocKey: "CH-000001", Status: enums.DIPStatusQueued},
				{UUID: dipUUID, DocKey: "CH-000002", Status: enums.DIPStatusQueued},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, svc := setUpClient(t, logr.Discard())
			for i, dip := range tt.dips {
				err := svc.CreateDIP(t.Context(), dip)
				if i < len(tt.dips)-1 {
					assert.NilError(t, err)
					continue
				}
				assert.Assert(t, errors.Is(err, persistence.ErrNotValid), "%v", err)
			}
		})
	}
}

func TestUpdateDIP(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, time.August, 31, 8, 30, 0, 0, time.UTC)
	started2 := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	completed := started.Add(time.Minute)
	completed2 := started2.Add(time.Minute)
	updatedUUID := uuid.MustParse("f34b96e7-0e7c-4f2e-a608-a3320bbfb8be")

	tests := []struct {
		name    string
		dip     *datatypes.DIP
		id      uuid.UUID
		updater persistence.DIPUpdater
		want    func(createdAt time.Time) *datatypes.DIP
		wantErr string
	}{
		{
			name: "Updates all mutable DIP columns without changing immutable fields",
			dip: &datatypes.DIP{
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusInProgress,
				ErrorMessage: "old error",
				StartedAt:    started,
				CompletedAt:  completed,
				ObjectKey:    "dips/old.zip",
			},
			id: dipUUID,
			updater: func(d *datatypes.DIP) (*datatypes.DIP, error) {
				d.DBID = 100
				d.UUID = updatedUUID
				d.DocKey = "CH-000002"
				d.Status = enums.DIPStatusDone
				d.ErrorMessage = "new error"
				d.CreatedAt = started2
				d.StartedAt = started2
				d.CompletedAt = completed2
				d.ObjectKey = "dips/new.zip"
				return d, nil
			},
			want: func(createdAt time.Time) *datatypes.DIP {
				return &datatypes.DIP{
					DBID:         1,
					UUID:         dipUUID,
					DocKey:       "CH-000001",
					Status:       enums.DIPStatusDone,
					ErrorMessage: "new error",
					CreatedAt:    createdAt,
					StartedAt:    started2,
					CompletedAt:  completed2,
					ObjectKey:    "dips/new.zip",
				}
			},
		},
		{
			name: "Only updates selected columns",
			dip: &datatypes.DIP{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Status:    enums.DIPStatusInProgress,
				StartedAt: started,
				ObjectKey: "dips/original.zip",
			},
			id: dipUUID,
			updater: func(d *datatypes.DIP) (*datatypes.DIP, error) {
				d.Status = enums.DIPStatusDone
				d.CompletedAt = completed
				return d, nil
			},
			want: func(createdAt time.Time) *datatypes.DIP {
				return &datatypes.DIP{
					DBID:        1,
					UUID:        dipUUID,
					DocKey:      "CH-000001",
					Status:      enums.DIPStatusDone,
					CreatedAt:   createdAt,
					StartedAt:   started,
					CompletedAt: completed,
					ObjectKey:   "dips/original.zip",
				}
			},
		},
		{
			name: "Ignores an invalid status",
			dip: &datatypes.DIP{
				UUID:   dipUUID,
				DocKey: "CH-000001",
				Status: enums.DIPStatusQueued,
			},
			id: dipUUID,
			updater: func(d *datatypes.DIP) (*datatypes.DIP, error) {
				d.Status = ""
				d.StartedAt = started
				return d, nil
			},
			want: func(createdAt time.Time) *datatypes.DIP {
				return &datatypes.DIP{
					DBID:      1,
					UUID:      dipUUID,
					DocKey:    "CH-000001",
					Status:    enums.DIPStatusQueued,
					CreatedAt: createdAt,
					StartedAt: started,
				}
			},
		},
		{
			name:    "Errors when the DIP is not found",
			id:      dipUUID,
			wantErr: "not found error: db: dip not found",
		},
		{
			name: "Errors when the updater errors",
			dip: &datatypes.DIP{
				UUID:   dipUUID,
				DocKey: "CH-000001",
				Status: enums.DIPStatusQueued,
			},
			id: dipUUID,
			updater: func(*datatypes.DIP) (*datatypes.DIP, error) {
				return nil, fmt.Errorf("bad input")
			},
			wantErr: "invalid data error: updater error: bad input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, svc := setUpClient(t, logr.Discard())
			var createdAt time.Time
			if tt.dip != nil {
				dip := *tt.dip
				err := svc.CreateDIP(t.Context(), &dip)
				assert.NilError(t, err)
				createdAt = dip.CreatedAt
			}

			dip, err := svc.UpdateDIP(t.Context(), tt.id, tt.updater)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, dip, tt.want(createdAt), cmpopts.EquateApproxTime(time.Second))
		})
	}
}

func TestDeleteDIP(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		id      uuid.UUID
		wantErr string
	}{
		{name: "Deletes a DIP"},
		{
			name:    "Fails to delete a missing DIP",
			id:      uuid.MustParse("c822307a-b224-4c6d-a03e-9bc24345dc2b"),
			wantErr: "not found error: db: dip not found: delete DIP",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, svc := setUpClient(t, logr.Discard())
			dip := &datatypes.DIP{
				UUID:   dipUUID,
				DocKey: "CH-000001",
				Status: enums.DIPStatusQueued,
			}
			assert.NilError(t, svc.CreateDIP(t.Context(), dip))

			id := tt.id
			if id == uuid.Nil {
				id = dipUUID
			}
			err := svc.DeleteDIP(t.Context(), id)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			_, err = svc.ReadDIP(t.Context(), id)
			assert.Error(t, err, "not found error: db: dip not found")
		})
	}
}

func TestReadDIP(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, time.August, 31, 8, 30, 0, 0, time.UTC)
	completed := started.Add(time.Minute)

	for _, tt := range []struct {
		name    string
		id      uuid.UUID
		want    *datatypes.DIP
		wantErr string
	}{
		{
			name: "Reads a DIP",
			id:   dipUUID,
			want: &datatypes.DIP{
				DBID:         1,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusFailed,
				ErrorMessage: "rendering failed",
				CreatedAt:    time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC),
				StartedAt:    started,
				CompletedAt:  completed,
				ObjectKey:    "dips/CH-000001.zip",
			},
		},
		{
			name:    "Fails to read a missing DIP",
			id:      dipUUID,
			wantErr: "not found error: db: dip not found",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entc, svc := setUpClient(t, logr.Discard())
			if tt.want != nil {
				_, err := entc.DIP.Create().
					SetUUID(tt.want.UUID).
					SetDocKey(tt.want.DocKey).
					SetStatus(tt.want.Status).
					SetErrorMessage(tt.want.ErrorMessage).
					SetCreatedAt(tt.want.CreatedAt).
					SetStartedAt(tt.want.StartedAt).
					SetCompletedAt(tt.want.CompletedAt).
					SetObjectKey(tt.want.ObjectKey).
					Save(t.Context())
				assert.NilError(t, err)
			}

			dip, err := svc.ReadDIP(t.Context(), tt.id)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, dip, tt.want, cmpopts.EquateApproxTime(time.Second))
		})
	}
}
