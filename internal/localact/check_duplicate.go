package localact

import (
	"context"
	"errors"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence"
)

type (
	CheckDuplicateParams struct {
		Name     string
		Checksum string
	}
	CheckDuplicateResult struct {
		IsDuplicate bool
	}
)

func CheckDuplicate(
	ctx context.Context,
	psvc persistence.Service,
	params *CheckDuplicateParams,
) (*CheckDuplicateResult, error) {
	err := psvc.CreateSIP(ctx, params.Name, params.Checksum)
	if err != nil {
		if errors.Is(err, persistence.ErrDuplicatedSIP) {
			return &CheckDuplicateResult{IsDuplicate: true}, nil
		} else {
			return nil, fmt.Errorf("CheckDuplicate: %v", err)
		}
	}
	return &CheckDuplicateResult{}, nil
}
