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

func TestLogFormatLoggerFormat(t *testing.T) {
	assert.Equal(t, config.LogFormatJSON.LoggerFormat(), log.FormatJSON)
	assert.Equal(t, config.LogFormatText.LoggerFormat(), log.FormatText)
}

func TestReadLoadsLoggingConfiguration(t *testing.T) {
	t.Setenv("SFA_DIPS_PERSISTENCE_DRIVER", "")
	t.Setenv("SFA_DIPS_PERSISTENCE_DSN", "")
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

[persistence]
driver = "mysql"
dsn = "root:root123@tcp(localhost:3306)/sfa_dips"
migrate = true
`))

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
}

func TestReadAllowsMissingImplicitConfigFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SFA_DIPS_PERSISTENCE_DRIVER", "mysql")
	t.Setenv("SFA_DIPS_PERSISTENCE_DSN", "root:root123@tcp(localhost:3306)/sfa_dips")
	t.Chdir(t.TempDir())

	var cfg config.Config
	found, used, err := config.Read(&cfg, "")

	assert.NilError(t, err)
	assert.Equal(t, found, false)
	assert.Equal(t, used, "")
	assert.Equal(t, cfg.Persistence.Driver, "mysql")
	assert.Equal(t, cfg.Persistence.DSN, "root:root123@tcp(localhost:3306)/sfa_dips")
}

func TestReadSetsDefaults(t *testing.T) {
	t.Setenv("SFA_DIPS_PERSISTENCE_DRIVER", "mysql")
	t.Setenv("SFA_DIPS_PERSISTENCE_DSN", "root:root123@tcp(localhost:3306)/sfa_dips")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", "# empty config\n"))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, cfg.LogFormat, config.LogFormatJSON)
	assert.Equal(t, cfg.API.Listen, "127.0.0.1:8080")
	assert.Equal(t, cfg.API.CORSOrigin, "127.0.0.1:8080")
	assert.Equal(t, cfg.API.Log.Level, slog.LevelInfo)
	assert.Equal(t, cfg.API.Log.Format, api.LogFormatJSON)
	assert.Equal(t, cfg.Persistence.Driver, "mysql")
	assert.Equal(t, cfg.Persistence.DSN, "root:root123@tcp(localhost:3306)/sfa_dips")
}

func TestReadSetsCORSOriginEnvironment(t *testing.T) {
	t.Setenv("SFA_DIPS_API_CORS_ORIGIN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://example.test"

[persistence]
driver = "mysql"
dsn = "root:root123@tcp(localhost:3306)/sfa_dips"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, os.Getenv("SFA_DIPS_API_CORS_ORIGIN"), "https://example.test")
}

func TestReadRejectsInvalidApplicationLogFormat(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
logFormat = "invalid"

[persistence]
driver = "mysql"
dsn = "root:root123@tcp(localhost:3306)/sfa_dips"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, `LogFormat: unsupported value "invalid"`)
}

func TestReadRejectsInvalidAPILogFormat(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api.log]
format = "invalid"

[persistence]
driver = "mysql"
dsn = "root:root123@tcp(localhost:3306)/sfa_dips"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, `unsupported log format: "invalid"`)
}

func TestReadRejectsInvalidAPILogLevel(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api.log]
level = "panic"

[persistence]
driver = "mysql"
dsn = "root:root123@tcp(localhost:3306)/sfa_dips"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, `invalid log level 'panic', valid values are: debug, info, warn, error`)
}

func TestReadRejectsMissingPersistenceConfiguration(t *testing.T) {
	t.Setenv("SFA_DIPS_PERSISTENCE_DRIVER", "")
	t.Setenv("SFA_DIPS_PERSISTENCE_DSN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api]
listen = "127.0.0.1:8080"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, "Persistence.Driver: missing required value")
	assert.ErrorContains(t, err, "Persistence.DSN: missing required value")
}

func TestReadLoadsPersistenceConfigurationFromEnvironment(t *testing.T) {
	t.Setenv("SFA_DIPS_PERSISTENCE_DRIVER", "mysql")
	t.Setenv("SFA_DIPS_PERSISTENCE_DSN", "env:env@tcp(env-mysql:3306)/env-dips")
	t.Setenv("SFA_DIPS_PERSISTENCE_MIGRATE", "true")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[persistence]
driver = "file-driver"
dsn = "file:file@tcp(file-mysql:3306)/file-dips"
migrate = false
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, cfg.Persistence.Driver, "mysql")
	assert.Equal(t, cfg.Persistence.DSN, "env:env@tcp(env-mysql:3306)/env-dips")
	assert.Equal(t, cfg.Persistence.Migrate, true)
}
