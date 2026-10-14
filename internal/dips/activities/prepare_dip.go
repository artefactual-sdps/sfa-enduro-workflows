package activities

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.artefactual.dev/tools/fsutil"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/checksum"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
)

const PrepareDIPName = "prepare-dip"

type PrepareDIPParams struct {
	DIPPath      string
	MetadataPath string
	XSDDir       string
	HeaderFiles  []*datatypes.File
}

type PrepareDIPResult struct{}

type PrepareDIP struct{}

func NewPrepareDIP() *PrepareDIP {
	return &PrepareDIP{}
}

// Execute copies the declared header schemas and validates their destination
// checksums, rejects unexpected header files, then moves the exported metadata
// into the DIP header directory.
func (a *PrepareDIP) Execute(ctx context.Context, params *PrepareDIPParams) (*PrepareDIPResult, error) {
	headerPath := filepath.Join(params.DIPPath, "header")
	if err := os.MkdirAll(headerPath, 0o700); err != nil {
		return nil, fmt.Errorf("create DIP header: %v", err)
	}

	var failures error
	for _, file := range params.HeaderFiles {
		relPath, ok := strings.CutPrefix(file.DIPPath, "header/xsd/")
		if !ok {
			failures = errors.Join(failures, fmt.Errorf("unexpected header file: %s", file.DIPPath))
			continue
		}

		src := filepath.Join(params.XSDDir, relPath)
		dst := filepath.Join(params.DIPPath, file.DIPPath)
		if err := copyDIPFile(src, dst); err != nil {
			failures = errors.Join(failures, fmt.Errorf("copy DIP schema %q: %v", file.DIPPath, err))
			continue
		}

		hash, err := checksum.GenerateHash(dst, file.ChecksumAlgorithm)
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("validate DIP schema checksum %q: %v", file.DIPPath, err))
			continue
		}
		if hash != strings.ToLower(file.Checksum) {
			failures = errors.Join(failures, fmt.Errorf(
				"validate DIP schema checksum %q: checksum mismatch (expected: %q, got: %q)",
				file.DIPPath, file.Checksum, hash,
			))
		}
	}
	if failures != nil {
		return nil, failures
	}

	if err := fsutil.Move(params.MetadataPath, filepath.Join(headerPath, "metadata.xml")); err != nil {
		return nil, fmt.Errorf("move DIP metadata: %v", err)
	}

	return &PrepareDIPResult{}, nil
}

func copyDIPFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}

	source, err := os.Open(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("source file not found")
		}
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	_, err = io.Copy(destination, source)

	return errors.Join(err, destination.Close())
}
