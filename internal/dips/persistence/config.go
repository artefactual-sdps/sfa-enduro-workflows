package persistence

import "errors"

type Config struct {
	Driver  string
	DSN     string
	Migrate bool
}

func (c Config) Validate() error {
	var errs error
	if c.Driver == "" {
		errs = errors.Join(errs, errors.New("Persistence.Driver: missing required value"))
	}
	if c.DSN == "" {
		errs = errors.Join(errs, errors.New("Persistence.DSN: missing required value"))
	}

	return errs
}
