package activities

import (
	"context"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const IdentifySIPName = "identify-sip"

type IdentifySIPParams struct {
	Path string
}
type IdentifySIPResult struct {
	SIP sip.SIP
}

type IdentifySIP struct{}

func NewIdentifySIP() *IdentifySIP {
	return &IdentifySIP{}
}

func (a *IdentifySIP) Execute(ctx context.Context, params *IdentifySIPParams) (*IdentifySIPResult, error) {
	s, err := sip.New(params.Path)
	if err != nil {
		return nil, fmt.Errorf("IdentifySIP: %v", err)
	}

	return &IdentifySIPResult{SIP: s}, nil
}
