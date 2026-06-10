package apis

import (
	"context"
	"fmt"
	"time"

	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

const PollImportTaskStatusActivityName = "poll-apis-import-task-status"

type (
	PollImportTaskStatusActivity struct {
		client       Client
		pollInterval time.Duration
	}

	PollImportTaskStatusParams struct {
		TaskID string
	}

	PollImportTaskStatusResult struct {
		AnalysisResult gen.AnalysisResult
	}
)

func NewPollImportTaskStatusActivity(client Client, pollInterval time.Duration) *PollImportTaskStatusActivity {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	return &PollImportTaskStatusActivity{
		client:       client,
		pollInterval: pollInterval,
	}
}

// Execute polls APIS until the import task leaves the analysis phase and
// returns the terminal analysis result.
func (a *PollImportTaskStatusActivity) Execute(
	ctx context.Context,
	params *PollImportTaskStatusParams,
) (*PollImportTaskStatusResult, error) {
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
				return nil, fmt.Errorf("poll APIS import task status: %w", err)
			}

			switch status := res.(type) {
			case *gen.ImportTaskStatusResponse:
				done, err := analysisComplete(status)
				if err != nil {
					return nil, err
				}
				if !done {
					continue
				}

				analysisResult, ok := status.AnalysisResult.Get()
				if !ok {
					return nil, temporal.NewNonRetryableError(fmt.Errorf(
						"poll APIS import task status: missing analysis result for completed task %q",
						params.TaskID,
					))
				}

				return &PollImportTaskStatusResult{AnalysisResult: analysisResult}, nil
			case *gen.APIImporttasksIDStatusGetUnauthorized:
				return nil, temporal.NewNonRetryableError(
					fmt.Errorf("poll APIS import task status: unauthorized"),
				)
			case *gen.APIImporttasksIDStatusGetNotFound:
				return nil, temporal.NewNonRetryableError(fmt.Errorf(
					"poll APIS import task status: task not found: %s",
					problemDetail(status.Detail),
				))
			case *gen.APIImporttasksIDStatusGetInternalServerError:
				return nil, fmt.Errorf(
					"poll APIS import task status: server error: %s",
					problemDetail(status.Detail),
				)
			default:
				return nil, temporal.NewNonRetryableError(
					fmt.Errorf("poll APIS import task status: unexpected response"),
				)
			}
		}
	}
}

func analysisComplete(status *gen.ImportTaskStatusResponse) (bool, error) {
	switch status.Status {
	case gen.ImportTaskStatusNeu, gen.ImportTaskStatusInAnalyse:
		return false, nil
	case gen.ImportTaskStatusAnalysiert:
		return true, nil
	case gen.ImportTaskStatusAbgebrochen, gen.ImportTaskStatusWirdImportiert, gen.ImportTaskStatusImportiert:
		return false, temporal.NewNonRetryableError(fmt.Errorf(
			"unexpected APIS import task status: %s", status.Status,
		))
	default:
		return false, temporal.NewNonRetryableError(fmt.Errorf(
			"unknown APIS import task status: %s", status.Status,
		))
	}
}
