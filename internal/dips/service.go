package dips

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	temporalapi_enums "go.temporal.io/api/enums/v1"
	temporalsdk_client "go.temporal.io/sdk/client"
	"goa.design/goa/v3/security"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/auth"
	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/workflows"
)

const maxDocKeyLength = 1024

type Service interface {
	goadips.Service
}

type svcImpl struct {
	logger        logr.Logger
	tokenVerifier auth.TokenVerifier
	psvc          persistence.Service
	tc            temporalsdk_client.Client
	taskQueue     string
}

var _ Service = (*svcImpl)(nil)

func NewService(
	logger logr.Logger,
	psvc persistence.Service,
	tokenVerifier auth.TokenVerifier,
	tc temporalsdk_client.Client,
	taskQueue string,
) *svcImpl {
	return &svcImpl{
		logger:        logger,
		tokenVerifier: tokenVerifier,
		psvc:          psvc,
		tc:            tc,
		taskQueue:     taskQueue,
	}
}

func (svc *svcImpl) BearerAuth(
	ctx context.Context,
	token string,
	schema *security.BearerScheme,
) (context.Context, error) {
	claims, err := svc.tokenVerifier.Verify(ctx, token)
	if err != nil {
		if !errors.Is(err, auth.ErrUnauthorized) {
			svc.logger.V(1).Info("failed to verify token", "err", err)
		}
		return ctx, goadips.MakeUnauthorized(errors.New("unauthorized"))
	}

	ctx = auth.WithUserClaims(ctx, claims)

	return ctx, nil
}

func (svc *svcImpl) Livez(context.Context) error {
	return nil
}

func (svc *svcImpl) Create(ctx context.Context, p *goadips.CreatePayload) (*goadips.CreateResult, error) {
	docKey := string(p.DocKey)
	docKeyLength := utf8.RuneCountInString(docKey)
	if docKeyLength == 0 {
		return nil, goadips.MakeBadRequest(errors.New("empty docKey"))
	}
	if docKeyLength > maxDocKeyLength {
		return nil, goadips.MakeBadRequest(errors.New("docKey exceeds 1024 characters"))
	}

	d := &datatypes.DIP{
		UUID:   uuid.New(),
		DocKey: docKey,
		Status: enums.DIPStatusQueued,
	}

	if err := svc.psvc.CreateDIP(ctx, d); err != nil {
		return nil, goadips.MakeInternalError(fmt.Errorf("create DIP: %v", err))
	}

	if err := svc.startDIPCreationWorkflow(ctx, d); err != nil {
		// Give cleanup a fresh deadline even if the request was canceled.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		err = errors.Join(
			fmt.Errorf("start DIP creation workflow: %v", err),
			svc.psvc.DeleteDIP(cleanupCtx, d.UUID),
		)

		return nil, goadips.MakeInternalError(fmt.Errorf("create DIP: %v", err))
	}

	return &goadips.CreateResult{ID: goadips.DIPID(d.UUID.String())}, nil
}

func (svc *svcImpl) startDIPCreationWorkflow(ctx context.Context, d *datatypes.DIP) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	opts := temporalsdk_client.StartWorkflowOptions{
		ID:                    fmt.Sprintf("%s-%s", workflows.CreateDIPName, d.UUID.String()),
		TaskQueue:             svc.taskQueue,
		WorkflowIDReusePolicy: temporalapi_enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}
	_, err := svc.tc.ExecuteWorkflow(ctx, opts, workflows.CreateDIPName, &workflows.CreateDIPParams{DIP: *d})

	return err
}

func (svc *svcImpl) Show(ctx context.Context, p *goadips.ShowPayload) (*goadips.ShowResult, error) {
	dipUUID, err := uuid.Parse(string(p.ID))
	if err != nil {
		return nil, goadips.MakeBadRequest(errors.New("invalid DIP ID"))
	}

	d, err := svc.psvc.ReadDIP(ctx, dipUUID)
	if errors.Is(err, persistence.ErrNotFound) {
		return nil, goadips.MakeNotFound(errors.New("DIP not found"))
	} else if err != nil {
		return nil, goadips.MakeInternalError(fmt.Errorf("read DIP: %v", err))
	}

	return d.Goa(), nil
}
