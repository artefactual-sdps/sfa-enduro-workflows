package actapro

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

const GetDocumentActivityName = "get-actapro-document"

type (
	GetDocumentActivity struct {
		client Client
	}

	GetDocumentParams struct {
		DocKey string
	}

	GetDocumentResult struct {
		AIPUUIDs []uuid.UUID
	}
)

func NewGetDocumentActivity(client Client) *GetDocumentActivity {
	return &GetDocumentActivity{client: client}
}

func (a *GetDocumentActivity) Execute(ctx context.Context, params *GetDocumentParams) (*GetDocumentResult, error) {
	res, err := a.client.GetDocument(ctx, gen.GetDocumentParams{
		Key:    params.DocKey,
		Format: gen.NewOptString("json"),
	})
	if err != nil {
		return nil, fmt.Errorf("get ACTApro document: %v", err)
	}

	switch t := res.(type) {
	case *gen.Document:
		result := &GetDocumentResult{AIPUUIDs: []uuid.UUID{}}
		// Block.Fields may contain zero or more AIP_ID_Gp groups mixed with other
		// document fields. Each group may contain multiple AIP_ID fields. Collect
		// only non-empty unique AIP_ID values from these groups, reporting all
		// UUID parsing errors before returning.
		seen := make(map[uuid.UUID]struct{})
		var errs error
		for _, group := range t.Block.Fields {
			if group.Type != "AIP_ID_Gp" {
				continue
			}
			for _, field := range group.Fields {
				if field.Type != "AIP_ID" {
					continue
				}
				id, ok := field.Value.Get()
				if !ok || id == "" {
					continue
				}
				aipUUID, err := uuid.Parse(id)
				if err != nil {
					errs = errors.Join(errs, fmt.Errorf("invalid AIP UUID %q: %v", id, err))
					continue
				}
				if _, ok := seen[aipUUID]; ok {
					continue
				}
				seen[aipUUID] = struct{}{}
				result.AIPUUIDs = append(result.AIPUUIDs, aipUUID)
			}
		}
		if errs != nil {
			return nil, temporal.NewNonRetryableError(fmt.Errorf("get ACTApro document: %v", errs))
		}
		return result, nil
	case *gen.GetDocumentBadRequest:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("get ACTApro document: bad request: %s", t.Message.Value),
		)
	case *gen.GetDocumentForbidden:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("get ACTApro document: forbidden: %s", t.Message.Value),
		)
	case *gen.GetDocumentNotFound:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("get ACTApro document: not found: %s", t.Message.Value),
		)
	case *gen.GetDocumentConflict:
		return nil, fmt.Errorf("get ACTApro document: conflict: %s", t.Message.Value)
	case *gen.GetDocumentLocked:
		return nil, fmt.Errorf("get ACTApro document: locked: %s", t.Message.Value)
	case *gen.GetDocumentInternalServerError:
		return nil, fmt.Errorf("get ACTApro document: server error: %s", t.Message.Value)
	default:
		return nil, fmt.Errorf("get ACTApro document: unexpected response")
	}
}
