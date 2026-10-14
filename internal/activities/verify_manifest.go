package activities

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	goset "github.com/deckarep/golang-set/v2"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/checksum"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/manifest"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const VerifyManifestName = "verify-manifest"

type (
	VerifyManifest       struct{}
	VerifyManifestParams struct {
		SIP sip.SIP
	}
	VerifyManifestResult struct {
		ChecksumFailures []string
		ManifestFailures []string
		MissingFiles     []string
		UnexpectedFiles  []string
	}
)

func NewVerifyManifest() *VerifyManifest {
	return &VerifyManifest{}
}

// Execute parses a SIP's manifest and verifies it against the actual files in
// the SIP directory. Any missing or unexpected files on disk are reported as
// failures.
func (a *VerifyManifest) Execute(ctx context.Context, params *VerifyManifestParams) (*VerifyManifestResult, error) {
	m, err := getManifest(params.SIP)
	if err != nil {
		return nil, fmt.Errorf("verify manifest: parse manifest: %v", err)
	}
	manifestSet := goset.NewSetFromMapKeys(m.Files)

	sipFiles, err := sipFiles(params.SIP)
	if err != nil {
		return nil, fmt.Errorf("verify manifest: get SIP contents: %v", err)
	}

	badChecksums, err := verifyChecksums(m.Files, sipFiles, params.SIP.Path)
	if err != nil {
		return nil, fmt.Errorf("verify checksums: %v", err)
	}

	sipBase := filepath.Base(params.SIP.Path)
	missing := missingFiles(sipBase, manifestSet, sipFiles)
	unexpected := unexpectedFiles(sipBase, manifestSet, sipFiles)

	var manifestFailures []string
	if !slices.Contains(manifest.AllowedSchemaVersions, m.SchemaVersion) {
		manifestFailures = []string{fmt.Sprintf("Unsupported schema version: %s", m.SchemaVersion)}
	}

	return &VerifyManifestResult{
		ChecksumFailures: badChecksums,
		ManifestFailures: manifestFailures,
		MissingFiles:     missing,
		UnexpectedFiles:  unexpected,
	}, nil
}

// getManifest parses the SIP manifest and returns a Manifest.
func getManifest(s sip.SIP) (*manifest.Manifest, error) {
	f, err := os.Open(s.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}

	m, err := manifest.Parse(f)
	if err != nil {
		return nil, err
	}

	// Prefix "content/" to AIP file paths.
	if s.IsAIP() {
		files := make(map[string]*manifest.File, len(m.Files))
		for k, v := range m.Files {
			files[filepath.Join("content", k)] = v
		}
		m.Files = files
	}

	return m, nil
}

// sipFiles recursively walks dir's tree and returns the set of all file
// (excluding directory) paths found.
func sipFiles(s sip.SIP) (goset.Set[string], error) {
	root := s.Path
	if s.IsAIP() {
		root = filepath.Join(s.Path, "content")
	}

	paths := goset.NewSet[string]()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		p, err := filepath.Rel(s.Path, path)
		if err != nil {
			return err
		}

		// SIPs don't include metadata.xml in the manifest, so ignore the file
		// here.
		if s.IsSIP() && p == "header/metadata.xml" {
			return nil
		}

		paths.Add(p)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

// missingFiles returns the list of all files that are in manifest but not
// filesys.
func missingFiles(base string, manifest, filesys goset.Set[string]) []string {
	var missing []string
	if s := manifest.Difference(filesys).ToSlice(); len(s) > 0 {
		slices.Sort(s)
		for _, p := range s {
			fp := filepath.Join(base, p)
			missing = append(missing, fmt.Sprintf("Missing file: %s", fp))
		}
	}
	return missing
}

// unexpectedFiles returns the list of all files that are in filesys but not
// manifest.
func unexpectedFiles(base string, manifest, filesys goset.Set[string]) []string {
	var unexpected []string
	if s := filesys.Difference(manifest).ToSlice(); len(s) > 0 {
		slices.Sort(s)
		for _, p := range s {
			fp := filepath.Join(base, p)
			unexpected = append(unexpected, fmt.Sprintf("Unexpected file: %s", fp))
		}
	}
	return unexpected
}

// verifyChecksums checks that each manifestFiles file checksum matches the
// checksum generated from the actual file contents. If a file is on the
// manifest but missing from the filesystem, or vice versa, it will be skipped
// with no validation message.  The root is the absolute path to the root
// directory of the SIP, and is prefixed to each relative file path in the
// manifest to create an absolute path the file.
func verifyChecksums(
	manifestFiles map[string]*manifest.File,
	sipFiles goset.Set[string],
	root string,
) ([]string, error) {
	var failures []string

	for path, file := range manifestFiles {
		// Check if file exists on filesystem.
		if !sipFiles.Contains(path) {
			continue
		}

		// Attempt to generate hash from filesystem file contents.
		hashResult, err := checksum.GenerateHash(filepath.Join(root, path), file.Checksum.Algorithm)
		if err != nil {
			return nil, err
		}

		// Compare hash to expected value.
		if hashResult != strings.ToLower(file.Checksum.Hash) {
			failures = append(
				failures,
				fmt.Sprintf(
					"Checksum mismatch for %q (expected: %q, got: %q)",
					path,
					file.Checksum.Hash,
					hashResult,
				),
			)
		}
	}
	slices.Sort(failures)

	return failures, nil
}
