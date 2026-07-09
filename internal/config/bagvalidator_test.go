package config_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
)

func TestValidatorConfig(t *testing.T) {
	t.Parallel()
	type test struct {
		name    string
		config  config.BagValidator
		wantErr string
	}
	tests := []test{
		{
			name: "valid config",
			config: config.BagValidator{
				CacheDir: "/home/enduro/bagvalidator_cache",
				PoolSize: 2,
			},
		},
		{
			name: "invalid pool size",
			config: config.BagValidator{
				PoolSize: 0,
			},
			wantErr: "PoolSize: 0 is less than the minimum value (1)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.config.Validate()
			if tc.wantErr != "" {
				assert.Error(t, err, tc.wantErr)
				return
			}
			assert.NilError(t, err)
		})
	}
}
