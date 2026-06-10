package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/identifiers"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/manifest"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/pips"
)

const WriteIdentifierFileName = "write-identifier-file"

type (
	WriteIdentifierFileActivity struct{}
	WriteIdentifierFileParams   struct {
		PIP pips.PIP
	}
	WriteIdentifierFileResult struct {
		Path string
	}
)

func NewWriteIdentifierFile() *WriteIdentifierFileActivity {
	return &WriteIdentifierFileActivity{}
}

func (a *WriteIdentifierFileActivity) Execute(
	ctx context.Context,
	params *WriteIdentifierFileParams,
) (*WriteIdentifierFileResult, error) {
	path, err := a.write(params.PIP)
	if err != nil {
		return nil, fmt.Errorf("write identifier file: %v", err)
	}

	return &WriteIdentifierFileResult{Path: path}, nil
}

func (a *WriteIdentifierFileActivity) write(pip pips.PIP) (string, error) {
	r, err := os.Open(pip.ManifestPath)
	if err != nil {
		return "", fmt.Errorf("open manifest: %v", err)
	}
	defer r.Close()

	m, err := manifest.Parse(r)
	if err != nil {
		return "", err
	}

	ids, err := identifiers.FromManifest(m)
	if err != nil {
		return "", fmt.Errorf("get manifest identifiers: %v", err)
	}

	b, err := json.MarshalIndent(pipIdentifiers(pip, ids), "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal identifiers: %v", err)
	}

	path := filepath.Join(pip.Path, "metadata", "identifiers.json")
	if err := os.WriteFile(path, b, os.FileMode(0o644)); err != nil {
		return "", fmt.Errorf("write identifiers.json: %v", err)
	}

	return path, nil
}

// pipIdentifiers takes a list of manifest file ids and converts the manifest
// file paths to the restructured PIP file paths. Any files that are included
// in the manifest but are not included in the PIP (e.g. XSD files) are removed
// from the returned file list.
func pipIdentifiers(pip pips.PIP, ids []identifiers.File) []identifiers.File {
	r := make([]identifiers.File, 0, len(ids))

	for _, id := range ids {
		if p := pip.ConvertSIPPath(id.Path); p != "" {
			id.Path = p
			r = append(r, id)
		}
	}
	slices.SortFunc(r, identifiers.Compare)

	return r
}
