package apis

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) AccessToken(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func TestSecuritySourceSmart(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name      string
		source    securitySource
		wantToken string
		wantErr   string
	}

	for _, tc := range []testCase{
		{
			name: "static token overrides provider",
			source: securitySource{
				staticToken:   "mock-token",
				tokenProvider: stubTokenProvider{token: "provider-token"},
			},
			wantToken: "mock-token",
		},
		{
			name: "provider token is used when static token is absent",
			source: securitySource{
				tokenProvider: stubTokenProvider{token: "provider-token"},
			},
			wantToken: "provider-token",
		},
		{
			name:    "missing token source returns error",
			source:  securitySource{},
			wantErr: "missing APIS token provider",
		},
		{
			name: "provider error is returned",
			source: securitySource{
				tokenProvider: stubTokenProvider{err: errors.New("error")},
			},
			wantErr: "failed to get access token: error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.source.Smart(t.Context(), gen.APIHealthzGetOperation)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, got.Token, tc.wantToken)
		})
	}
}
