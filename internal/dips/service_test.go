package dips_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.artefactual.dev/tools/mockutil"
	"go.uber.org/mock/gomock"
	goa "goa.design/goa/v3/pkg"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips"
	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	persistencefake "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/fake"
)

func TestLivez(t *testing.T) {
	t.Parallel()

	svc := dips.NewService(nil)

	assert.NilError(t, svc.Livez(t.Context()))
}

func TestCreate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		docKey      goadips.DocKey
		mock        func(*testing.T, *persistencefake.MockService, *uuid.UUID)
		wantErr     string
		wantErrName string
	}{
		{
			name:   "Creates a DIP",
			docKey: "CH-000001",
			mock: func(t *testing.T, psvc *persistencefake.MockService, createdID *uuid.UUID) {
				t.Helper()

				psvc.EXPECT().
					CreateDIP(mockutil.Context(), gomock.Any()).
					DoAndReturn(func(_ context.Context, d *datatypes.DIP) error {
						assert.Assert(t, d.UUID != uuid.Nil)
						assert.Equal(t, d.DocKey, "CH-000001")
						assert.Equal(t, d.Status, enums.DIPStatusQueued)
						*createdID = d.UUID
						return nil
					})
			},
		},
		{
			name:   "Returns an internal error when persistence fails",
			docKey: "CH-000001",
			mock: func(t *testing.T, psvc *persistencefake.MockService, _ *uuid.UUID) {
				t.Helper()

				psvc.EXPECT().
					CreateDIP(mockutil.Context(), gomock.Any()).
					DoAndReturn(func(_ context.Context, d *datatypes.DIP) error {
						assert.Assert(t, d.UUID != uuid.Nil)
						assert.Equal(t, d.DocKey, "CH-000001")
						assert.Equal(t, d.Status, enums.DIPStatusQueued)
						return errors.New("persistence error")
					})
			},
			wantErr:     "create DIP: persistence error",
			wantErrName: "internal_error",
		},
		{
			name:        "Returns a bad request for an empty document key",
			mock:        func(*testing.T, *persistencefake.MockService, *uuid.UUID) {},
			wantErr:     "empty docKey",
			wantErrName: "bad_request",
		},
		{
			name:        "Returns a bad request for a document key longer than 1024 characters",
			docKey:      goadips.DocKey(strings.Repeat("a", 1025)),
			mock:        func(*testing.T, *persistencefake.MockService, *uuid.UUID) {},
			wantErr:     "docKey exceeds 1024 characters",
			wantErrName: "bad_request",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			psvc := persistencefake.NewMockService(gomock.NewController(t))
			var createdID uuid.UUID
			tt.mock(t, psvc, &createdID)
			svc := dips.NewService(psvc)

			got, err := svc.Create(t.Context(), &goadips.CreatePayload{DocKey: tt.docKey})
			if tt.wantErrName != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assertServiceErrorName(t, err, tt.wantErrName)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, got, &goadips.CreateResult{ID: goadips.DIPID(createdID.String())})
		})
	}
}

func TestShow(t *testing.T) {
	t.Parallel()

	dipUUID := uuid.MustParse("52fdfc07-2182-454f-963f-5f0f9a621d72")
	createdAt := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	d := &datatypes.DIP{
		DBID:      1,
		UUID:      dipUUID,
		DocKey:    "CH-000001",
		Status:    enums.DIPStatusQueued,
		CreatedAt: createdAt,
	}

	for _, tt := range []struct {
		name    string
		id      goadips.DIPID
		mock    func(*persistencefake.MockService)
		want    *goadips.ShowResult
		wantErr string
	}{
		{
			name: "Shows a DIP",
			id:   goadips.DIPID(dipUUID.String()),
			mock: func(psvc *persistencefake.MockService) {
				psvc.EXPECT().ReadDIP(mockutil.Context(), dipUUID).Return(d, nil)
			},
			want: &goadips.ShowResult{
				ID:        goadips.DIPID(dipUUID.String()),
				DocKey:    "CH-000001",
				Status:    goadips.DIPStatus(enums.DIPStatusQueued),
				CreatedAt: goadips.DateTime(createdAt.Format(time.RFC3339)),
			},
		},
		{
			name:    "Returns a bad request for an invalid DIP ID",
			id:      "invalid",
			mock:    func(*persistencefake.MockService) {},
			wantErr: "bad_request",
		},
		{
			name: "Returns not found when the DIP does not exist",
			id:   goadips.DIPID(dipUUID.String()),
			mock: func(psvc *persistencefake.MockService) {
				psvc.EXPECT().ReadDIP(mockutil.Context(), dipUUID).Return(nil, persistence.ErrNotFound)
			},
			wantErr: "not_found",
		},
		{
			name: "Returns an internal error when persistence fails",
			id:   goadips.DIPID(dipUUID.String()),
			mock: func(psvc *persistencefake.MockService) {
				psvc.EXPECT().ReadDIP(mockutil.Context(), dipUUID).Return(nil, errors.New("persistence error"))
			},
			wantErr: "internal_error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			psvc := persistencefake.NewMockService(gomock.NewController(t))
			tt.mock(psvc)
			svc := dips.NewService(psvc)

			got, err := svc.Show(t.Context(), &goadips.ShowPayload{ID: tt.id})
			if tt.wantErr != "" {
				assertServiceErrorName(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func assertServiceErrorName(t *testing.T, err error, want string) {
	t.Helper()

	var serviceErr *goa.ServiceError
	assert.Assert(t, errors.As(err, &serviceErr))
	assert.Equal(t, serviceErr.Name, want)
}
