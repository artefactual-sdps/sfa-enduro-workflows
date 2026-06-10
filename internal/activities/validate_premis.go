package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"go.artefactual.dev/tools/temporal"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fsutil"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
)

const ValidatePREMISName = "ValidatePREMIS"

type (
	ValidatePREMIS struct {
		validator xmlvalidate.XSDValidator
		xsd       string
	}

	ValidatePREMISParams struct {
		// Path of the PREMIS XML file to be validated.
		Path string
	}

	ValidatePREMISResult struct {
		Failures []string
	}
)

func NewValidatePREMIS(v xmlvalidate.XSDValidator) *ValidatePREMIS {
	return &ValidatePREMIS{validator: v}
}

// Execute validates the given PREMIS file against an XSD.
func (a *ValidatePREMIS) Execute(ctx context.Context, params *ValidatePREMISParams) (*ValidatePREMISResult, error) {
	var failures []string

	logger := temporal.GetLogger(ctx)

	if !fsutil.FileExists(params.Path) {
		failures = append(
			failures,
			fmt.Sprintf("file not found: %s", filepath.Base(params.Path)),
		)
		return &ValidatePREMISResult{Failures: failures}, nil
	}

	xsd, err := a.xsdPath()
	if err != nil {
		return nil, fmt.Errorf("get PREMIS XSD path: %v", err)
	}

	out, err := a.validator.Validate(ctx, params.Path, xsd)
	if err != nil {
		return nil, fmt.Errorf("validate PREMIS: %v", err)
	}
	if out != "" {
		logger.Info("PREMIS validation failed", "file", params.Path, "output", out)
		failures = append(
			failures,
			fmt.Sprintf("%s does not match expected metadata requirements", filepath.Base(params.Path)),
		)
	}

	return &ValidatePREMISResult{Failures: failures}, nil
}

// xsdPath returns the path to a local PREMIS v3 XSD file, creating the file if
// necessary.
func (a *ValidatePREMIS) xsdPath() (string, error) {
	if a.xsd != "" {
		return a.xsd, nil
	}

	f, err := os.CreateTemp("", "premis-v3-*.xsd")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(premis.XSDv3); err != nil {
		return "", err
	}

	a.xsd = f.Name()

	return f.Name(), nil
}
