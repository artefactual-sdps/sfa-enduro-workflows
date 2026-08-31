package persistence_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  persistence.Config
		wantErr string
	}{
		{
			name:    "Missing driver",
			config:  persistence.Config{DSN: "test-dsn"},
			wantErr: "Persistence.Driver: missing required value",
		},
		{
			name:    "Missing DSN",
			config:  persistence.Config{Driver: "mysql"},
			wantErr: "Persistence.DSN: missing required value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	err := (persistence.Config{Driver: "mysql", DSN: "test-dsn"}).Validate()
	assert.NilError(t, err)
}
