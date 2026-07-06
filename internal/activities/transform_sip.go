package activities

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"go.artefactual.dev/tools/fsutil"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/pips"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const TransformSIPName = "transform-sip"

type TransformSIPParams struct {
	SIP sip.SIP
}

type TransformSIPResult struct {
	PIP pips.PIP
}

type TransformSIP struct{}

func NewTransformSIP() *TransformSIP {
	return &TransformSIP{}
}

func (a *TransformSIP) Execute(ctx context.Context, params *TransformSIPParams) (*TransformSIPResult, error) {
	// Create a metadata directory.
	mdPath := filepath.Join(params.SIP.Path, "metadata")
	if err := os.MkdirAll(mdPath, 0o700); err != nil {
		return nil, err
	}

	// Move the Prozess_Digitalisierung_PREMIS.xml file to the metadata
	// directory. Prozess_Digitalisierung_PREMIS.xml is only present in
	// digitized SIPs/AIPs, and there can only be one dossier in a digitized
	// SIP/AIP.
	if params.SIP.Type == enums.SIPTypeDigitizedSIP || params.SIP.Type == enums.SIPTypeDigitizedAIP {
		entries, err := os.ReadDir(params.SIP.ContentPath)
		if err != nil {
			return nil, err
		}

		p := filepath.Join(
			params.SIP.ContentPath,
			entries[0].Name(), // dossier name.
			"Prozess_Digitalisierung_PREMIS.xml",
		)

		err = fsutil.Move(p, filepath.Join(mdPath, "Prozess_Digitalisierung_PREMIS.xml"))
		if err != nil {
			return nil, err
		}
	}

	// Move the UpdatedAreldaMetatdata.xml and logical metadata files to the
	// metadata directory (AIP only).
	if params.SIP.IsAIP() {
		err := fsutil.Move(
			params.SIP.UpdatedAreldaMDPath,
			filepath.Join(mdPath, filepath.Base(params.SIP.UpdatedAreldaMDPath)),
		)
		if err != nil {
			return nil, err
		}

		err = fsutil.Move(
			params.SIP.LogicalMDPath,
			filepath.Join(mdPath, filepath.Base(params.SIP.LogicalMDPath)),
		)
		if err != nil {
			return nil, err
		}
	}

	// Create objects and [sip-name] sub-directories.
	objectsPath := filepath.Join(params.SIP.Path, "objects", params.SIP.Name())
	if err := os.MkdirAll(objectsPath, 0o700); err != nil {
		return nil, err
	}

	// Move the content directory into the objects directory.
	err := fsutil.Move(params.SIP.ContentPath, filepath.Join(objectsPath, "content"))
	if err != nil {
		return nil, err
	}

	// Create a header directory in the objects folder.
	headerPath := filepath.Join(objectsPath, "header")
	if err = os.MkdirAll(headerPath, 0o700); err != nil {
		return nil, err
	}

	// Move the metadata.xml file into the header directory.
	err = fsutil.Move(params.SIP.MetadataPath, filepath.Join(headerPath, filepath.Base(params.SIP.MetadataPath)))
	if err != nil {
		return nil, err
	}

	// Remove the old top-level directories.
	for _, path := range params.SIP.TopLevelPaths {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
	}
	if err != nil {
		return nil, err
	}

	// Set all the file modes.
	if err = fsutil.SetFileModes(params.SIP.Path, 0o700, 0o600); err != nil {
		return nil, err
	}

	return &TransformSIPResult{PIP: pips.NewFromSIP(params.SIP)}, nil
}
