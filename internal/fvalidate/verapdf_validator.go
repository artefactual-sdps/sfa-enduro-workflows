package fvalidate

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fsutil"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
)

// pdfaPUIDs are the https://www.nationalarchives.gov.uk/pronom/ IDs of the
// PDF/A formats.
var pdfaPUIDs = []string{
	"fmt/95",   // PDF/A 1a
	"fmt/354",  // PDF/A 1b
	"fmt/476",  // PDF/A 2a
	"fmt/477",  // PDF/A 2b
	"fmt/478",  // PDF/A 2u
	"fmt/479",  // PDF/A 3a
	"fmt/480",  // PDF/A 3b
	"fmt/481",  // PDF/A 3u
	"fmt/1910", // PDF/A 4
	"fmt/1911", // PDF/A 4e
	"fmt/1912", // PDF/A 4f
}

type veraPDFValidator struct {
	cmd   string
	scope TargetType
}

var _ Validator = (*veraPDFValidator)(nil)

func NewVeraPDFValidator(cmd string) *veraPDFValidator {
	return &veraPDFValidator{cmd: cmd, scope: TargetTypeDir}
}

func (v *veraPDFValidator) FormatIDs() []string {
	return pdfaPUIDs
}

func (v *veraPDFValidator) Name() string {
	return "veraPDF"
}

func (v *veraPDFValidator) PREMISAgent() premis.Agent {
	name, err := v.Version()
	if err != nil || name == "" {
		name = fmt.Sprintf("%s (version unknown)", v.Name())
	}

	return premis.Agent{
		Type:    "software",
		Name:    name,
		IdType:  "url",
		IdValue: "https://verapdf.org",
	}
}

func (v *veraPDFValidator) Validate(path string) (string, error) {
	// If the veraPDF cmd path is not set then skip validation.
	if v.cmd == "" {
		return "", nil
	}

	if !fsutil.FileExists(path) {
		return "", fmt.Errorf("validate: file not found: %s", path)
	}

	_, err := v.run("--recurse", path)
	if err == nil { // error IS nil.
		return "", nil
	}

	e, ok := err.(*exec.ExitError)
	if !ok {
		return "", err
	}

	switch e.ExitCode() {
	case 1:
		// Exit code 1 indicates a validation error, and there is no
		// STDERR. In this case the full validation report is written to
		// STDOUT, but we are ignoring it right now because it is very
		// long.
		return "One or more PDF/A files are invalid", nil

	default:
		// Other exit codes (e.g. file not found) should write an error
		// message to STDERR.
		return "", NewSystemError(
			v.Name(),
			e.ExitCode(),
			errors.New(string(e.Stderr)),
			"PDF/A validation failed with an application error",
		)
	}
}

func (v *veraPDFValidator) Scope() TargetType {
	return v.scope
}

func (v *veraPDFValidator) Version() (string, error) {
	// If the veraPDF cmd path is not set then skip returning the version.
	if v.cmd == "" {
		return "", nil
	}

	output, err := v.run("--version")
	if err != nil {
		return "", err
	}

	lines := strings.Split(output, "\n")

	return lines[0], nil
}

func (v *veraPDFValidator) run(args ...string) (string, error) {
	result := exec.Command(v.cmd, args...) // #nosec: G204 -- trusted path.

	output, err := result.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}
