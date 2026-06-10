package activities

import (
	"context"
	"errors"
	"fmt"
	"slices"

	temporalsdk_activity "go.temporal.io/sdk/activity"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fformat"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const ValidateFilesName = "validate-files"

type (
	ValidateFiles struct {
		identifier fformat.Identifier
		validators []fvalidate.Validator
	}
	ValidateFilesParams struct {
		SIP sip.SIP
	}
	ValidateFilesResult struct {
		Failures []string
	}
)

func NewValidateFiles(idr fformat.Identifier, vdrs ...fvalidate.Validator) *ValidateFiles {
	return &ValidateFiles{
		identifier: idr,
		validators: vdrs,
	}
}

// Execute validates SIP files against a file format specification. The
// only format validator currently implemented verapdf for PDF/A.
func (a *ValidateFiles) Execute(ctx context.Context, params *ValidateFilesParams) (*ValidateFilesResult, error) {
	logger := temporalsdk_activity.GetLogger(ctx)

	formats, err := fformat.IdentifyFormats(ctx, a.identifier, params.SIP)
	if err != nil {
		return nil, fmt.Errorf("identifyFormats: %v", err)
	}

	failures, err := a.validateFiles(params.SIP, formats)
	if err != nil {
		var se *fvalidate.SystemError
		if errors.As(err, &se) {
			// Log the underlying system error and the validator name.
			logger.Error(se.Error(), "validator", se.Validator())

			// Return a user-friendly error message.
			return nil, errors.New(se.Message())
		}

		return nil, fmt.Errorf("validateFiles: %v", err)
	}

	return &ValidateFilesResult{Failures: failures}, nil
}

func (a *ValidateFiles) validateFiles(
	sip sip.SIP,
	files fformat.FileFormats,
) ([]string, error) {
	var failures []string
	for _, v := range a.validators {
		var (
			out string
			err error
		)

		if v.Scope() == fvalidate.TargetTypeDir {
			out, err = validateDir(v, sip.ContentPath, files)
		} else {
			return nil, fmt.Errorf("unsupported validator scope")
		}

		if err != nil {
			return nil, err
		}
		if out != "" {
			failures = append(failures, out)
		}
	}

	return failures, nil
}

func validateDir(v fvalidate.Validator, path string, ff fformat.FileFormats) (string, error) {
	var canValidate bool
	allowedIds := v.FormatIDs()

	for _, f := range ff {
		if slices.Contains(allowedIds, f.ID) {
			canValidate = true
			break
		}
	}

	if !canValidate {
		return "", nil
	}

	out, err := v.Validate(path)
	if err != nil {
		return "", err
	}

	return out, nil
}
