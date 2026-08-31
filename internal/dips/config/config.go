package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.artefactual.dev/tools/log"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

var logLevels = []string{
	"debug",
	"info",
	"warn",
	"error",
}

type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

func (f LogFormat) Validate() error {
	switch f {
	case LogFormatJSON, LogFormatText:
		return nil
	default:
		return fmt.Errorf("LogFormat: unsupported value %q (use %q or %q)", f, LogFormatJSON, LogFormatText)
	}
}

// LoggerFormat returns the corresponding application logger format.
func (f LogFormat) LoggerFormat() log.Format {
	switch f {
	case LogFormatJSON:
		return log.FormatJSON
	case LogFormatText:
		return log.FormatText
	default:
		panic(fmt.Sprintf("config: invalid log format %q", f))
	}
}

type Config struct {
	// LogFormat controls the encoding of application log messages. Supported
	// values are "json" for structured output and "text" for human-readable,
	// colorized output.
	LogFormat LogFormat

	// Verbosity controls the verbosity of log messages. The default is 0 which
	// will only log the most important messages. The development environment
	// log level is 2 which will log most messages. See the developer
	// documentation for more information on logging levels.
	Verbosity int

	API api.Config

	Persistence persistence.Config
}

func (c *Config) Validate() error {
	return errors.Join(
		c.LogFormat.Validate(),
		c.API.Validate(),
		c.Persistence.Validate(),
	)
}

func Read(config *Config, configFile string) (found bool, configFileUsed string, err error) {
	v := viper.New()

	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/")
	v.AddConfigPath("/etc")
	v.SetConfigName("sfa-dips")
	v.SetDefault("api.listen", "127.0.0.1:8080")
	v.SetDefault("logFormat", LogFormatJSON)
	// Register the persistence keys so AutomaticEnv can override them during unmarshalling.
	v.SetDefault("persistence.driver", "")
	v.SetDefault("persistence.dsn", "")
	v.SetDefault("persistence.migrate", false)
	v.SetEnvPrefix("SFA_DIPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if configFile != "" {
		v.SetConfigFile(configFile)
	}

	err = v.ReadInConfig()
	_, ok := err.(viper.ConfigFileNotFoundError)
	if !ok {
		found = true
	}
	if found && err != nil {
		return found, configFileUsed, fmt.Errorf("failed to read configuration file: %w", err)
	}

	decodeHookFunc := mapstructure.ComposeDecodeHookFunc(
		// These are the viper DecodeHookFunc defaults.
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		stringToLogLevelHookFunc(),
	)

	err = v.Unmarshal(config, viper.DecodeHook(decodeHookFunc))
	if err != nil {
		return found, configFileUsed, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := config.Validate(); err != nil {
		return found, configFileUsed, fmt.Errorf("failed to validate the provided config: %w", err)
	}

	configFileUsed = v.ConfigFileUsed()

	if err := setCORSOriginEnv(config); err != nil {
		return found, configFileUsed, fmt.Errorf(
			"failed to set CORS Origin environment variable: %w", err,
		)
	}

	return found, configFileUsed, nil
}

// setCORSOriginEnv sets the CORS Origin environment variable needed by
// Goa-generated code for the API.
func setCORSOriginEnv(cfg *Config) error {
	if err := os.Setenv("SFA_DIPS_API_CORS_ORIGIN", cfg.API.CORSOrigin); err != nil {
		return err
	}

	return nil
}

func stringToLogLevelHookFunc() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || t != reflect.TypeFor[slog.Level]() {
			return data, nil
		}

		name := strings.ToLower(data.(string))
		if slices.Contains(logLevels, name) {
			var lvl slog.Level
			if err := lvl.UnmarshalText([]byte(name)); err != nil {
				return nil, fmt.Errorf("failed to unmarshal log level '%s': %w", data.(string), err)
			}
			return lvl, nil
		} else {
			return nil, fmt.Errorf(
				"invalid log level '%s', valid values are: %s",
				data.(string),
				strings.Join(logLevels, ", "),
			)
		}
	}
}
