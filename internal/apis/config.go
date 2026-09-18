package apis

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
	// Enabled toggles APIS integration on or off.
	Enabled bool
	// URL is the APIS base URL.
	URL string
	// Timeout configures APIS HTTP client timeout. A value of 0 disables the timeout.
	Timeout time.Duration
	// PollInterval configures the interval between APIS import task status polls.
	PollInterval time.Duration
	// Token overrides the token provider and is mainly useful for local or mock
	// APIS deployments that expect a fixed bearer token.
	Token string
	// OIDC config for gotools/clientauth token provider.
	OIDC OIDCConfig
}

type OIDCConfig struct {
	Enabled                                  bool
	clientauth.OIDCAccessTokenProviderConfig `mapstructure:",squash"`
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	var err error
	if c.URL == "" {
		err = fmt.Errorf("APIS.URL: missing required value")
	}
	if c.Timeout < 0 {
		err = errors.Join(err, fmt.Errorf("APIS.Timeout: value %s is less than 0", c.Timeout))
	}
	if c.PollInterval <= 0 {
		err = errors.Join(err, fmt.Errorf("APIS.PollInterval: value %s is less than or equal to 0", c.PollInterval))
	}
	if c.OIDC.Enabled {
		if oidcErr := c.OIDC.Validate(); oidcErr != nil {
			err = errors.Join(err, fmt.Errorf("APIS.OIDC:\n%v", oidcErr))
		}
	}

	return err
}
