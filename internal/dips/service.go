package dips

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"goa.design/goa/v3/security"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

const maxDocKeyLength = 1024

type Service interface {
	goadips.Service
}

type svcImpl struct {
	psvc persistence.Service
}

var _ Service = (*svcImpl)(nil)

func NewService(psvc persistence.Service) *svcImpl {
	return &svcImpl{psvc: psvc}
}

func (svc *svcImpl) BearerAuth(
	ctx context.Context,
	token string,
	schema *security.BearerScheme,
) (context.Context, error) {
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

	// TODO: Trigger the DIP creation workflow.
	// if err := svc.startDIPCreationWorkflow(ctx, d); err != nil {
	// 	cleanupErr := svc.psvc.DeleteDIP(ctx, d.UUID)
	// 	err = errors.Join(fmt.Errorf("start DIP creation workflow: %v", err), cleanupErr)
	// 	return nil, goadips.MakeInternalError(fmt.Errorf("create DIP: %v", err))
	// }

	return &goadips.CreateResult{ID: goadips.DIPID(d.UUID.String())}, nil
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
