package dips

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	goa "goa.design/goa/v3/pkg"
	"gotest.tools/v3/assert"

	di_ps "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
)

func TestShowValidatesDIPID(t *testing.T) {
	svc := NewService(logr.Discard())

	_, err := svc.Show(context.Background(), &di_ps.ShowPayload{
		ID:    "invalid-uuid",
		Token: "test",
	})

	var serviceErr *goa.ServiceError
	assert.Assert(t, errors.As(err, &serviceErr))
	assert.Equal(t, serviceErr.Name, "bad_request")
	assert.ErrorContains(t, serviceErr, `invalid DIP ID "invalid-uuid"`)
}
