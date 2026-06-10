package fformat

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"

	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

// Identifier provides a interface for identifying a file's format.
type Identifier interface {
	// Identify returns a file format identification for the file at path.
	Identify(path string) (*FileFormat, error)

	// Version returns the file format identification software version.
	Version() string
}

// A FileFormat represents a file format.
type FileFormat struct {
	Namespace  string // Namespace of the format identifier (e.g. "PRONOM").
	ID         string // ID is the unique format identifier (e.g. "fmt/40").
	CommonName string // Common name of the format (e.g. "Microsoft Word Document").
	Version    string // Version of the format (e.g. "97-2003").
	MIMEType   string // MIMEType of the format (e.g. "application/msword").
	Basis      string // Basis for identification of the format (e.g. "magic").
	Warning    string // Warning message (if any) from the format identifier.
}

type FileFormats map[string]*FileFormat

func IdentifyFormats(ctx context.Context, identifier Identifier, sip sip.SIP) (FileFormats, error) {
	logger := temporal.GetLogger(ctx)
	formats := make(FileFormats)
	err := filepath.WalkDir(sip.ContentPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return errors.New("context cancelled")
		}

		if d.IsDir() {
			return nil
		}

		ff, err := identifier.Identify(path)
		if err != nil {
			logger.Info("format identification failed", "path", path)
		} else {
			formats[path] = ff
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return formats, nil
}
