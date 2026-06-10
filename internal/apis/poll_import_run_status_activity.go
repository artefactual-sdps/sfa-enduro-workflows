package apis

import (
	"context"
	"fmt"
	"time"

	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

const PollImportRunStatusActivityName = "poll-apis-import-run-status"

type (
	PollImportRunStatusActivity struct {
		client       Client
		pollInterval time.Duration
	}

	PollImportRunStatusParams struct {
		TaskID string
	}

	PollImportRunStatusResult struct {
		ImportResult gen.ImportResult
	}
)

func NewPollImportRunStatusActivity(client Client, pollInterval time.Duration) *PollImportRunStatusActivity {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	return &PollImportRunStatusActivity{
		client:       client,
		pollInterval: pollInterval,
	}
}

func (a *PollImportRunStatusActivity) Execute(
	ctx context.Context,
	params *PollImportRunStatusParams,
) (*PollImportRunStatusResult, error) {
	h := temporal.StartAutoHeartbeat(ctx)
	defer h.Stop()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			res, err := a.client.APIImporttasksIDStatusGet(ctx, gen.APIImporttasksIDStatusGetParams{
				ID: params.TaskID,
			})
			if err != nil {
				return nil, fmt.Errorf("poll APIS import run status: %w", err)
			}

			switch status := res.(type) {
			case *gen.ImportTaskStatusResponse:
				done, err := importComplete(status)
				if err != nil {
					return nil, err
				}
				if !done {
					continue
				}

				importResult, ok := status.ImportResult.Get()
				if !ok {
					return nil, temporal.NewNonRetryableError(fmt.Errorf(
						"poll APIS import run status: missing import result for completed task %q",
						params.TaskID,
					))
				}

				return &PollImportRunStatusResult{ImportResult: importResult}, nil
			case *gen.APIImporttasksIDStatusGetUnauthorized:
				return nil, temporal.NewNonRetryableError(
					fmt.Errorf("poll APIS import run status: unauthorized"),
				)
			case *gen.APIImporttasksIDStatusGetNotFound:
				return nil, temporal.NewNonRetryableError(fmt.Errorf(
					"poll APIS import run status: task not found: %s",
					problemDetail(status.Detail),
				))
			case *gen.APIImporttasksIDStatusGetInternalServerError:
				return nil, fmt.Errorf(
					"poll APIS import run status: server error: %s",
					problemDetail(status.Detail),
				)
			default:
				return nil, fmt.Errorf("poll APIS import run status: unexpected response")
			}
		}
	}
}

func importComplete(status *gen.ImportTaskStatusResponse) (bool, error) {
	switch status.Status {
	case gen.ImportTaskStatusAnalysiert, gen.ImportTaskStatusWirdImportiert:
		return false, nil
	case gen.ImportTaskStatusImportiert:
		return true, nil
	case gen.ImportTaskStatusNeu, gen.ImportTaskStatusInAnalyse, gen.ImportTaskStatusAbgebrochen:
		return false, temporal.NewNonRetryableError(fmt.Errorf(
			"unexpected APIS import task status during import: %s", status.Status,
		))
	default:
		return false, temporal.NewNonRetryableError(fmt.Errorf(
			"unknown APIS import task status during import: %s", status.Status,
		))
	}
}
