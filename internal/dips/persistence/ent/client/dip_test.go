package client_test

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	entclient "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/client"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db/dip"
)

func TestGetOrCreateDIP(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	created := datatypes.DIP{
		DBID:       1,
		UUID:       dipUUID,
		DocKey:     "CH-000001",
		Status:     enums.DIPStatusQueued,
		CreatedAt:  now,
		WorkflowID: "create-dip-" + dipUUID.String(),
	}
	cached := created
	cached.UUID = uuid.MustParse("c822307a-b224-4c6d-a03e-9bc24345dc2b")
	cached.WorkflowID = "create-dip-" + cached.UUID.String()
	cached.Status = enums.DIPStatusDone
	cached.CreatedAt = now.Add(-time.Hour)
	cached.StartedAt = cached.CreatedAt.Add(time.Minute)
	cached.CompletedAt = cached.StartedAt.Add(time.Minute)
	cached.ObjectKey = "dips/CH-000001.zip"
	cached.ExpiresAt = now.Add(30 * time.Minute)
	renewed := cached
	renewed.ExpiresAt = now.Add(time.Hour)
	indefinite := cached
	indefinite.ExpiresAt = time.Time{}
	queued := cached
	queued.Status = enums.DIPStatusQueued
	queued.ObjectKey = ""
	queued.ExpiresAt = time.Time{}
	pending := queued
	pending.Status = enums.DIPStatusInProgress
	expired := cached
	expired.ExpiresAt = now.Add(-time.Minute)
	missingObject := cached
	missingObject.ObjectKey = ""

	for _, tt := range []struct {
		name     string
		req      persistence.GetOrCreateRequest
		existing []datatypes.DIP
		want     *datatypes.DIP
		update   bool
		wantErr  string
	}{
		{
			name: "Creates a DIP",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			want: &created,
		},
		{
			name: "Creates a DIP without retention",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: 0,
			},
			want: &created,
		},
		{
			name: "Reuses a queued DIP",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			existing: []datatypes.DIP{queued},
			want:     &queued,
		},
		{
			name: "Reuses an in progress DIP when ignoring cache",
			req: persistence.GetOrCreateRequest{
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				IgnoreCache: true,
				Retention:   time.Hour,
			},
			existing: []datatypes.DIP{pending},
			want:     &pending,
		},
		{
			name: "Prefers pending over cached",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			existing: []datatypes.DIP{cached, pending},
			want:     &pending,
		},
		{
			name: "Renews a cached DIP",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			existing: []datatypes.DIP{cached},
			want:     &renewed,
			update:   true,
		},
		{
			name: "Does not shorten retention",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Minute,
			},
			existing: []datatypes.DIP{cached},
			want:     &cached,
		},
		{
			name: "Clears retention",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: 0,
			},
			existing: []datatypes.DIP{cached},
			want:     &indefinite,
			update:   true,
		},
		{
			name: "Preserves indefinite retention",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			existing: []datatypes.DIP{indefinite},
			want:     &indefinite,
		},
		{
			name: "Ignores a cached DIP",
			req: persistence.GetOrCreateRequest{
				UUID:        dipUUID,
				DocKey:      "CH-000001",
				IgnoreCache: true,
				Retention:   time.Hour,
			},
			existing: []datatypes.DIP{cached},
			want:     &created,
		},
		{
			name: "Skips an expired DIP",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			existing: []datatypes.DIP{expired},
			want:     &created,
		},
		{
			name: "Skips a DIP without an object",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			existing: []datatypes.DIP{missingObject},
			want:     &created,
		},
		{
			name: "Falls back to a usable cached DIP",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: time.Minute,
			},
			existing: []datatypes.DIP{expired, cached},
			want:     &cached,
		},
		{
			name: "Missing UUID",
			req: persistence.GetOrCreateRequest{
				DocKey:    "CH-000001",
				Retention: time.Hour,
			},
			wantErr: `invalid data error: field "UUID" is required`,
		},
		{
			name: "Missing DocKey",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				Retention: time.Hour,
			},
			wantErr: `invalid data error: field "DocKey" is required`,
		},
		{
			name: "Negative retention",
			req: persistence.GetOrCreateRequest{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Retention: -time.Second,
			},
			wantErr: "invalid data error: retention must not be negative",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// SQLite cannot execute FOR UPDATE.
			sqlDB, mock, err := sqlmock.New()
			assert.NilError(t, err)
			entc := db.NewClient(db.Driver(entsql.OpenDB(dialect.MySQL, sqlDB)))
			t.Cleanup(func() { entc.Close() })
			svc := entclient.New(logr.Discard(), entc)
			rows := func(ds ...datatypes.DIP) *sqlmock.Rows {
				r := sqlmock.NewRows(dip.Columns)
				for _, d := range ds {
					r.AddRow(d.DBID, d.UUID.String(), d.DocKey, d.Status.String(), d.ErrorMessage,
						d.CreatedAt, d.StartedAt, d.CompletedAt, d.ExpiresAt, d.ObjectKey, d.WorkflowID)
				}
				return r
			}
			if tt.wantErr == "" {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT .* FROM `dip` WHERE .* ORDER BY `dip`.`created_at` DESC FOR UPDATE").
					WithArgs(tt.req.DocKey, enums.DIPStatusQueued.String(), enums.DIPStatusInProgress.String(), enums.DIPStatusDone.String()).
					WillReturnRows(rows(tt.existing...))
				if tt.want.UUID == tt.req.UUID {
					mock.ExpectExec("INSERT INTO `dip`").
						WithArgs(tt.req.UUID.String(), tt.req.DocKey, enums.DIPStatusQueued.String(), approximateTime{now}, created.WorkflowID).
						WillReturnResult(sqlmock.NewResult(1, 1))
				} else if tt.update {
					if tt.want.ExpiresAt.IsZero() {
						mock.ExpectExec("UPDATE `dip` SET `expires_at` = NULL WHERE `id` = ").WithArgs(tt.want.DBID).
							WillReturnResult(sqlmock.NewResult(0, 1))
					} else {
						mock.ExpectExec("UPDATE `dip` SET `expires_at` = .* WHERE `id` = ").
							WithArgs(approximateTime{tt.want.ExpiresAt}, tt.want.DBID).
							WillReturnResult(sqlmock.NewResult(0, 1))
					}
					mock.ExpectQuery("SELECT .* FROM `dip` WHERE `id` = ").WithArgs(tt.want.DBID).
						WillReturnRows(rows(*tt.want))
				}
				mock.ExpectCommit()
			}

			d, err := svc.GetOrCreateDIP(t.Context(), tt.req)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				assert.Assert(t, d == nil)
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, d, tt.want, cmpopts.EquateApproxTime(time.Second))
			}
			assert.NilError(t, mock.ExpectationsWereMet())
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
		set     string
		args    []driver.Value
	}{
		{
			name: "Updates all mutable DIP columns without changing immutable fields",
			set:  "`status` = ?, `error_message` = ?, `started_at` = ?, `completed_at` = ?, `expires_at` = ?, `object_key` = ?",
			args: []driver.Value{
				"done",
				"new error",
				started2,
				completed2,
				completed2.Add(time.Hour),
				"dips/new.zip",
				1,
			},
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
				d.ExpiresAt = completed2.Add(time.Hour)
				d.ObjectKey = "dips/new.zip"
				d.WorkflowID = "changed"
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
					ExpiresAt:    completed2.Add(time.Hour),
					ObjectKey:    "dips/new.zip",
				}
			},
		},
		{
			name: "Only updates selected columns",
			set:  "`status` = ?, `started_at` = ?, `completed_at` = ?, `object_key` = ?",
			args: []driver.Value{"done", started, completed, "dips/original.zip", 1},
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
			set:  "`object_key` = NULL, `started_at` = ?",
			args: []driver.Value{started, 1},
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
			name: "Clears the object key when deleted and preserves zero-valued fields",
			dip: &datatypes.DIP{
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusDone,
				ErrorMessage: "old error",
				StartedAt:    started,
				CompletedAt:  completed,
				ExpiresAt:    completed.Add(time.Hour),
				ObjectKey:    "dips/original.zip",
			},
			id: dipUUID,
			updater: func(d *datatypes.DIP) (*datatypes.DIP, error) {
				return &datatypes.DIP{Status: enums.DIPStatusDeleted}, nil
			},
			set:  "`object_key` = NULL, `status` = ?",
			args: []driver.Value{"deleted", 1},
			want: func(createdAt time.Time) *datatypes.DIP {
				return &datatypes.DIP{
					DBID:   1,
					UUID:   dipUUID,
					DocKey: "CH-000001",

					Status:       enums.DIPStatusDeleted,
					ErrorMessage: "old error",
					CreatedAt:    createdAt,

					StartedAt:   started,
					CompletedAt: completed,
					ExpiresAt:   completed.Add(time.Hour),
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

			// SQLite cannot execute FOR UPDATE.
			sqlDB, mock, err := sqlmock.New()
			assert.NilError(t, err)
			entc := db.NewClient(db.Driver(entsql.OpenDB(dialect.MySQL, sqlDB)))
			t.Cleanup(func() { entc.Close() })
			svc := entclient.New(logr.Discard(), entc)
			createdAt := started.Add(-time.Hour)
			workflowID := "create-dip-" + dipUUID.String()
			rows := func(d *datatypes.DIP) *sqlmock.Rows {
				r := sqlmock.NewRows(dip.Columns)
				if d != nil {
					r.AddRow(1, d.UUID.String(), d.DocKey, d.Status.String(), d.ErrorMessage,
						createdAt, d.StartedAt, d.CompletedAt, d.ExpiresAt, d.ObjectKey, workflowID)
				}
				return r
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `dip` WHERE .* FOR UPDATE").WithArgs(tt.id.String()).
				WillReturnRows(rows(tt.dip))
			var want *datatypes.DIP
			if tt.wantErr != "" {
				mock.ExpectRollback()
			} else {
				want = tt.want(createdAt)
				want.WorkflowID = workflowID
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `dip` SET " + tt.set + " WHERE `id` = ?")).
					WithArgs(tt.args...).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery("SELECT .* FROM `dip` WHERE `id` = ").WithArgs(1).
					WillReturnRows(rows(want))
				mock.ExpectCommit()
			}

			dip, err := svc.UpdateDIP(t.Context(), tt.id, tt.updater)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				assert.Assert(t, dip == nil)
				assert.NilError(t, mock.ExpectationsWereMet())
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, dip, want, cmpopts.EquateApproxTime(time.Second))
			assert.NilError(t, mock.ExpectationsWereMet())
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
		{
			name: "Deletes a DIP",
		},
		{
			name:    "Fails to delete a missing DIP",
			id:      uuid.MustParse("c822307a-b224-4c6d-a03e-9bc24345dc2b"),
			wantErr: "not found error: db: dip not found: delete DIP",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entc, svc := setUpClient(t, logr.Discard())
			_, err := entc.DIP.Create().
				SetUUID(dipUUID).
				SetDocKey("CH-000001").
				SetStatus(enums.DIPStatusQueued).
				SetCreatedAt(time.Now()).
				SetWorkflowID("create-dip-" + dipUUID.String()).
				Save(t.Context())
			assert.NilError(t, err)

			id := tt.id
			if id == uuid.Nil {
				id = dipUUID
			}
			err = svc.DeleteDIP(t.Context(), id)
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
				WorkflowID:   "create-dip-" + dipUUID.String(),
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
					SetWorkflowID(tt.want.WorkflowID).
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
			assert.Assert(t, dip.ExpiresAt.IsZero())
		})
	}
}

// approximateTime allows for the elapsed time between test setup and the call.
type approximateTime struct{ want time.Time }

func (a approximateTime) Match(value driver.Value) bool {
	actual, ok := value.(time.Time)
	return ok && actual.Sub(a.want).Abs() < time.Second
}
