package auth

import (
	"context"
	"errors"

	"github.com/coreos/go-oidc/v3/oidc"
)

var ErrUnauthorized error = errors.New("unauthorized")

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*Claims, error)
}

type NoopTokenVerifier struct{}

var _ TokenVerifier = (*NoopTokenVerifier)(nil)

func (t *NoopTokenVerifier) Verify(ctx context.Context, token string) (*Claims, error) {
	return nil, nil
}

type OIDCTokenVerifiers []oidcVerifier

type oidcVerifier struct {
	verifier *oidc.IDTokenVerifier
	cfg      OIDCConfig
}

var _ TokenVerifier = (OIDCTokenVerifiers)(nil)

func NewOIDCTokenVerifiers(ctx context.Context, cfgs OIDCConfigs) (OIDCTokenVerifiers, error) {
	if len(cfgs) == 0 {
		return nil, errors.New("missing OIDC token verifier configuration")
	}

	verifiers := make([]oidcVerifier, len(cfgs))
	for i, cfg := range cfgs {
		// Initialize an OIDC provider.
		provider, err := oidc.NewProvider(ctx, cfg.ProviderURL)
		if err != nil {
			return nil, err
		}

		// Create an ID token verifier and only trust ID tokens issued to this client ID.
		verifiers[i] = oidcVerifier{
			verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
			cfg:      cfg,
		}
	}

	return verifiers, nil
}

func (t OIDCTokenVerifiers) Verify(ctx context.Context, token string) (*Claims, error) {
	var errs []error
	for _, verifier := range t {
		claims, err := verifier.verify(ctx, token)
		if err == nil {
			return claims, nil
		}

		if errors.Is(err, ErrUnauthorized) {
			continue
		}
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return nil, ErrUnauthorized
}

func (t *oidcVerifier) verify(ctx context.Context, token string) (*Claims, error) {
	// Verify token.
	idToken, err := t.verifier.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	// Extract custom claims.
	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}

	// Check that claims are verified.
	if !t.cfg.SkipEmailVerifiedCheck && !claims.EmailVerified {
		return nil, ErrUnauthorized
	}

	return &claims, nil
}
