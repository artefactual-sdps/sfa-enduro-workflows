package fvalidate_test

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
)

func TestFormatIDs(t *testing.T) {
	t.Parallel()

	v := fvalidate.NewVeraPDFValidator("")
	got := v.FormatIDs()

	assert.DeepEqual(t, got, []string{
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
	})
}

func TestName(t *testing.T) {
	t.Parallel()

	v := fvalidate.NewVeraPDFValidator("")
	got := v.Name()

	assert.Equal(t, got, "veraPDF")
}

func TestValidate(t *testing.T) {
	t.Parallel()

	type test struct {
		name    string
		cmd     string
		path    func(td string) string
		want    func(td string) string
		wantErr func(td string) string
	}
	for _, tt := range []test{
		{
			name: "Does nothing when cmd is not set",
		},
		{
			name: "Errors when path doesn't exist",
			cmd:  "echo",
			path: func(td string) string { return td + "/foo" },
			wantErr: func(td string) string {
				return fmt.Sprintf("validate: file not found: %s/foo", td)
			},
		},
		{
			name: "Returns nothing when no error",
			cmd:  "echo",
			path: func(td string) string { return td },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := fvalidate.NewVeraPDFValidator(tt.cmd)
			td := t.TempDir()

			path := ""
			if tt.path != nil {
				path = tt.path(td)
			}

			got, err := v.Validate(path)
			if tt.wantErr != nil {
				assert.Error(t, err, tt.wantErr(td))
				return
			}

			assert.NilError(t, err)

			if tt.want != nil {
				assert.Equal(t, got, tt.want(td))
			} else {
				assert.Equal(t, got, "")
			}
		})
	}
}

func TestScope(t *testing.T) {
	t.Parallel()

	v := fvalidate.NewVeraPDFValidator("")
	assert.Equal(t, v.Scope(), fvalidate.TargetTypeDir)
}

func TestVeraPDFPREMISAgent(t *testing.T) {
	t.Parallel()

	v := fvalidate.NewVeraPDFValidator("")

	got := v.PREMISAgent()
	assert.DeepEqual(t, got, premis.Agent{
		Type:    "software",
		Name:    "veraPDF (version unknown)",
		IdType:  "url",
		IdValue: "https://verapdf.org",
	})
}

func TestVersion(t *testing.T) {
	t.Parallel()

	v := fvalidate.NewVeraPDFValidator("")
	got, err := v.Version()

	assert.NilError(t, err)
	assert.Equal(t, got, "")
}
