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
}

func TestReadAllowsMissingImplicitConfigFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	var cfg config.Config
	found, used, err := config.Read(&cfg, "")

	assert.NilError(t, err)
	assert.Equal(t, found, false)
	assert.Equal(t, used, "")
}

func TestReadSetsDefaults(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", "# empty config\n"))

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
	t.Setenv("SFA_DIPS_API_CORS_ORIGIN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://example.test"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, os.Getenv("SFA_DIPS_API_CORS_ORIGIN"), "https://example.test")
}

func TestReadRejectsInvalidApplicationLogFormat(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
logFormat = "invalid"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, `LogFormat: unsupported value "invalid"`)
}

func TestReadRejectsInvalidAPILogFormat(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api.log]
format = "invalid"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, `unsupported log format: "invalid"`)
}

func TestReadRejectsInvalidAPILogLevel(t *testing.T) {
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api.log]
level = "panic"
`))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.ErrorContains(t, err, `invalid log level 'panic', valid values are: debug, info, warn, error`)
}
