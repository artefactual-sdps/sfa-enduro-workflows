package identifiers

import (
	"errors"
	"slices"
	"strings"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/manifest"
)

// Identifier represents a file identifier in an Archivematica
// "identifiers.json" document.
type Identifier struct {
	Value string `json:"identifier"`
	Type  string `json:"identifierType"`
}

// File represents a file and its associated identifiers in an Archivematica
// "identifiers.json" document.
type File struct {
	Path        string       `json:"file"`
	Identifiers []Identifier `json:"identifiers"`
}

// Compare returns an integer comparing two File paths lexicographically for
// sorting purposes. The returned value matches the return values of
// https://pkg.go.dev/strings#Compare.
func Compare(a, b File) int {
	return strings.Compare(a.Path, b.Path)
}

func FromManifest(m *manifest.Manifest) ([]File, error) {
	if m == nil || len(m.Files) == 0 {
		return nil, errors.New("no files in manifest")
	}

	res := make([]File, len(m.Files))
	i := 0
	for path, file := range m.Files {
		res[i] = File{
			Path: path,
			Identifiers: []Identifier{
				{
					Value: file.ID,
					Type:  "local",
				},
			},
		}
		i++
	}
	slices.SortFunc(res, Compare)

	return res, nil
}
