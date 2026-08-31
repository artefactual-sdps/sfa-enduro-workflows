package auth_test

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/auth"
)

func TestUserClaimsFromContext(t *testing.T) {
	t.Parallel()

	t.Run("Returns claims when found", func(t *testing.T) {
		t.Parallel()

		claims := auth.Claims{
			Email:         "info@artefactual.com",
			EmailVerified: true,
			Name:          "Test User",
			Iss:           "http://keycloak:7470/realms/artefactual",
			Sub:           "61a16d59-5029-4d85-8aef-290d1951b8d3",
		}

		ctx := context.Background()
		ctx = auth.WithUserClaims(ctx, &claims)
		assert.Equal(t, auth.UserClaimsFromContext(ctx), &claims)
	})

	t.Run("Returns nil when not found", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		assert.Assert(t, cmp.Nil(auth.UserClaimsFromContext(ctx)))
	})
}
