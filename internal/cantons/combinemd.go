package cantons

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/antchfx/xmlquery"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fsutil"
)

const CombineMDActivityName = "combine-cantons-metadata-files"

type (
	CombineMDActivity       struct{}
	CombineMDActivityParams struct {
		AreldaPath string
		METSPath   string
		LocalDir   string
	}
	CombineMDActivityResult struct {
		Path string
	}
)

func NewCombineMDActivity() *CombineMDActivity {
	return &CombineMDActivity{}
}

func (a *CombineMDActivity) Execute(
	ctx context.Context,
	params CombineMDActivityParams,
) (*CombineMDActivityResult, error) {
	if !fsutil.FileExists(params.AreldaPath) {
		return nil, fmt.Errorf("missing Arelda file: %s", params.AreldaPath)
	}
	if !fsutil.FileExists(params.METSPath) {
		return nil, fmt.Errorf("missing METS file: %s", params.METSPath)
	}

	name, err := cantonsFilename(params.AreldaPath)
	if err != nil {
		return nil, fmt.Errorf("name Cantons metadata file: %v", err)
	}

	dest := filepath.Join(params.LocalDir, name)

	w, err := os.Create(dest) // #nosec G304 -- generated path.
	if err != nil {
		return nil, fmt.Errorf("create Cantons metadata file: %v", err)
	}
	defer w.Close()

	if err := w.Chmod(os.FileMode(0o644)); err != nil {
		return nil, fmt.Errorf("set Cantons metadata file permissions: %v", err)
	}

	if err = concat(w, params.AreldaPath, params.METSPath); err != nil {
		return nil, fmt.Errorf("concat: %v", err)
	}

	if err = removePaths(params.AreldaPath, params.METSPath); err != nil {
		return nil, fmt.Errorf("removePaths: %v", err)
	}

	return &CombineMDActivityResult{Path: dest}, nil
}

func cantonsFilename(mdpath string) (string, error) {
	id, err := parseAccessionID(mdpath)
	if err != nil {
		return "", fmt.Errorf("get accession number: %v", err)
	}

	id = strings.ReplaceAll(id, "/", "_")

	return fmt.Sprintf("AIS_%s", id), nil
}

func parseAccessionID(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- trusted path.
	if err != nil {
		return "", fmt.Errorf("open metadata file: %v", err)
	}
	defer f.Close()

	sp, err := xmlquery.CreateStreamParser(f, "//paket/ablieferung/ablieferungsnummer")
	if err != nil {
		return "", fmt.Errorf("create XML parser: %v", err)
	}

	n, err := sp.Read()
	if err == io.EOF {
		return "", fmt.Errorf("can't find ablieferungsnummer in %q", filepath.Base(path))
	}
	if err != nil {
		return "", fmt.Errorf("read XML stream: %v", err)
	}
	return n.InnerText(), nil
}

func concat(w io.Writer, paths ...string) error {
	for i := range paths {
		r, err := os.Open(paths[i]) // #nosec G304 -- trusted path.
		if err != nil {
			return fmt.Errorf("read: %v", err)
		}
		defer r.Close()

		if _, err := io.Copy(w, r); err != nil {
			return fmt.Errorf("copy: %v", err)
		}
		_ = r.Close()
	}

	return nil
}

func removePaths(paths ...string) error {
	var err error
	for i := range paths {
		if e := os.Remove(paths[i]); e != nil {
			err = errors.Join(err, fmt.Errorf("remove: %v", e))
		}
	}

	return err
}
