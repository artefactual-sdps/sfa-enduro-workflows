package dips

import (
	"context"
	"errors"

	"goa.design/goa/v3/security"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
)

type Service interface {
	goadips.Service
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
	return goadips.MakeNotImplemented(errors.New("not implemented"))
}

func (svc *svcImpl) Create(ctx context.Context, p *goadips.CreatePayload) (*goadips.CreateResult, error) {
	return nil, goadips.MakeNotImplemented(errors.New("not implemented"))
}

func (svc *svcImpl) Show(ctx context.Context, p *goadips.ShowPayload) (*goadips.ShowResult, error) {
	return nil, goadips.MakeNotImplemented(errors.New("not implemented"))
}
