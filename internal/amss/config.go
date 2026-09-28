package amss

import (
	"errors"
	"fmt"

	"go.artefactual.dev/ssclient"
)

// Config configures the Archivematica Storage Service client.
type Config ssclient.Config

func (c Config) Validate() error {
	var errs error
	if c.BaseURL == "" {
		errs = errors.Join(errs, fmt.Errorf("AMSS.BaseURL: missing required value"))
	}
	if c.Username == "" {
		errs = errors.Join(errs, fmt.Errorf("AMSS.Username: missing required value"))
	}
	if c.Key == "" {
		errs = errors.Join(errs, fmt.Errorf("AMSS.Key: missing required value"))
	}
	return errs
}
