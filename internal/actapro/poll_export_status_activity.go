package actapro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

const (
	PollExportStatusActivityName = "poll-actapro-export-status"
	ExportStatusCompleted        = "COMPLETED"
	ExportStatusFailed           = "FAILED"
	ExportStatusCanceled         = "CANCELED"
)

type (
	PollExportStatusActivity struct {
		client       Client
		pollInterval time.Duration
	}

	PollExportStatusParams struct {
		ExportID string
	}

	PollExportStatusResult struct {
		Status string
		Logs   string
	}
)

func NewPollExportStatusActivity(client Client, pollInterval time.Duration) *PollExportStatusActivity {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	return &PollExportStatusActivity{client: client, pollInterval: pollInterval}
}

// Execute polls ACTApro until the export reaches a final status. Export failures
// are returned with formatted logs in the result; request failures are activity errors.
func (a *PollExportStatusActivity) Execute(
	ctx context.Context,
	params *PollExportStatusParams,
) (*PollExportStatusResult, error) {
	id, err := uuid.Parse(params.ExportID)
	if err != nil {
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("poll ACTApro export status: invalid export ID: %w", err),
		)
	}

	h := temporal.StartAutoHeartbeat(ctx)
	defer h.Stop()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			res, err := a.client.GetMassOperationInfo(ctx, gen.GetMassOperationInfoParams{ID: id})
			if err != nil {
				return nil, fmt.Errorf("poll ACTApro export status: %w", err)
			}

			switch t := res.(type) {
			case *gen.MassOperationInfoDTO:
				if t.Status.Value == "" {
					return nil, temporal.NewNonRetryableError(
						fmt.Errorf("poll ACTApro export status: missing status"),
					)
				}
				switch status := strings.ToUpper(t.Status.Value); status {
				case ExportStatusCompleted:
					return &PollExportStatusResult{Status: status}, nil
				case ExportStatusFailed, ExportStatusCanceled:
					logs, err := a.exportLogs(ctx, id)
					if err != nil {
						return nil, err
					}
					return &PollExportStatusResult{Status: status, Logs: formatExportLogs(logs)}, nil
				}
			case *gen.GetMassOperationInfoBadRequest:
				return nil, temporal.NewNonRetryableError(
					fmt.Errorf("poll ACTApro export status: bad request: %s", t.Message.Value),
				)
			case *gen.GetMassOperationInfoForbidden:
				return nil, temporal.NewNonRetryableError(
					fmt.Errorf("poll ACTApro export status: forbidden: %s", t.Message.Value),
				)
			case *gen.GetMassOperationInfoNotFound:
				return nil, temporal.NewNonRetryableError(
					fmt.Errorf("poll ACTApro export status: not found: %s", t.Message.Value),
				)
			case *gen.GetMassOperationInfoConflict:
				return nil, fmt.Errorf("poll ACTApro export status: conflict: %s", t.Message.Value)
			case *gen.GetMassOperationInfoLocked:
				return nil, fmt.Errorf("poll ACTApro export status: locked: %s", t.Message.Value)
			case *gen.GetMassOperationInfoInternalServerError:
				return nil, fmt.Errorf("poll ACTApro export status: server error: %s", t.Message.Value)
			default:
				return nil, fmt.Errorf("poll ACTApro export status: unexpected response")
			}
		}
	}
}

func (a *PollExportStatusActivity) exportLogs(ctx context.Context, id uuid.UUID) ([]gen.MassOperationLogDTO, error) {
	res, err := a.client.GetMassOperationLogs(ctx, gen.GetMassOperationLogsParams{ID: id})
	if err != nil {
		return nil, fmt.Errorf("get ACTApro export logs: %w", err)
	}

	switch t := res.(type) {
	case *gen.GetMassOperationLogsOKApplicationJSON:
		return []gen.MassOperationLogDTO(*t), nil
	case *gen.GetMassOperationLogsBadRequest:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("get ACTApro export logs: bad request: %s", t.Message.Value),
		)
	case *gen.GetMassOperationLogsForbidden:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("get ACTApro export logs: forbidden: %s", t.Message.Value),
		)
	case *gen.GetMassOperationLogsNotFound:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("get ACTApro export logs: not found: %s", t.Message.Value),
		)
	case *gen.GetMassOperationLogsConflict:
		return nil, fmt.Errorf("get ACTApro export logs: conflict: %s", t.Message.Value)
	case *gen.GetMassOperationLogsLocked:
		return nil, fmt.Errorf("get ACTApro export logs: locked: %s", t.Message.Value)
	case *gen.GetMassOperationLogsInternalServerError:
		return nil, fmt.Errorf("get ACTApro export logs: server error: %s", t.Message.Value)
	default:
		return nil, fmt.Errorf("get ACTApro export logs: unexpected response")
	}
}

func formatExportLogs(logs []gen.MassOperationLogDTO) string {
	entries := make([]string, 0, len(logs))
	for _, entry := range logs {
		var prefix []string
		if status := entry.Status.Or(""); status != "" {
			prefix = append(prefix, "["+string(status)+"]")
		}
		if docKey := entry.DocKey.Or(""); docKey != "" {
			prefix = append(prefix, docKey)
		}
		line := strings.Join(prefix, " ")
		if message := entry.Message.Or(""); message != "" {
			if line != "" {
				line += ": "
			}
			line += message
		}
		if line != "" {
			entries = append(entries, "- "+line)
		}
	}
	return strings.Join(entries, "\n")
}
