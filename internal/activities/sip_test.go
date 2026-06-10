package activities_test

import (
	"testing"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func testSIP(t *testing.T, path string) sip.SIP {
	t.Helper()

	s, err := sip.New(path)
	if err != nil {
		t.Fatalf("sip: New(): %v", err)
	}

	return s
}
