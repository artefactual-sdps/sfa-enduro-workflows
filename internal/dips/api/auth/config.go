package auth

import (
	"errors"
)

type Config struct {
	Enabled bool
	OIDC    OIDCConfigs
}

type OIDCConfigs []OIDCConfig

type OIDCConfig struct {
	ProviderURL            string
	ClientID               string
	SkipEmailVerifiedCheck bool
}

// Validate implements config.ConfigurationValidator.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if len(c.OIDC) == 0 {
		return errors.New("OIDC configuration required when API auth is enabled")
	}

	for i := range c.OIDC {
		if c.OIDC[i].ProviderURL == "" {
			return errors.New("OIDC provider URL required")
		}
		if c.OIDC[i].ClientID == "" {
			return errors.New("OIDC client ID required")
		}
	}

	return nil
}
