package dips_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"go.artefactual.dev/tools/mockutil"
	temporalapi_enums "go.temporal.io/api/enums/v1"
	temporalsdk_client "go.temporal.io/sdk/client"
	temporalsdk_mocks "go.temporal.io/sdk/mocks"
	"go.uber.org/mock/gomock"
	goa "goa.design/goa/v3/pkg"
	"goa.design/goa/v3/security"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/auth"
	authfake "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/auth/fake"
	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	persistencefake "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/fake"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/workflows"
)

func TestBearerAuth(t *testing.T) {
	t.Parallel()

	type test struct {
		name    string
		mock    func(tv *authfake.MockTokenVerifier, claims *auth.Claims)
		claims  *auth.Claims
		logged  string
		wantErr string
	}
	for _, tt := range []test{
		{
			name: "Verifies and adds claims to context",
			mock: func(tv *authfake.MockTokenVerifier, claims *auth.Claims) {
				tv.EXPECT().
					Verify(context.Background(), "abc").
					Return(claims, nil)
			},
			claims: &auth.Claims{
				Email:         "info@artefactual.com",
				EmailVerified: true,
			},
		},
		{
			name: "Fails with unauthorized error",
			mock: func(tv *authfake.MockTokenVerifier, claims *auth.Claims) {
				tv.EXPECT().
					Verify(context.Background(), "abc").
					Return(nil, auth.ErrUnauthorized)
			},
			wantErr: "unauthorized",
		},
		{
			name: "Fails with unauthorized error (logging)",
			mock: func(tv *authfake.MockTokenVerifier, claims *auth.Claims) {
				tv.EXPECT().
					Verify(context.Background(), "abc").
					Return(nil, fmt.Errorf("fail"))
			},
			logged:  `"level"=1 "msg"="failed to verify token" "err"="fail"`,
			wantErr: "unauthorized",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logged string
			logger := funcr.New(
				func(prefix, args string) { logged = args },
				funcr.Options{Verbosity: 1},
			)

			tvMock := authfake.NewMockTokenVerifier(gomock.NewController(t))
			tt.mock(tvMock, tt.claims)
			svc := dips.NewService(logger, nil, tvMock, nil, "")

			ctx, err := svc.BearerAuth(context.Background(), "abc", &security.BearerScheme{})
			assert.Equal(t, logged, tt.logged)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, auth.UserClaimsFromContext(ctx), tt.claims)
		})
	}
}

func TestLivez(t *testing.T) {
	t.Parallel()

	svc := dips.NewService(logr.Discard(), nil, nil, nil, "")

	assert.NilError(t, svc.Livez(t.Context()))
}

