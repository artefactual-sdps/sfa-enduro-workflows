package apis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ogenhttp "github.com/ogen-go/ogen/http"
	"go.artefactual.dev/tools/temporal"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const (
	CreateImportTaskActivityName             = "create-apis-import-task"
	CreateImportTaskDefaultValuesErrorType   = "APISImportTaskDefaultValuesError"
	CreateImportTaskDefaultValuesErrorDetail = "The uploaded package contains information that didn't allow us to determine the default values."
)

type (
	CreateImportTaskActivity struct {
		client Client
	}

	CreateImportTaskParams struct {
		SIP      sip.SIP
		Username string
	}

	CreateImportTaskResult struct {
		TaskID string
	}
)

func NewCreateImportTaskActivity(client Client) *CreateImportTaskActivity {
	return &CreateImportTaskActivity{client: client}
}

// Execute uploads the SIP metadata.xml file to APIS and returns the created
// import-task identifier for later polling and post-storage work.
func (a *CreateImportTaskActivity) Execute(
	ctx context.Context,
	params *CreateImportTaskParams,
) (*CreateImportTaskResult, error) {
	metadata, err := os.Open(params.SIP.MetadataPath)
	if err != nil {
		return nil, temporal.NewNonRetryableError(fmt.Errorf("open metadata.xml: %v", err))
	}
	defer metadata.Close()

	// SIP types should match between our internal representation and APIS, but
	// validating here keeps bad inputs from being retried pointlessly.
	sipType := gen.SipType(params.SIP.Type.String())
	if err := sipType.Validate(); err != nil {
		return nil, temporal.NewNonRetryableError(fmt.Errorf("invalid SIP type %q: %v", params.SIP.Type, err))
	}

	req := gen.APIImporttasksPostReq{
		File: ogenhttp.MultipartFile{
			Name: filepath.Base(params.SIP.MetadataPath),
			File: metadata,
		},
		SipType:  sipType,
		Username: params.Username,
	}
	res, err := a.client.APIImporttasksPost(ctx, gen.NewOptAPIImporttasksPostReq(req))
	if err != nil {
		return nil, fmt.Errorf("create APIS import task: %v", err)
	}

	switch t := res.(type) {
	case *gen.CreateImportTaskResponse:
		if t.ImportTaskId == "" {
			return nil, temporal.NewNonRetryableError(fmt.Errorf(
				"create APIS import task: missing task ID in created response",
			))
		}
		return &CreateImportTaskResult{TaskID: t.ImportTaskId}, nil
	case *gen.APIImporttasksPostBadRequest:
		detail := problemDetail(t.Detail)
		if strings.Contains(detail, CreateImportTaskDefaultValuesErrorDetail) {
			return nil, temporalsdk_temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("create APIS import task: bad request: %s", detail),
				CreateImportTaskDefaultValuesErrorType,
				nil,
			)
		}
		return nil, temporal.NewNonRetryableError(fmt.Errorf(
			"create APIS import task: bad request: %s",
			detail,
		))
	case *gen.APIImporttasksPostUnsupportedMediaType:
		return nil, temporal.NewNonRetryableError(fmt.Errorf(
			"create APIS import task: unsupported media type: %s",
			problemDetail(t.Detail),
		))
	case *gen.APIImporttasksPostInternalServerError:
		return nil, fmt.Errorf(
			"create APIS import task: server error: %s",
			problemDetail(t.Detail),
		)
	case *gen.APIImporttasksPostUnauthorized:
		return nil, temporal.NewNonRetryableError(fmt.Errorf("create APIS import task: unauthorized"))
	default:
		return nil, temporal.NewNonRetryableError(fmt.Errorf("create APIS import task: unexpected response"))
	}
}
