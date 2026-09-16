package actapro

import (
	"errors"
	"fmt"
	"time"

	"go.artefactual.dev/tools/clientauth"
)

const (
	DefaultTimeout      = 10 * time.Second
	DefaultPollInterval = 30 * time.Second
)

type Config struct {
	// URL is the ACTApro base URL.
	URL string
	// Timeout configures ACTApro HTTP client timeout.
	Timeout time.Duration
	// PollInterval configures the interval between ACTApro mass operation status polls.
	PollInterval time.Duration
	// Token overrides the token provider and is mainly useful for local or mock
	// ACTApro deployments that expect a fixed bearer token.
	Token string
	// OIDC config for gotools/clientauth token provider.
	OIDC OIDCConfig
}

type OIDCConfig struct {
	Enabled bool
	//nolint:staticcheck // ACTApro requires the legacy password grant.
	clientauth.OIDCPasswordGrantAccessTokenProviderConfig `mapstructure:",squash"`
}

func (c Config) Validate() error {
	var err error
	if c.URL == "" {
		err = fmt.Errorf("ACTApro.URL: missing required value")
	}
	if c.Timeout < 0 {
		err = errors.Join(err, fmt.Errorf("ACTApro.Timeout: value %s is less than 0", c.Timeout))
	}
	if c.PollInterval <= 0 {
		err = errors.Join(err, fmt.Errorf("ACTApro.PollInterval: value %s is less than or equal to 0", c.PollInterval))
	}
	if c.OIDC.Enabled {
		if oidcErr := c.OIDC.Validate(); oidcErr != nil {
			err = errors.Join(err, fmt.Errorf("ACTApro.OIDC:\n%v", oidcErr))
		}
	}

	return err
}
