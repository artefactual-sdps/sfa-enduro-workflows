package cantons

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
)

const ParseActivityName = "parse-mets-file"

type (
	ParseActivityParams struct {
		METSPath string
	}
	ParseActivityResult struct {
		// Relative path to the UpdatedAreldaMetadata.xml file from the AIP data folder.
		UpdatedAreldaMetadataRelPath string
		// Relative path to the metadata.xml file from the AIP data folder.
		MetadataRelPath string
	}
	ParseActivity struct{}
)

func NewParseActivity() *ParseActivity {
	return &ParseActivity{}
}

// Execute parses the METS file at params.METSPath and returns the relative
// paths to the AIP data folder for the UpdatedAreldaMetadata.xml and
// metadata.xml files.
func (a *ParseActivity) Execute(ctx context.Context, params *ParseActivityParams) (*ParseActivityResult, error) {
	mets, err := os.Open(params.METSPath)
	if err != nil {
		return nil, fmt.Errorf("ParseActivity: open METS: %w", err)
	}
	defer mets.Close()

	var (
		inFileGrp, inStructMap, inHeaderDiv, inMdDiv, inUAMdDiv bool
		currentFileID, mdFileID, uamdFileID                     string
	)

	// Stream the METS file creating a map of metadata and original
	// file IDs to their relative paths from the fileSec section.
	// Obtain the file IDs for the UpdatedAreldaMetadata.xml and
	// metadata.xml files from the physical structMap section.
	mdFiles := make(map[string]string)
	decoder := xml.NewDecoder(mets)
	for {
		tok, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("ParseActivity: parse METS: %w", err)
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "fileGrp":
				use := getAttr(elem, "USE")
				if use == "metadata" || use == "original" {
					inFileGrp = true
				}
			case "file":
				if inFileGrp {
					currentFileID = getAttr(elem, "ID")
				}
			case "FLocat":
				if currentFileID != "" {
					mdFiles[currentFileID] = getAttr(elem, "href")
				}
			case "structMap":
				if getAttr(elem, "TYPE") == "physical" {
					inStructMap = true
				}
			case "div":
				if inStructMap {
					label := getAttr(elem, "LABEL")
					if label == "header" {
						inHeaderDiv = true
					}
					if label == "UpdatedAreldaMetadata.xml" {
						inUAMdDiv = true
					}
					if inHeaderDiv && label == "metadata.xml" {
						inMdDiv = true
					}
				}
			case "fptr":
				if inUAMdDiv {
					uamdFileID = getAttr(elem, "FILEID")
				}
				if inMdDiv {
					mdFileID = getAttr(elem, "FILEID")
				}
			}
		case xml.EndElement:
			switch elem.Name.Local {
			case "fileGrp":
				inFileGrp = false
			case "file":
				currentFileID = ""
			case "structMap":
				inStructMap = false
			case "div":
				inHeaderDiv = false
				inUAMdDiv = false
				inMdDiv = false
			}
		}
	}

	re := &ParseActivityResult{}
	if relPath, ok := mdFiles[uamdFileID]; ok {
		re.UpdatedAreldaMetadataRelPath = relPath
	}
	if relPath, ok := mdFiles[mdFileID]; ok {
		re.MetadataRelPath = relPath
	}

	return re, nil
}

func getAttr(el xml.StartElement, attr string) string {
	for _, a := range el.Attr {
		if a.Name.Local == attr {
			return a.Value
		}
	}
	return ""
}
