package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.artefactual.dev/tools/bucket"
	"go.artefactual.dev/tools/log"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
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

type TemporalConfig struct {
	Address               string
	Namespace             string
	TaskQueue             string
	MaxConcurrentSessions int
}

func (c TemporalConfig) Validate() error {
	var errs error

	if c.Address == "" {
		errs = errors.Join(errs, fmt.Errorf("Temporal.Address: missing required value"))
	}
	if c.Namespace == "" {
		errs = errors.Join(errs, fmt.Errorf("Temporal.Namespace: missing required value"))
	}
	if c.TaskQueue == "" {
		errs = errors.Join(errs, fmt.Errorf("Temporal.TaskQueue: missing required value"))
	}
	if c.MaxConcurrentSessions <= 0 {
		errs = errors.Join(errs, fmt.Errorf("Temporal.MaxConcurrentSessions: must be greater than 0"))
	}

	return errs
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

	// WorkingDir is used to prepare DIP files and defaults to the OS temporary directory.
	WorkingDir string

	// XSDDir contains arelda.xsd and its supporting schemas. ACTApro metadata
	// exports are validated against arelda.xsd, and all .xsd files are copied
	// into each DIP's header/xsd directory.
	XSDDir string

	// RetentionPeriod is the duration to retain completed DIP archives after a
	// successful DIP creation. Must not be negative. Zero (default) retains
	// them indefinitely.
	RetentionPeriod time.Duration

	API         api.Config
	Persistence persistence.Config
	Temporal    TemporalConfig
	ACTApro     actapro.Config
	AMSS        amss.Config

	// Bucket is the destination for completed DIP ZIP archives.
	Bucket bucket.Config
}

func (c *Config) Validate() error {
	var err error
	if c.XSDDir == "" {
		err = fmt.Errorf("XSDDir: missing required value")
	}
	if c.RetentionPeriod < 0 {
		err = errors.Join(err, fmt.Errorf("RetentionPeriod: must be greater than or equal to 0"))
	}
	if c.Bucket.URL == "" && c.Bucket.Endpoint == "" {
		err = errors.Join(err, fmt.Errorf("Bucket.URL or Bucket.Endpoint: missing required value"))
	}
	return errors.Join(
		err,
		c.LogFormat.Validate(),
		c.API.Validate(),
		c.Persistence.Validate(),
		c.Temporal.Validate(),
		c.ACTApro.Validate(),
		c.AMSS.Validate(),
	)
}

func Read(config *Config, configFile string) (found bool, configFileUsed string, err error) {
	v := viper.New()

	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/")
	v.AddConfigPath("/etc")
	v.SetConfigName("sfa-dips")
	// Register keys so AutomaticEnv can override them during unmarshalling.
	v.SetDefault("logFormat", LogFormatJSON)
	v.SetDefault("verbosity", 0)
	v.SetDefault("workingDir", os.TempDir())
	v.SetDefault("xsdDir", "")
	v.SetDefault("retentionPeriod", time.Duration(0))
	v.SetDefault("api.listen", "127.0.0.1:8080")
	v.SetDefault("api.corsOrigin", "")
	v.SetDefault("api.log.path", "")
	v.SetDefault("api.log.level", slog.LevelInfo)
	v.SetDefault("api.log.format", api.LogFormatJSON)
	v.SetDefault("api.auth.enabled", false)
	v.SetDefault("persistence.driver", "")
	v.SetDefault("persistence.dsn", "")
	v.SetDefault("persistence.migrate", false)
	v.SetDefault("temporal.address", "")
	v.SetDefault("temporal.namespace", "")
	v.SetDefault("temporal.taskQueue", "")
	v.SetDefault("temporal.maxConcurrentSessions", 0)
	v.SetDefault("actapro.url", "")
	v.SetDefault("actapro.timeout", actapro.DefaultTimeout)
	v.SetDefault("actapro.pollInterval", actapro.DefaultPollInterval)
	v.SetDefault("actapro.token", "")
	v.SetDefault("actapro.oidc.enabled", false)
	v.SetDefault("actapro.oidc.providerURL", "")
	v.SetDefault("actapro.oidc.tokenURL", "")
	v.SetDefault("actapro.oidc.clientID", "")
	v.SetDefault("actapro.oidc.clientSecret", "")
	v.SetDefault("actapro.oidc.username", "")
	v.SetDefault("actapro.oidc.password", "")
	v.SetDefault("actapro.oidc.scopes", []string(nil))
	v.SetDefault("actapro.oidc.tokenExpiryLeeway", 0)
	v.SetDefault("actapro.oidc.retryMaxAttempts", 0)
	v.SetDefault("actapro.oidc.retryInitialInterval", 0)
	v.SetDefault("actapro.oidc.retryMaxInterval", 0)
	v.SetDefault("actapro.oidc.retryBackoffCoefficient", 0.0)
	v.SetDefault("amss.baseURL", "")
	v.SetDefault("amss.username", "")
	v.SetDefault("amss.key", "")
	v.SetDefault("bucket.url", "")
	v.SetDefault("bucket.endpoint", "")
	v.SetDefault("bucket.bucket", "")
	v.SetDefault("bucket.accessKey", "")
	v.SetDefault("bucket.secretKey", "")
	v.SetDefault("bucket.token", "")
	v.SetDefault("bucket.profile", "")
	v.SetDefault("bucket.region", "")
	v.SetDefault("bucket.pathStyle", false)
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
	if err := os.Setenv("SFA_DIPS_API_CORSORIGIN", cfg.API.CORSOrigin); err != nil {
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
