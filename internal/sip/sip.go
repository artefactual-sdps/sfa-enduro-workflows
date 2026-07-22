package sip

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fsutil"
)

type SIP struct {
	// Type is the type of SIP (DigitizedAIP, DigitizedSIP, BornDigital).
	Type enums.SIPType

	// Path is the filepath of the SIP directory.
	Path string

	// ContentPath is the filepath of the "content" directory.
	ContentPath string

	// LogicalMDPath is the filepath of the logical metadata file (AIP only).
	LogicalMDPath string

	// ManifestPath is the filepath of the SIP manifest —
	// "UpdatedAreldaMetadata.xml" for digitized AIPs, "metadata.xml" for all
	// other SIP types.
	ManifestPath string

	// MetadataPath is the path of the "metadata.xml" file.
	MetadataPath string

	// UpdatedAreldaMDPath is the filepath of the "UpdatedAreldaMetadata.xml"
	// file for digitized AIPs — it is empty for all other SIP types.
	UpdatedAreldaMDPath string

	// XSDPath is the filepath of the "arelda.xsd" file.
	XSDPath string

	// TopLevelPaths is a list of all the top level SIP directories.
	TopLevelPaths []string
}

func New(path string) (SIP, error) {
	s := SIP{}

	if _, err := os.Stat(path); err != nil {
		return s, fmt.Errorf("SIP: New: %v", err)
	}
	s.Path = path

	f, err := fsutil.FindFilename(s.Path, "Prozess_Digitalisierung_PREMIS.xml")
	if err != nil {
		return s, fmt.Errorf("SIP: New: %v", err)
	}
	hasProzessFile := len(f) > 0
	hasAdditionalDir := fsutil.FileExists(filepath.Join(s.Path, "additional"))

	if hasProzessFile {
		if hasAdditionalDir {
			return s.digitizedAIP(), nil
		} else {
			return s.digitizedSIP(), nil
		}
	} else {
		if hasAdditionalDir {
			return s.bornDigitalAIP(), nil
		} else {
			return s.bornDigitalSIP(), nil
		}
	}
}

func (s SIP) digitizedAIP() SIP {
	s.Type = enums.SIPTypeDigitizedAIP
	s.ContentPath = filepath.Join(s.Path, "content", "content")
	s.LogicalMDPath = filepath.Join(s.Path, "additional", s.Name()+"-premis.xml")
	s.MetadataPath = filepath.Join(s.Path, "content", "header", "old", "SIP", "metadata.xml")
	s.UpdatedAreldaMDPath = filepath.Join(s.Path, "additional", "UpdatedAreldaMetadata.xml")
	s.ManifestPath = s.UpdatedAreldaMDPath
	s.XSDPath = filepath.Join(s.Path, "content", "header", "xsd", "arelda.xsd")
	s.TopLevelPaths = []string{
		filepath.Join(s.Path, "content"),
		filepath.Join(s.Path, "additional"),
	}

	return s
}

func (s SIP) digitizedSIP() SIP {
	s.Type = enums.SIPTypeDigitizedSIP
	s.ContentPath = filepath.Join(s.Path, "content")
	s.MetadataPath = filepath.Join(s.Path, "header", "metadata.xml")
	s.ManifestPath = s.MetadataPath
	s.XSDPath = filepath.Join(s.Path, "header", "xsd", "arelda.xsd")
	s.TopLevelPaths = []string{
		filepath.Join(s.Path, "content"),
		filepath.Join(s.Path, "header"),
	}

	return s
}

func (s SIP) bornDigitalAIP() SIP {
	s = s.digitizedAIP()
	s.Type = enums.SIPTypeBornDigitalAIP

	return s
}

func (s SIP) bornDigitalSIP() SIP {
	s = s.digitizedSIP()
	s.Type = enums.SIPTypeBornDigitalSIP

	return s
}

func (s SIP) Name() string {
	return filepath.Base(s.Path)
}

func (s SIP) IsAIP() bool {
	return s.Type == enums.SIPTypeBornDigitalAIP || s.Type == enums.SIPTypeDigitizedAIP
}

func (s SIP) IsSIP() bool {
	return s.Type == enums.SIPTypeBornDigitalSIP || s.Type == enums.SIPTypeDigitizedSIP
}

func (s SIP) IsDigitized() bool {
	return s.Type == enums.SIPTypeDigitizedAIP || s.Type == enums.SIPTypeDigitizedSIP
}

func (s SIP) HasValidName() bool {
	yyyymmdd := "(\\d{4})(\\d{2})(\\d{2})"
	alphaNum := "[a-zA-Z0-9]"
	alphaNumAndUnderscore := "[a-zA-Z0-9_]"

	if s.Type == enums.SIPTypeBornDigitalSIP {
		match, _ := regexp.MatchString(
			fmt.Sprintf("^SIP_%s_%s+(_%s+)?$", yyyymmdd, alphaNum, alphaNumAndUnderscore),
			s.Name(),
		)
		return match
	}

	if s.Type == enums.SIPTypeDigitizedSIP {
		match, _ := regexp.MatchString(
			fmt.Sprintf("^SIP_%s_[V|v]ecteur_%s+$", yyyymmdd, alphaNumAndUnderscore),
			s.Name(),
		)
		return match
	}

	return true
}
