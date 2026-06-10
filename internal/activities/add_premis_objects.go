package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const AddPREMISObjectsName = "add-premis-objects"

type AddPREMISObjectsParams struct {
	SIP            sip.SIP
	PREMISFilePath string
}

type AddPREMISObjectsResult struct{}

type AddPREMISObjectsActivity struct {
	rng io.Reader
}

func NewAddPREMISObjects(rand io.Reader) *AddPREMISObjectsActivity {
	return &AddPREMISObjectsActivity{rng: rand}
}

func (a *AddPREMISObjectsActivity) Execute(
	ctx context.Context,
	params *AddPREMISObjectsParams,
) (*AddPREMISObjectsResult, error) {
	// Get subpaths of files in transfer.
	subpaths, err := premis.FilesWithinDirectory(params.SIP.ContentPath)
	if err != nil {
		return nil, err
	}

	// Create parent directory, if necessary.
	mdPath := filepath.Dir(params.PREMISFilePath)
	if err := os.MkdirAll(mdPath, 0o700); err != nil {
		return nil, err
	}

	doc, err := premis.ParseOrInitialize(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(params.SIP.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("open manifest file: %v", err)
	}
	defer f.Close()

	for _, subpath := range subpaths {
		id, err := uuid.NewRandomFromReader(a.rng)
		if err != nil {
			return nil, fmt.Errorf("generate UUID: %v", err)
		}

		object := premis.Object{
			IdType:       "UUID",
			IdValue:      id.String(),
			OriginalName: premis.OriginalNameForSubpath(params.SIP, subpath),
		}

		err = premis.AppendObjectXML(doc, object)
		if err != nil {
			return nil, err
		}
	}

	doc.Indent(2)
	err = doc.WriteToFile(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	return &AddPREMISObjectsResult{}, nil
}
