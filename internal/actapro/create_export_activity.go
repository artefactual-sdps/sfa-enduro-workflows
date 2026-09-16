package actapro

import (
	"context"
	"fmt"

	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

const CreateExportActivityName = "create-actapro-export"

type (
	CreateExportActivity struct {
		client Client
	}

	CreateExportParams struct {
		DocKey string
	}

	CreateExportResult struct {
		ExportID string
	}
)

func NewCreateExportActivity(client Client) *CreateExportActivity {
	return &CreateExportActivity{client: client}
}

// Execute starts a recursive document export and returns its ID for polling.
func (a *CreateExportActivity) Execute(ctx context.Context, params *CreateExportParams) (*CreateExportResult, error) {
	res, err := a.client.CreateMassOperationExport(ctx, &gen.ExportParamsDTO{
		ScriptId:      "fb104fbe-de3f-419c-82e9-4a0767ea5d48",
		KeysRecursive: gen.NewOptBool(true),
		ExportDocKeys: []string{params.DocKey},
		ExportScriptOptions: gen.ExportScriptOptionsDTO{
			ScriptOptions: gen.NewOptExportScriptOptionsDTOScriptOptions(gen.ExportScriptOptionsDTOScriptOptions{}),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create ACTApro export: %w", err)
	}

	switch t := res.(type) {
	case *gen.MassOperationInfoDTO:
		if t.ID.Value == "" {
			return nil, temporal.NewNonRetryableError(
				fmt.Errorf("create ACTApro export: missing export ID in created response"),
			)
		}
		return &CreateExportResult{ExportID: t.ID.Value}, nil
	case *gen.CreateMassOperationExportBadRequest:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("create ACTApro export: bad request: %s", t.Message.Value),
		)
	case *gen.CreateMassOperationExportForbidden:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("create ACTApro export: forbidden: %s", t.Message.Value),
		)
	case *gen.CreateMassOperationExportNotFound:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("create ACTApro export: not found: %s", t.Message.Value),
		)
	case *gen.CreateMassOperationExportConflict:
		return nil, fmt.Errorf("create ACTApro export: conflict: %s", t.Message.Value)
	case *gen.CreateMassOperationExportLocked:
		return nil, fmt.Errorf("create ACTApro export: locked: %s", t.Message.Value)
	case *gen.CreateMassOperationExportInternalServerError:
		return nil, fmt.Errorf("create ACTApro export: server error: %s", t.Message.Value)
	default:
		return nil, fmt.Errorf("create ACTApro export: unexpected response")
	}
}
