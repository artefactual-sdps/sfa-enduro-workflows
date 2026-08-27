package dips

import (
	"context"
	"errors"

	"goa.design/goa/v3/security"

	di_ps "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
)

type Service interface {
	di_ps.Service
}

type svcImpl struct{}

var _ Service = (*svcImpl)(nil)

func NewService() *svcImpl {
	return &svcImpl{}
}

func (svc *svcImpl) BearerAuth(
	ctx context.Context,
	token string,
	schema *security.BearerScheme,
) (context.Context, error) {
	return ctx, nil
}

func (svc *svcImpl) Livez(context.Context) error {
	return di_ps.MakeNotImplemented(errors.New("not implemented"))
}

func (svc *svcImpl) Create(ctx context.Context, p *di_ps.CreatePayload) (*di_ps.CreateResult, error) {
	return nil, di_ps.MakeNotImplemented(errors.New("not implemented"))
}

func (svc *svcImpl) Show(ctx context.Context, p *di_ps.ShowPayload) (*di_ps.ShowResult, error) {
	return nil, di_ps.MakeNotImplemented(errors.New("not implemented"))
}
