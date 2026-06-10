package fvalidate_test

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
)

func TestSystemError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("some error")
	se := fvalidate.NewSystemError("veraPDF", 1, err, "PDF/A validation failed with an application error")

	assert.Equal(t, se.Validator(), "veraPDF")
	assert.Equal(t, se.Error(), "system error: exit code 1: some error")
	assert.Equal(t, se.Unwrap(), err)
	assert.Equal(t, se.Message(), "PDF/A validation failed with an application error")
}
