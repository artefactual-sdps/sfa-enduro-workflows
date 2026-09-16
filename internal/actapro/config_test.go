package actapro

import (
	"testing"
	"time"

	"go.artefactual.dev/tools/clientauth"
	"gotest.tools/v3/assert"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "valid config",
			config: Config{
				URL:          "http://actapro.example.test",
				Timeout:      DefaultTimeout,
				PollInterval: DefaultPollInterval,
				Token:        "mock-token",
			},
		},
		{
			name: "valid OIDC config",
			config: Config{
				URL:          "http://actapro.example.test",
				Timeout:      DefaultTimeout,
				PollInterval: DefaultPollInterval,
				OIDC: OIDCConfig{
					Enabled: true,
					//nolint:staticcheck,gosec // ACTApro uses the legacy password grant; credentials are test fixtures.
					OIDCPasswordGrantAccessTokenProviderConfig: clientauth.OIDCPasswordGrantAccessTokenProviderConfig{
						TokenURL: "https://oidc.example.test/token",
						ClientID: "actapro",
						Username: "test-user",
						Password: "test-password",
					},
				},
			},
		},
		{
			name: "invalid OIDC config",
			config: Config{
				URL:          "http://actapro.example.test",
				Timeout:      DefaultTimeout,
				PollInterval: DefaultPollInterval,
				OIDC:         OIDCConfig{Enabled: true},
			},
			wantErr: "ACTApro.OIDC:\nmissing OIDC providerURL or tokenURL",
		},
		{
			name: "missing URL",
			config: Config{
				Timeout:      DefaultTimeout,
				PollInterval: DefaultPollInterval,
			},
			wantErr: "ACTApro.URL: missing required value",
		},
		{
			name: "negative timeout",
			config: Config{
				URL:          "http://actapro.example.test",
				Timeout:      -1 * time.Second,
				PollInterval: DefaultPollInterval,
			},
			wantErr: "ACTApro.Timeout: value -1s is less than 0",
		},
		{
			name: "zero poll interval",
			config: Config{
				URL:          "http://actapro.example.test",
				Timeout:      DefaultTimeout,
				PollInterval: 0,
			},
			wantErr: "ACTApro.PollInterval: value 0s is less than or equal to 0",
		},
		{
			name: "negative poll interval",
			config: Config{
				URL:          "http://actapro.example.test",
				Timeout:      DefaultTimeout,
				PollInterval: -1 * time.Second,
			},
			wantErr: "ACTApro.PollInterval: value -1s is less than or equal to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)
		})
	}
}
