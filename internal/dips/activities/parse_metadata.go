package activities

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
)

const ParseMetadataName = "parse-dip-metadata"

const premisNamespace = "http://www.loc.gov/premis/v3"

type AIP struct {
	UUID     uuid.UUID
	DirName  string
	METSPath string
}

type ParseMetadataParams struct {
	MetadataPath string
	AIPs         []AIP
}

type ParseMetadataResult struct {
	Files []*datatypes.File
}

type ParseMetadata struct{}

func NewParseMetadata() *ParseMetadata {
	return &ParseMetadata{}
}

// Execute collects content files from the ACTApro export and locates each file
// in the supplied AIPs using its PREMIS local object identifier.
func (a *ParseMetadata) Execute(ctx context.Context, params *ParseMetadataParams) (*ParseMetadataResult, error) {
	files, err := parseExport(params.MetadataPath)
	if err != nil {
		return nil, fmt.Errorf("parse DIP metadata: %w", err)
	}

	// Track files still needing an AIP match by datei ID. The pointers share
	// updates with files, which retains every file for the result.
	unmatched := make(map[string]*datatypes.File, len(files))
	for _, file := range files {
		unmatched[file.DateiID] = file
	}

	for _, aip := range params.AIPs {
		if len(unmatched) == 0 {
			break
		}
		if err := matchMETS(ctx, aip, unmatched); err != nil {
			return nil, fmt.Errorf("parse DIP metadata: AIP %s: %w", aip.UUID, err)
		}
	}

	// Any IDs left after searching the AIPs are missing from their METS files.
	if len(unmatched) > 0 {
		var missing []string
		for _, file := range files {
			if _, ok := unmatched[file.DateiID]; ok {
				missing = append(missing, fmt.Sprintf("%s (%s)", file.DateiID, file.DIPPath))
			}
		}
		return nil, fmt.Errorf("files not found in AIP METS:\n%s", strings.Join(missing, "\n"))
	}

	// TODO: Large file lists can exceed Temporal's default 2 MiB payload limit
	// (roughly 7,000 files with metadata like the examples). Consider returning
	// a manifest reference for larger DIPs.
	return &ParseMetadataResult{Files: files}, nil
}

type exportFolder struct {
	Name    string         `xml:"name"`
	Folders []exportFolder `xml:"ordner"`
	Files   []struct {
		ID                string `xml:"id,attr"`
		Name              string `xml:"name"`
		Checksum          string `xml:"pruefsumme"`
		ChecksumAlgorithm string `xml:"pruefalgorithmus"`
	} `xml:"datei"`
}

func parseExport(metadataPath string) ([]*datatypes.File, error) {
	metadata, err := os.Open(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("open ACTApro export: %w", err)
	}
	defer metadata.Close()

	var export struct {
		Folders []exportFolder `xml:"inhaltsverzeichnis>ordner"`
	}
	if err := xml.NewDecoder(metadata).Decode(&export); err != nil {
		return nil, fmt.Errorf("parse ACTApro export: %w", err)
	}

	var files []*datatypes.File
	for _, folder := range export.Folders {
		if folder.Name == "content" {
			files = appendExportFiles(files, folder, "")
		}
	}
	return files, nil
}

// appendExportFiles recursively collects files, building DIP paths from folder names.
func appendExportFiles(files []*datatypes.File, folder exportFolder, parent string) []*datatypes.File {
	folderPath := path.Join(parent, folder.Name)
	for _, file := range folder.Files {
		files = append(files, &datatypes.File{
			DateiID:           file.ID,
			DIPPath:           path.Join(folderPath, file.Name),
			Checksum:          file.Checksum,
			ChecksumAlgorithm: file.ChecksumAlgorithm,
		})
	}
	for _, child := range folder.Folders {
		files = appendExportFiles(files, child, folderPath)
	}
	return files
}

func matchMETS(ctx context.Context, aip AIP, unmatched map[string]*datatypes.File) error {
	mets, err := os.Open(aip.METSPath)
	if err != nil {
		return fmt.Errorf("open METS: %w", err)
	}
	defer mets.Close()

	decoder := xml.NewDecoder(mets)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("parse METS: %w", err)
		}
		elem, ok := tok.(xml.StartElement)
		if !ok || elem.Name.Space != premisNamespace || elem.Name.Local != "object" {
			continue
		}
		var object struct {
			Identifiers []struct {
				Type  string `xml:"objectIdentifierType"`
				Value string `xml:"objectIdentifierValue"`
			} `xml:"objectIdentifier"`
			OriginalName string `xml:"originalName"`
		}
		if err := decoder.DecodeElement(&object, &elem); err != nil {
			return fmt.Errorf("parse METS object: %w", err)
		}
		aipPath := strings.TrimPrefix(object.OriginalName, "%transferDirectory%")
		if aipPath == "" {
			continue
		}
		for _, id := range object.Identifiers {
			if id.Type != "local" {
				continue
			}
			if file, ok := unmatched[id.Value]; ok {
				file.AIPUUID = aip.UUID
				file.AIPPath = path.Join(aip.DirName, aipPath)
				// Remove the ID so later objects or AIPs cannot overwrite this match.
				delete(unmatched, id.Value)
				if len(unmatched) == 0 {
					return nil
				}
			}
		}
	}
}
