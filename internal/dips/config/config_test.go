package config_test

import (
	"log/slog"
	"os"
	"testing"

	"go.artefactual.dev/tools/log"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/config"
)

const validPersistenceConfig = `
[persistence]
driver = "mysql"
dsn = "root:root123@tcp(localhost:3306)/sfa_dips"
migrate = true
`

const validTemporalConfig = `
[temporal]
address = "localhost:7233"
namespace = "default"
taskQueue = "sfa-dips"
maxConcurrentSessions = 1
`

func TestLogFormatLoggerFormat(t *testing.T) {
	assert.Equal(t, config.LogFormatJSON.LoggerFormat(), log.FormatJSON)
	assert.Equal(t, config.LogFormatText.LoggerFormat(), log.FormatText)
}

func TestReadLoadsConfiguration(t *testing.T) {
	t.Setenv("SFA_DIPS_API_CORSORIGIN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
logFormat = "text"
verbosity = 2

[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://example.test"

[api.log]
path = "stdout"
level = "WARN"
format = "text"
`+validPersistenceConfig+validTemporalConfig))

	var cfg config.Config
	found, used, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, found, true)
	assert.Equal(t, used, tmpDir.Join("sfa-dips.toml"))
	assert.Equal(t, cfg.LogFormat, config.LogFormatText)
	assert.Equal(t, cfg.Verbosity, 2)
	assert.Equal(t, cfg.API.Listen, "127.0.0.1:8080")
	assert.Equal(t, cfg.API.CORSOrigin, "https://example.test")
	assert.Equal(t, cfg.API.Log.Path, "stdout")
	assert.Equal(t, cfg.API.Log.Level, slog.LevelWarn)
	assert.Equal(t, cfg.API.Log.Format, api.LogFormatText)
	assert.Equal(t, cfg.Persistence.Driver, "mysql")
	assert.Equal(t, cfg.Persistence.DSN, "root:root123@tcp(localhost:3306)/sfa_dips")
	assert.Equal(t, cfg.Persistence.Migrate, true)
	assert.Equal(t, cfg.Temporal.Address, "localhost:7233")
	assert.Equal(t, cfg.Temporal.Namespace, "default")
	assert.Equal(t, cfg.Temporal.TaskQueue, "sfa-dips")
	assert.Equal(t, cfg.Temporal.MaxConcurrentSessions, 1)
}

func TestReadRejectsInvalidConfiguration(t *testing.T) {
	const invalidConfig = `
logFormat = "invalid"

[api.auth]
enabled=true

[api.log]
format = "invalid"
`
	tmpDir := fs.NewDir(t, "",
		fs.WithFile("invalid-log-level.toml", invalidConfig+`level = "panic"`),
		fs.WithFile("invalid-config.toml", invalidConfig),
	)

	// An invalid log level stops decoding before configuration validation runs.
	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("invalid-log-level.toml"))
	assert.ErrorContains(t, err, `invalid log level 'panic', valid values are: debug, info, warn, error`)

	cfg = config.Config{}
	_, _, err = config.Read(&cfg, tmpDir.Join("invalid-config.toml"))
	assert.Error(
		t,
		err,
		`failed to validate the provided config: LogFormat: unsupported value "invalid" (use "json" or "text")
unsupported log format: "invalid", supported formats are "json", "text"
OIDC configuration required when API auth is enabled
Persistence.Driver: missing required value
Persistence.DSN: missing required value
Temporal.Address: missing required value
Temporal.Namespace: missing required value
Temporal.TaskQueue: missing required value
Temporal.MaxConcurrentSessions: must be greater than 0`,
	)
}

func TestReadLoadsConfigurationFromEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("SFA_DIPS_LOGFORMAT", "text")
	t.Setenv("SFA_DIPS_VERBOSITY", "2")
	t.Setenv("SFA_DIPS_API_LISTEN", "127.0.0.1:8090")
	t.Setenv("SFA_DIPS_API_CORSORIGIN", "https://env.example.test")
	t.Setenv("SFA_DIPS_API_LOG_PATH", "stderr")
	t.Setenv("SFA_DIPS_API_LOG_LEVEL", "WARN")
	t.Setenv("SFA_DIPS_API_LOG_FORMAT", "text")
	t.Setenv("SFA_DIPS_API_AUTH_ENABLED", "false")
	t.Setenv("SFA_DIPS_PERSISTENCE_DRIVER", "mysql")
	t.Setenv("SFA_DIPS_PERSISTENCE_DSN", "env:env@tcp(env-mysql:3306)/env-dips")
	t.Setenv("SFA_DIPS_PERSISTENCE_MIGRATE", "true")
	t.Setenv("SFA_DIPS_TEMPORAL_ADDRESS", "temporal:7233")
	t.Setenv("SFA_DIPS_TEMPORAL_NAMESPACE", "env-namespace")
	t.Setenv("SFA_DIPS_TEMPORAL_TASKQUEUE", "env-dips")
	t.Setenv("SFA_DIPS_TEMPORAL_MAXCONCURRENTSESSIONS", "3")

	var cfg config.Config
	found, used, err := config.Read(&cfg, "")

	assert.NilError(t, err)
	assert.Equal(t, found, false)
	assert.Equal(t, used, "")
	assert.Equal(t, cfg.LogFormat, config.LogFormatText)
	assert.Equal(t, cfg.Verbosity, 2)
	assert.Equal(t, cfg.API.Listen, "127.0.0.1:8090")
	assert.Equal(t, cfg.API.CORSOrigin, "https://env.example.test")
	assert.Equal(t, cfg.API.Log.Path, "stderr")
	assert.Equal(t, cfg.API.Log.Level, slog.LevelWarn)
	assert.Equal(t, cfg.API.Log.Format, api.LogFormatText)
	assert.Equal(t, cfg.API.Auth.Enabled, false)
	assert.Equal(t, cfg.Persistence.Driver, "mysql")
	assert.Equal(t, cfg.Persistence.DSN, "env:env@tcp(env-mysql:3306)/env-dips")
	assert.Equal(t, cfg.Persistence.Migrate, true)
	assert.Equal(t, cfg.Temporal.Address, "temporal:7233")
	assert.Equal(t, cfg.Temporal.Namespace, "env-namespace")
	assert.Equal(t, cfg.Temporal.TaskQueue, "env-dips")
	assert.Equal(t, cfg.Temporal.MaxConcurrentSessions, 3)
}

func TestReadSetsDefaults(t *testing.T) {
	t.Setenv("SFA_DIPS_API_CORSORIGIN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", validPersistenceConfig+validTemporalConfig))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, cfg.LogFormat, config.LogFormatJSON)
	assert.Equal(t, cfg.API.Listen, "127.0.0.1:8080")
	assert.Equal(t, cfg.API.CORSOrigin, "127.0.0.1:8080")
	assert.Equal(t, cfg.API.Log.Level, slog.LevelInfo)
	assert.Equal(t, cfg.API.Log.Format, api.LogFormatJSON)
}

func TestReadSetsCORSOriginEnvironment(t *testing.T) {
	t.Setenv("SFA_DIPS_API_CORSORIGIN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://example.test"
`+validPersistenceConfig+validTemporalConfig))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, os.Getenv("SFA_DIPS_API_CORSORIGIN"), "https://example.test")
}
