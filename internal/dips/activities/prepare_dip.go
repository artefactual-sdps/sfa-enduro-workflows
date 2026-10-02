package activities

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.artefactual.dev/tools/fsutil"
)

const PrepareDIPName = "prepare-dip"

type PrepareDIPParams struct {
	DIPPath      string
	MetadataPath string
	XSDDir       string
}

type PrepareDIPResult struct{}

type PrepareDIP struct{}

func NewPrepareDIP() *PrepareDIP {
	return &PrepareDIP{}
}

// Execute creates the DIP header directory with its schemas and exported metadata.
func (a *PrepareDIP) Execute(ctx context.Context, params *PrepareDIPParams) (*PrepareDIPResult, error) {
	headerPath := filepath.Join(params.DIPPath, "header")
	xsdPath := filepath.Join(headerPath, "xsd")
	if err := os.MkdirAll(xsdPath, 0o700); err != nil {
		return nil, fmt.Errorf("create DIP header: %v", err)
	}

	entries, err := os.ReadDir(params.XSDDir)
	if err != nil {
		return nil, fmt.Errorf("read DIP schemas: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".xsd" {
			continue
		}
		err := copyDIPFile(filepath.Join(params.XSDDir, entry.Name()), filepath.Join(xsdPath, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("copy DIP schema %s: %v", entry.Name(), err)
		}
	}

	if err := fsutil.Move(params.MetadataPath, filepath.Join(headerPath, "metadata.xml")); err != nil {
		return nil, fmt.Errorf("move DIP metadata: %v", err)
	}

	return &PrepareDIPResult{}, nil
}

func copyDIPFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
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
