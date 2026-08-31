package auth

import (
	"context"
)

type Claims struct {
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Name              string `json:"name,omitempty"`
	Iss               string `json:"iss,omitempty"`
	Sub               string `json:"sub,omitempty"`
}

type contextUserClaimsType struct{}

var contextUserClaimsKey = &contextUserClaimsType{}

// WithUserClaims puts the user claims into the current context.
func WithUserClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, contextUserClaimsKey, claims)
}

// UserClaimsFromContext returns the user claims from the context.
// A nil value is returned if they are not found.
func UserClaimsFromContext(ctx context.Context) *Claims {
	v := ctx.Value(contextUserClaimsKey)
	if v == nil {
		return nil
	}
	c, ok := v.(*Claims)
	if !ok {
		return nil
	}
	return c
}
