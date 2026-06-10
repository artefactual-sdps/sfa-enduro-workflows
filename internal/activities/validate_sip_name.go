package activities

import (
	"context"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const ValidateSIPNameName = "validate-sip-name"

type ValidateSIPNameParams struct {
	SIP sip.SIP
}

type ValidateSIPNameResult struct {
	Failures []string
}

type ValidateSIPName struct{}

func NewValidateSIPName() *ValidateSIPName {
	return &ValidateSIPName{}
}

func (a *ValidateSIPName) Execute(
	ctx context.Context,
	params *ValidateSIPNameParams,
) (*ValidateSIPNameResult, error) {
	var failures []string

	// Check SIP name for naming standards.
	if !params.SIP.HasValidName() {
		failures = append(failures, fmt.Sprintf("SIP name %q violates naming standard", params.SIP.Name()))
	}

	return &ValidateSIPNameResult{Failures: failures}, nil
}
