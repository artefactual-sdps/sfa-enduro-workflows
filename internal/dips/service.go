package dips

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"goa.design/goa/v3/security"

	di_ps "github.com/artefactual-sdps/preprocessing-sfa/internal/dips/api/gen/di_ps"
)

type Service interface {
	di_ps.Service
}

type svcImpl struct {
	logger logr.Logger
}

var _ Service = (*svcImpl)(nil)

func NewService(logger logr.Logger) *svcImpl {
	return &svcImpl{logger: logger}
}

func (svc *svcImpl) BearerAuth(
	ctx context.Context,
	token string,
	schema *security.BearerScheme,
) (context.Context, error) {
	svc.logger.Info("Authenticating request", "token", token)
	if token == "unauthorized" {
		return ctx, &di_ps.UnauthorizedProblem{
			Type:   "about:blank",
			Title:  "Unauthorized",
			Status: 401,
			Detail: "The bearer token is missing or invalid.",
		}
	}

	return ctx, nil
}

func (svc *svcImpl) Create(ctx context.Context, p *di_ps.CreatePayload) (*di_ps.CreateResult, error) {
	svc.logger.Info("DIP creation requested", "docKey", p.DocKey)

	if p.DocKey == "" {
		return nil, &di_ps.BadRequestProblem{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: 400,
			Detail: "The request parameters are invalid, docKey field is empty.",
		}
	}

	if p.DocKey == "error-doc-key" {
		err := errors.New("fake database error")
		return nil, svc.internalProblem(err, "Failed to create DIP in the database")
	}

	return &di_ps.CreateResult{ID: di_ps.DIPID(uuid.New().String())}, nil
}

func (svc *svcImpl) Show(ctx context.Context, p *di_ps.ShowPayload) (*di_ps.ShowResult, error) {
	svc.logger.Info("DIP details requested", "id", p.ID)

	if p.ID == "e8d32bd5-faa4-4ce1-bb50-55d9c28b306d" {
		return nil, &di_ps.NotFoundProblem{
			Type:   "about:blank",
			Title:  "Not Found",
			Status: 404,
			Detail: "The requested DIP was not found.",
		}
	}

	if p.ID == "52fdfc07-2182-454f-963f-5f0f9a621d72" {
		err := errors.New("fake database error")
		return nil, svc.internalProblem(err, "Failed to retrieve DIP details from the database")
	}

	return &di_ps.ShowResult{
		ID:        p.ID,
		DocKey:    "example-doc-key",
		Status:    "queued",
		CreatedAt: "2024-01-01T00:00:00Z",
	}, nil
}

func (svc *svcImpl) internalProblem(err error, detail string) *di_ps.InternalServerErrorProblem {
	svc.logger.Error(err, "Internal problem occurred", "detail", detail)

	return &di_ps.InternalServerErrorProblem{
		Type:   "about:blank",
		Title:  "Internal Server Error",
		Status: 500,
		Detail: detail,
	}
}
