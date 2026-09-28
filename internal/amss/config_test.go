package amss_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		config  amss.Config
		wantErr string
	}{
		{
			name: "valid config",
			config: amss.Config{
				BaseURL:  "http://amss.example.test",
				Username: "test-user",
				Key:      "test-key",
			},
		},
		{
			name:    "missing base URL",
			config:  amss.Config{Username: "test-user", Key: "test-key"},
			wantErr: "AMSS.BaseURL: missing required value",
		},
		{
			name:    "missing username",
			config:  amss.Config{BaseURL: "http://amss.example.test", Key: "test-key"},
			wantErr: "AMSS.Username: missing required value",
		},
		{
			name:    "missing key",
			config:  amss.Config{BaseURL: "http://amss.example.test", Username: "test-user"},
			wantErr: "AMSS.Key: missing required value",
		},
		{
			name: "collects all missing values",
			wantErr: "AMSS.BaseURL: missing required value\n" +
				"AMSS.Username: missing required value\n" +
				"AMSS.Key: missing required value",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
		})
	}
}
