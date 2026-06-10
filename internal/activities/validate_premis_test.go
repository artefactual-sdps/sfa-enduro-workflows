package activities_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"github.com/tonglil/buflogr"
	"go.artefactual.dev/tools/temporal"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_interceptor "go.temporal.io/sdk/interceptor"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
)

var premisXML = `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>uuid</premis:objectIdentifierType>
      <premis:objectIdentifierValue>c74a85b7-919b-409e-8209-9c7ebe0e7945</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/test_transfer/content/cat.jpg</premis:originalName>
  </premis:object>
</premis:premis>
`

type fakeValidator struct {
	Msg string
	Err error
}

func (v *fakeValidator) Validate(ctx context.Context, xmlPath, xsdPath string) (string, error) {
	return v.Msg, v.Err
}

func (v *fakeValidator) WithMsg(msg string) *fakeValidator {
	v.Msg = msg
	return v
}

func (v *fakeValidator) WithErr(err error) *fakeValidator {
	v.Err = err
	return v
}

func newFakeValidator() *fakeValidator {
	return &fakeValidator{}
}

func TestValidatePREMIS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validator xmlvalidate.XSDValidator
		params    activities.ValidatePREMISParams
		want      activities.ValidatePREMISResult
		wantErr   string
	}{
		{
			name:      "Validates a PREMIS file",
			validator: newFakeValidator(),
			params: activities.ValidatePREMISParams{
				Path: fs.NewDir(t, "enduro-test",
					fs.WithFile("premis.xml", premisXML),
				).Join("premis.xml"),
			},
		},
		{
			name:      "Returns a validation failure",
			validator: newFakeValidator().WithMsg("premis.xml:12: parser error"),
			params: activities.ValidatePREMISParams{
				Path: fs.NewDir(t, "enduro-test",
					fs.WithFile("premis.xml", premisXML),
				).Join("premis.xml"),
			},
			want: activities.ValidatePREMISResult{
				Failures: []string{"premis.xml does not match expected metadata requirements"},
			},
		},
		{
			name:      "Returns a file not found failure",
			validator: newFakeValidator().WithErr(errors.New("file not found")),
			params: activities.ValidatePREMISParams{
				Path: fs.NewDir(t, "enduro-test").Join("premis.xml"),
			},
			want: activities.ValidatePREMISResult{
				Failures: []string{"file not found: premis.xml"},
			},
		},
		{
			name:      "Returns a system error",
			validator: newFakeValidator().WithErr(errors.New("permission denied: premis.xml")),
			params: activities.ValidatePREMISParams{
				Path: fs.NewDir(t, "enduro-test",
					fs.WithFile("premis.xml", premisXML),
				).Join("premis.xml"),
			},
			wantErr: "permission denied: premis.xml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logbuf bytes.Buffer
			logger := buflogr.NewWithBuffer(&logbuf)

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.SetWorkerOptions(temporalsdk_worker.Options{
				Interceptors: []temporalsdk_interceptor.WorkerInterceptor{
					temporal.NewLoggerInterceptor(logger),
				},
			})
			env.RegisterActivityWithOptions(
				activities.NewValidatePREMIS(tt.validator).Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidatePREMISName},
			)

			enc, err := env.ExecuteActivity(activities.ValidatePREMISName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}

			t.Log(logbuf.String()) // Echo log for debugging.
			assert.NilError(t, err)

			var result activities.ValidatePREMISResult
			_ = enc.Get(&result)
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
