package actapro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

const DownloadExportActivityName = "download-actapro-export"

type (
	DownloadExportActivity struct {
		client Client
	}

	DownloadExportParams struct {
		ExportID     string
		MetadataPath string
	}

	DownloadExportResult struct{}
)

func NewDownloadExportActivity(client Client) *DownloadExportActivity {
	return &DownloadExportActivity{client: client}
}

// Execute downloads the export binary to the metadata path, creating parent
// directories and overwriting any file left by an earlier attempt.
func (a *DownloadExportActivity) Execute(
	ctx context.Context,
	params *DownloadExportParams,
) (*DownloadExportResult, error) {
	id, err := uuid.Parse(params.ExportID)
	if err != nil {
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("download ACTApro export: invalid export ID: %v", err),
		)
	}
	if params.MetadataPath == "" {
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("download ACTApro export: missing metadata path"),
		)
	}

	h := temporal.StartAutoHeartbeat(ctx)
	defer h.Stop()

	res, err := a.client.GetExportFile(ctx, gen.GetExportFileParams{ID: id})
	if err != nil {
		return nil, fmt.Errorf("download ACTApro export: %v", err)
	}

	var data io.Reader
	switch t := res.(type) {
	case *gen.GetExportFileOKApplicationOctetStream:
		data = t.Data
	case *gen.GetExportFileOKApplicationXML:
		data = t.Data
	case *gen.GetExportFileBadRequest:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("download ACTApro export: bad request: %s", t.Message.Value),
		)
	case *gen.GetExportFileForbidden:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("download ACTApro export: forbidden: %s", t.Message.Value),
		)
	case *gen.GetExportFileNotFound:
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("download ACTApro export: not found: %s", t.Message.Value),
		)
	case *gen.GetExportFileConflict:
		return nil, fmt.Errorf("download ACTApro export: conflict: %s", t.Message.Value)
	case *gen.GetExportFileLocked:
		return nil, fmt.Errorf("download ACTApro export: locked: %s", t.Message.Value)
	case *gen.GetExportFileInternalServerError:
		return nil, fmt.Errorf("download ACTApro export: server error: %s", t.Message.Value)
	default:
		return nil, fmt.Errorf("download ACTApro export: unexpected response")
	}
	if data == nil {
		return nil, temporal.NewNonRetryableError(
			fmt.Errorf("download ACTApro export: missing export binary"),
		)
	}
	if err := os.MkdirAll(filepath.Dir(params.MetadataPath), 0o700); err != nil {
		return nil, fmt.Errorf("download ACTApro export: create directory: %v", err)
	}
	file, err := os.OpenFile(
		params.MetadataPath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o600,
	) // #nosec G304 -- trusted path
	if err != nil {
		return nil, fmt.Errorf("download ACTApro export: create file: %v", err)
	}
	_, copyErr := io.Copy(file, data)
	closeErr := file.Close()
	if copyErr != nil {
		copyErr = fmt.Errorf("download ACTApro export: write file: %v", copyErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("download ACTApro export: close file: %v", closeErr)
	}
	if err := errors.Join(copyErr, closeErr); err != nil {
		return nil, err
	}
	return &DownloadExportResult{}, nil
}
