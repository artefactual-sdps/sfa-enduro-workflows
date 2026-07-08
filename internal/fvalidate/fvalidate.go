package fvalidate

import "github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"

type TargetType int

const (
	TargetTypeDir TargetType = iota
	TargetTypeFile
)

// Validator provides an interface for validating a file's format.
type Validator interface {
	// FormatIDs lists the format IDs that the validator can validate.
	FormatIDs() []string

	// Name of the validator.
	Name() string

	// PREMISAgent returns a PREMIS agent representing the validator.
	PREMISAgent() premis.Agent

	// Scope of the validator, whether it targets an individual file or all the
	// files in a directory.
	Scope() TargetType

	// Validate validates the file or directory at path.
	Validate(path string) (string, error)

	// Returns the version of a validator.
	Version() (string, error)
}