func TestCreate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name           string
		docKey         goadips.DocKey
		persistenceErr error
		startErr       error
		cleanupErr     error
		cancelRequest  bool
		wantErr        string
		wantErrName    string
	}{
		{
			name:   "Creates a DIP and starts its workflow",
			docKey: "CH-000001",
		},
		{
			name:           "Does not start a workflow when persistence fails",
			docKey:         "CH-000001",
			persistenceErr: errors.New("persistence error"),
			wantErr:        "create DIP: persistence error",
			wantErrName:    "internal_error",
		},
		{
			name:        "Deletes the DIP when workflow startup fails",
			docKey:      "CH-000001",
			startErr:    errors.New("temporal error"),
			wantErr:     "create DIP: start DIP creation workflow: temporal error",
			wantErrName: "internal_error",
		},
		{
			name:        "Reports both workflow startup and cleanup failures",
			docKey:      "CH-000001",
			startErr:    errors.New("temporal error"),
			cleanupErr:  errors.New("delete DIP error"),
			wantErr:     "create DIP: start DIP creation workflow: temporal error\ndelete DIP error",
			wantErrName: "internal_error",
		},
		{
			name:          "Cleans up after request cancellation",
			docKey:        "CH-000001",
			startErr:      context.Canceled,
			cancelRequest: true,
			wantErr:       "create DIP: start DIP creation workflow: context canceled",
			wantErrName:   "internal_error",
		},
		{
			name:        "Returns a bad request for an empty document key",
			wantErr:     "empty docKey",
			wantErrName: "bad_request",
		},
		{
			name:        "Returns a bad request for a document key longer than 1024 characters",
			docKey:      goadips.DocKey(strings.Repeat("a", 1025)),
			wantErr:     "docKey exceeds 1024 characters",
			wantErrName: "bad_request",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			psvc := persistencefake.NewMockService(gomock.NewController(t))
			tc := temporalsdk_mocks.NewClient(t)
			svc := dips.NewService(logr.Discard(), psvc, nil, tc, "test-dips")
			claims := &auth.Claims{Email: "info@artefactual.com"}
			ctx, cancel := context.WithCancel(auth.WithUserClaims(t.Context(), claims))
			defer cancel()
			var savedDIP datatypes.DIP

			if tt.wantErrName == "bad_request" {
				got, err := svc.Create(ctx, &goadips.CreatePayload{DocKey: tt.docKey})
				assert.Assert(t, got == nil)
				assert.ErrorContains(t, err, tt.wantErr)
				assertServiceErrorName(t, err, tt.wantErrName)
				return
			}

			psvc.EXPECT().CreateDIP(ctx, gomock.Any()).
				DoAndReturn(func(_ context.Context, d *datatypes.DIP) error {
					assert.Assert(t, d.UUID != uuid.Nil)
					assert.Equal(t, d.DocKey, "CH-000001")
					assert.Equal(t, d.Status, enums.DIPStatusQueued)
					d.DBID = 42
					d.CreatedAt = time.Date(2026, time.September, 10, 8, 0, 0, 0, time.UTC)
					savedDIP = *d
					return tt.persistenceErr
				})
			if tt.persistenceErr == nil {
				tc.On("ExecuteWorkflow", mock.Anything, mock.Anything, "create-dip", mock.Anything).
					Run(func(args mock.Arguments) {
						assert.Assert(t, savedDIP.UUID != uuid.Nil)
						opts := args.Get(1).(temporalsdk_client.StartWorkflowOptions)
						assert.Equal(t, opts.ID, "create-dip-"+savedDIP.UUID.String())
						assert.Equal(t, opts.TaskQueue, "test-dips")
						assert.Equal(
							t,
							opts.WorkflowIDReusePolicy,
							temporalapi_enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
						)
						assert.DeepEqual(t, args.Get(3), &workflows.CreateDIPParams{DIP: savedDIP})
						startCtx := args.Get(0).(context.Context)
						deadline, ok := startCtx.Deadline()
						assert.Assert(t, ok)
						assert.Assert(t, time.Until(deadline) > 0 && time.Until(deadline) <= 5*time.Second)
						if tt.cancelRequest {
							cancel()
							assert.ErrorIs(t, startCtx.Err(), context.Canceled)
						}
					}).Return(nil, tt.startErr).Once()
			}
			if tt.startErr != nil {
				psvc.EXPECT().DeleteDIP(mockutil.Context(), gomock.Any()).
					DoAndReturn(func(cleanupCtx context.Context, id uuid.UUID) error {
						assert.Equal(t, id, savedDIP.UUID)
						assert.NilError(t, cleanupCtx.Err())
						assert.DeepEqual(t, auth.UserClaimsFromContext(cleanupCtx), claims)
						deadline, ok := cleanupCtx.Deadline()
						assert.Assert(t, ok)
						assert.Assert(t, time.Until(deadline) > 0 && time.Until(deadline) <= 5*time.Second)
						return tt.cleanupErr
					})
			}

			got, err := svc.Create(ctx, &goadips.CreatePayload{DocKey: tt.docKey})
			if tt.wantErrName != "" {
				assert.Assert(t, got == nil)
				assert.ErrorContains(t, err, tt.wantErr)
				assertServiceErrorName(t, err, tt.wantErrName)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, got, &goadips.CreateResult{ID: goadips.DIPID(savedDIP.UUID.String())})
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
			svc := dips.NewService(logr.Discard(), psvc, nil, nil, "")

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
