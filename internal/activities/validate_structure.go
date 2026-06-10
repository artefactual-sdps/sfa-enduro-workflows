package activities

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const ValidateStructureName = "validate-structure"

type (
	ValidateStructure       struct{}
	ValidateStructureParams struct {
		SIP sip.SIP
	}

	ValidateStructureResult struct {
		Failures []string
	}
)

// dir represents a file or directory in the SIP, with its path,
// parent path, and whether it is a directory or not. Both path and parent paths
// are relative to the SIP base path.
type dir struct {
	path     string
	children int
}

type validationResult struct {
	dirs                   []dir
	fileCount              int
	invalidNames           []string
	hasContentDir          bool
	hasXSDDir              bool
	hasMetadataFile        bool
	hasUpdatedAreldaMDFile bool
	hasLogicalMDFile       bool
	extraDirs              []string
	extraFiles             []string
}

func NewValidateStructure() *ValidateStructure {
	return &ValidateStructure{}
}

func (a *ValidateStructure) Execute(
	ctx context.Context,
	params *ValidateStructureParams,
) (*ValidateStructureResult, error) {
	var failures []string

	res, err := validateStructure(params.SIP)
	failures = reportFailures(res, params.SIP)

	return &ValidateStructureResult{Failures: failures}, err
}

// validateStructure walks the SIP directory tree, counts directory children and
// checks for structural issues like invalid names or missing directories and
// files.
func validateStructure(sip sip.SIP) (*validationResult, error) {
	res := &validationResult{}

	// Walk the SIP directory tree.
	err := filepath.WalkDir(sip.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sip.Path, path)
		if err != nil {
			return fmt.Errorf("ValidateStructure: relative path: %w", err)
		}

		// Validate name.
		if !validateName(d.Name()) {
			res.invalidNames = append(res.invalidNames, relativePath)
		}

		// Add directories to the list of dirs to check for emptiness later.
		if d.IsDir() {
			res.dirs = append(res.dirs, dir{path: relativePath})
		} else {
			res.fileCount += 1
		}

		// Skip the rest of the checks for the SIP base path.
		if path == sip.Path {
			return nil
		}

		// Add this node to its parent directory's child count.
		parentPath := filepath.Dir(relativePath)
		for i := range res.dirs {
			if res.dirs[i].path == parentPath {
				res.dirs[i].children += 1
				break
			}
		}

		// Check for unexpected top level directories.
		if parentPath == "." {
			if d.IsDir() && !slices.Contains(sip.TopLevelPaths, path) {
				res.extraDirs = append(res.extraDirs, relativePath)
			}
		}

		// Check for unexpected files in the content directory.
		if filepath.Dir(path) == sip.ContentPath && !d.IsDir() {
			res.extraFiles = append(res.extraFiles, relativePath)
		}

		// Check for missing directories.
		if path == sip.ContentPath {
			res.hasContentDir = true
		}
		if path == sip.XSDPath {
			res.hasXSDDir = true
		}

		// Check for missing files.
		if path == sip.MetadataPath {
			res.hasMetadataFile = true
		}
		if path == sip.UpdatedAreldaMDPath {
			res.hasUpdatedAreldaMDFile = true
		}
		if path == sip.LogicalMDPath {
			res.hasLogicalMDFile = true
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ValidateStructure: %v", err)
	}

	return res, nil
}

// reportFailures takes the result of validateStructure and returns a list of
// human-readable failure messages.
func reportFailures(res *validationResult, sip sip.SIP) []string {
	var failures []string

	// Report an empty SIP and stop further checks to avoid reporting multiple
	// failures that are a consequence of the SIP being empty.
	if len(res.dirs) == 1 && res.fileCount == 0 {
		failures = append(failures, "The SIP is empty")
		return failures
	}

	// Report empty directories.
	hasEmptyDir := false
	for _, node := range res.dirs {
		if node.children == 0 {
			failures = append(failures, fmt.Sprintf("An empty directory has been found - %s", node.path))
			hasEmptyDir = true
		}
	}
	if hasEmptyDir {
		failures = append(failures, "Please remove the empty directories and update the metadata manifest accordingly")
	}

	// Report invalid file/directory names.
	for _, path := range res.invalidNames {
		failures = append(failures, fmt.Sprintf("Name %q contains invalid character(s)", path))
	}

	// Report missing content directory.
	if !res.hasContentDir {
		failures = append(failures, "Content folder is missing")
	}

	// Report missing XSD directory.
	if !res.hasXSDDir {
		failures = append(failures, "XSD folder is missing")
	}

	// Report missing metadata file.
	if !res.hasMetadataFile {
		failures = append(failures, fmt.Sprintf(
			"%s is missing", filepath.Base(sip.MetadataPath),
		))
	}

	// Report missing UpdatedAreldaMetadata file (AIPs only).
	if sip.IsAIP() && !res.hasUpdatedAreldaMDFile {
		failures = append(failures, fmt.Sprintf(
			"%s is missing", filepath.Base(sip.UpdatedAreldaMDPath),
		))
	}

	// Report missing logical metadata file (AIPs only).
	if sip.IsAIP() && !res.hasLogicalMDFile {
		failures = append(failures, fmt.Sprintf("%s is missing", filepath.Base(sip.LogicalMDPath)))
	}

	// Report unexpected directories.
	for _, path := range res.extraDirs {
		failures = append(failures, fmt.Sprintf("Unexpected directory: %q", path))
	}

	// Report unexpected files.
	for _, path := range res.extraFiles {
		failures = append(failures, fmt.Sprintf("Unexpected file: %q", path))
	}

	// Report more than one dossier in the content dir for digitized SIPs and
	// AIPs.
	if sip.Type == enums.SIPTypeDigitizedSIP || sip.Type == enums.SIPTypeDigitizedAIP && res.hasContentDir {
		for _, d := range res.dirs {
			if filepath.Join(sip.Path, d.path) == sip.ContentPath {
				if d.children > 1 {
					failures = append(failures, "More than one dossier in the content directory")
				}
				break
			}
		}
	}

	return failures
}

// validateName checks that all characters in the name are valid. Valid
// characters are letters, numbers, "-", "_", ".", "(", and ")".
func validateName(name string) bool {
	const validChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.()"

	for i := range len(name) {
		if !strings.Contains(validChars, string(name[i])) {
			return false
		}
	}

	return true
}
