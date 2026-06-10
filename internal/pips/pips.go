package pips

import (
	"path/filepath"
	"strings"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

type PIP struct {
	// Type is the type of SIP (DigitizedAIP, DigitizedSIP, BornDigital).
	Type enums.SIPType

	// Path is the filepath of the PIP directory.
	Path string

	// ManifestPath is the filepath of the SIP manifest —
	// "UpdatedAreldaMetadata.xml" for digitized AIPs, "metadata.xml" for all
	// other SIP types.
	ManifestPath string
}

func New(path string, t enums.SIPType) PIP {
	p := PIP{Path: path, Type: t}
	if p.Type == enums.SIPTypeDigitizedAIP {
		p.ManifestPath = filepath.Join(path, "metadata", "UpdatedAreldaMetadata.xml")
	} else {
		p.ManifestPath = filepath.Join(path, "objects", p.Name(), "header", "metadata.xml")
	}

	return p
}

func NewFromSIP(sip sip.SIP) PIP {
	return New(sip.Path, sip.Type)
}

func (p PIP) Name() string {
	return filepath.Base(p.Path)
}

func (p PIP) ConvertSIPPath(path string) string {
	switch name := filepath.Base(path); name {
	case "Prozess_Digitalisierung_PREMIS.xml", "UpdatedAreldaMetadata.xml":
		return filepath.Join("metadata", name)
	case "metadata.xml":
		return filepath.Join("objects", p.Name(), "header", name)
	}

	if strings.HasPrefix(path, "content") {
		return filepath.Join("objects", p.Name(), path)
	}

	return ""
}
