package amss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"go.artefactual.dev/ssclient"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/checksum"
)

const FetchActivityName = "fetch-aip-file"

type (
	FetchActivityParams struct {
		AIPUUID      uuid.UUID
		RelativePath string
		Destination  string
		// Validate the downloaded file when both checksum fields are set.
		Checksum          string
		ChecksumAlgorithm string
	}
	FetchActivityResult struct{}
	FetchActivity       struct {
		packages *ssclient.PackagesService
	}
)

func NewFetchActivity(packages *ssclient.PackagesService) *FetchActivity {
	return &FetchActivity{packages: packages}
}

// Execute downloads a file, overwriting any file left by an earlier attempt,
// and optionally validates its checksum.
func (a *FetchActivity) Execute(ctx context.Context, params *FetchActivityParams) (*FetchActivityResult, error) {
	h := temporal.StartAutoHeartbeat(ctx)
	defer h.Stop()

	if err := os.MkdirAll(filepath.Dir(params.Destination), 0o700); err != nil {
		return nil, fmt.Errorf("FetchActivity: create directory: %w", err)
	}

	stream, err := a.packages.DownloadFile(ctx, params.AIPUUID, params.RelativePath)
	if err != nil {
		return nil, fmt.Errorf("FetchActivity: download file: %w", err)
	}
	if stream == nil || stream.Body == nil {
		return nil, errors.New("FetchActivity: download file: empty response body")
	}
	defer stream.Body.Close()

	file, err := os.OpenFile(params.Destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("FetchActivity: create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, stream.Body); err != nil {
		return nil, fmt.Errorf("FetchActivity: write file: %w", err)
	}

	if params.Checksum != "" && params.ChecksumAlgorithm != "" {
		hash, err := checksum.GenerateHash(params.Destination, params.ChecksumAlgorithm)
		if err != nil {
			return nil, fmt.Errorf("FetchActivity: validate checksum: %w", err)
		}
		if hash != strings.ToLower(params.Checksum) {
			return nil, fmt.Errorf("FetchActivity: checksum mismatch (expected: %q, got: %q)", params.Checksum, hash)
		}
	}

	return &FetchActivityResult{}, nil
}
