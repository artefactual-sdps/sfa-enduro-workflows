package apis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ogenhttp "github.com/ogen-go/ogen/http"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

const CreateImportRunActivityName = "create-apis-import-run"

type (
	CreateImportRunActivity struct {
		client Client
	}

	CreateImportRunParams struct {
		TaskID          string
		METSPath        string
		ImportBehaviour gen.ImportBehaviourType
		Username        string
	}

	CreateImportRunResult struct {
		RunID string
	}
)

func NewCreateImportRunActivity(client Client) *CreateImportRunActivity {
	return &CreateImportRunActivity{client: client}
}

func (a *CreateImportRunActivity) Execute(
	ctx context.Context,
	params *CreateImportRunParams,
) (*CreateImportRunResult, error) {
	mets, err := os.Open(params.METSPath)
	if err != nil {
		return nil, temporal.NewNonRetryableError(fmt.Errorf("open METS.xml: %v", err))
	}
	defer mets.Close()

	req := gen.APIImporttasksIDImportrunsPostReq{
		File: ogenhttp.MultipartFile{
			Name: filepath.Base(params.METSPath),
			File: mets,
		},
		ImportBehaviour: gen.NewOptImportBehaviourType(params.ImportBehaviour),
		Username:        params.Username,
	}
	res, err := a.client.APIImporttasksIDImportrunsPost(
		ctx,
		gen.NewOptAPIImporttasksIDImportrunsPostReq(req),
		gen.APIImporttasksIDImportrunsPostParams{ID: params.TaskID},
	)
	if err != nil {
		return nil, fmt.Errorf("create APIS import run: %v", err)
	}

	switch t := res.(type) {
	case *gen.CreateImportRunResponse:
		if t.ImportRunId == "" {
			return nil, temporal.NewNonRetryableError(fmt.Errorf(
				"create APIS import run: missing run ID in created response",
			))
		}
		return &CreateImportRunResult{RunID: t.ImportRunId}, nil
	case *gen.APIImporttasksIDImportrunsPostBadRequest:
		return nil, temporal.NewNonRetryableError(fmt.Errorf(
			"create APIS import run: bad request: %s",
			problemDetail(t.Detail),
		))
	case *gen.APIImporttasksIDImportrunsPostUnauthorized:
		return nil, temporal.NewNonRetryableError(fmt.Errorf("create APIS import run: unauthorized"))
	case *gen.APIImporttasksIDImportrunsPostNotFound:
		return nil, temporal.NewNonRetryableError(fmt.Errorf(
			"create APIS import run: task not found: %s",
			problemDetail(t.Detail),
		))
	case *gen.APIImporttasksIDImportrunsPostUnsupportedMediaType:
		return nil, temporal.NewNonRetryableError(fmt.Errorf(
			"create APIS import run: unsupported media type: %s",
			problemDetail(t.Detail),
		))
	default:
		return nil, fmt.Errorf("create APIS import run: unexpected response")
	}
}
