package activities_test

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func testSIP(t *testing.T, path string) sip.SIP {
	t.Helper()

	s, err := sip.New(filepath.Clean(path))
	assert.NilError(t, err)

	return s
}
