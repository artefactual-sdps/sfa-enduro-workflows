package config_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"go.artefactual.dev/tools/clientauth"
	"go.artefactual.dev/tools/log"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
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

const validACTAproConfig = `
[actapro]
url = "http://actapro.example.test"
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
workingDir = "/var/tmp/dips"

[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://example.test"

[api.log]
path = "stdout"
level = "WARN"
format = "text"

[actapro]
url = "http://actapro.example.test"
timeout = "12s"
pollInterval = "3s"
token = "test-token"

[actapro.oidc]
enabled = true
providerURL = "https://oidc.example.test"
tokenURL = "https://oidc.example.test/token"
clientID = "actapro"
clientSecret = "test-secret"
username = "test-user"
password = "test-password"
scopes = "read,write"
tokenExpiryLeeway = "45s"
retryMaxAttempts = 5
retryInitialInterval = "1s"
retryMaxInterval = "4s"
retryBackoffCoefficient = 3.0
`+validPersistenceConfig+validTemporalConfig))

	var cfg config.Config
	found, used, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, found, true)
	assert.Equal(t, used, tmpDir.Join("sfa-dips.toml"))
	assert.DeepEqual(t, cfg, config.Config{
		LogFormat:  config.LogFormatText,
		Verbosity:  2,
		WorkingDir: "/var/tmp/dips",
		API: api.Config{
			Listen:     "127.0.0.1:8080",
			CORSOrigin: "https://example.test",
			Log: api.LogConfig{
				Path:   "stdout",
				Level:  slog.LevelWarn,
				Format: api.LogFormatText,
			},
		},
		Persistence: persistence.Config{
			Driver:  "mysql",
			DSN:     "root:root123@tcp(localhost:3306)/sfa_dips",
			Migrate: true,
		},
		Temporal: config.TemporalConfig{
			Address:               "localhost:7233",
			Namespace:             "default",
			TaskQueue:             "sfa-dips",
			MaxConcurrentSessions: 1,
		},
		ACTApro: actapro.Config{
			URL:          "http://actapro.example.test",
			Timeout:      12 * time.Second,
			PollInterval: 3 * time.Second,
			Token:        "test-token",
			OIDC: actapro.OIDCConfig{
				Enabled: true,
				//nolint:staticcheck,gosec // ACTApro uses the legacy password grant; credentials are test fixtures.
				OIDCPasswordGrantAccessTokenProviderConfig: clientauth.OIDCPasswordGrantAccessTokenProviderConfig{
					ProviderURL:             "https://oidc.example.test",
					TokenURL:                "https://oidc.example.test/token",
					ClientID:                "actapro",
					ClientSecret:            "test-secret",
					Username:                "test-user",
					Password:                "test-password",
					Scopes:                  []string{"read", "write"},
					TokenExpiryLeeway:       45 * time.Second,
					RetryMaxAttempts:        5,
					RetryInitialInterval:    time.Second,
					RetryMaxInterval:        4 * time.Second,
					RetryBackoffCoefficient: 3.0,
				},
			},
		},
	})
}

func TestReadRejectsInvalidConfiguration(t *testing.T) {
	const invalidConfig = `
logFormat = "invalid"

[actapro]
timeout = "-1s"
pollInterval = "0s"

[actapro.oidc]
enabled = true
tokenExpiryLeeway = "-1s"
retryMaxAttempts = -1
retryInitialInterval = "-1s"
retryMaxInterval = "-2s"
retryBackoffCoefficient = 0.5

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
Temporal.MaxConcurrentSessions: must be greater than 0
ACTApro.URL: missing required value
ACTApro.Timeout: value -1s is less than 0
ACTApro.PollInterval: value 0s is less than or equal to 0
ACTApro.OIDC:
missing OIDC providerURL or tokenURL
missing OIDC client ID
missing OIDC resource owner credentials
invalid OIDC retry max attempts, value must be >= 1
invalid OIDC duration configuration, values must be > 0
invalid OIDC retry interval configuration, max interval must be >= initial interval
invalid OIDC retry backoff coefficient, value must be >= 1`,
	)
}

func TestReadLoadsConfigurationFromEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("SFA_DIPS_LOGFORMAT", "text")
	t.Setenv("SFA_DIPS_VERBOSITY", "2")
	t.Setenv("SFA_DIPS_WORKINGDIR", "/var/tmp/env-dips")
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
	t.Setenv("SFA_DIPS_ACTAPRO_URL", "http://actapro-env.example.test")
	t.Setenv("SFA_DIPS_ACTAPRO_TIMEOUT", "20s")
	t.Setenv("SFA_DIPS_ACTAPRO_POLLINTERVAL", "5s")
	t.Setenv("SFA_DIPS_ACTAPRO_TOKEN", "env-token")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_ENABLED", "true")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_PROVIDERURL", "https://oidc-env.example.test")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_TOKENURL", "https://oidc-env.example.test/token")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_CLIENTID", "env-actapro")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_CLIENTSECRET", "env-secret")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_USERNAME", "env-user")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_PASSWORD", "env-password")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_SCOPES", "read,export")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_TOKENEXPIRYLEEWAY", "60s")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_RETRYMAXATTEMPTS", "4")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_RETRYINITIALINTERVAL", "2s")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_RETRYMAXINTERVAL", "8s")
	t.Setenv("SFA_DIPS_ACTAPRO_OIDC_RETRYBACKOFFCOEFFICIENT", "4.0")

	var cfg config.Config
	found, used, err := config.Read(&cfg, "")

	assert.NilError(t, err)
	assert.Equal(t, found, false)
	assert.Equal(t, used, "")
	assert.DeepEqual(t, cfg, config.Config{
		LogFormat:  config.LogFormatText,
		Verbosity:  2,
		WorkingDir: "/var/tmp/env-dips",
		API: api.Config{
			Listen:     "127.0.0.1:8090",
			CORSOrigin: "https://env.example.test",
			Log: api.LogConfig{
				Path:   "stderr",
				Level:  slog.LevelWarn,
				Format: api.LogFormatText,
			},
		},
		Persistence: persistence.Config{
			Driver:  "mysql",
			DSN:     "env:env@tcp(env-mysql:3306)/env-dips",
			Migrate: true,
		},
		Temporal: config.TemporalConfig{
			Address:               "temporal:7233",
			Namespace:             "env-namespace",
			TaskQueue:             "env-dips",
			MaxConcurrentSessions: 3,
		},
		ACTApro: actapro.Config{
			URL:          "http://actapro-env.example.test",
			Timeout:      20 * time.Second,
			PollInterval: 5 * time.Second,
			Token:        "env-token",
			OIDC: actapro.OIDCConfig{
				Enabled: true,
				//nolint:staticcheck,gosec // ACTApro uses the legacy password grant; credentials are test fixtures.
				OIDCPasswordGrantAccessTokenProviderConfig: clientauth.OIDCPasswordGrantAccessTokenProviderConfig{
					ProviderURL:             "https://oidc-env.example.test",
					TokenURL:                "https://oidc-env.example.test/token",
					ClientID:                "env-actapro",
					ClientSecret:            "env-secret",
					Username:                "env-user",
					Password:                "env-password",
					Scopes:                  []string{"read", "export"},
					TokenExpiryLeeway:       time.Minute,
					RetryMaxAttempts:        4,
					RetryInitialInterval:    2 * time.Second,
					RetryMaxInterval:        8 * time.Second,
					RetryBackoffCoefficient: 4.0,
				},
			},
		},
	})
}

func TestReadSetsDefaults(t *testing.T) {
	t.Setenv("SFA_DIPS_API_CORSORIGIN", "")
	tmpDir := fs.NewDir(
		t,
		"",
		fs.WithFile("sfa-dips.toml", validPersistenceConfig+validTemporalConfig+validACTAproConfig),
	)

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.DeepEqual(t, cfg, config.Config{
		LogFormat:  config.LogFormatJSON,
		WorkingDir: os.TempDir(),
		API: api.Config{
			Listen:     "127.0.0.1:8080",
			CORSOrigin: "127.0.0.1:8080",
			Log: api.LogConfig{
				Level:  slog.LevelInfo,
				Format: api.LogFormatJSON,
			},
		},
		Persistence: persistence.Config{
			Driver:  "mysql",
			DSN:     "root:root123@tcp(localhost:3306)/sfa_dips",
			Migrate: true,
		},
		Temporal: config.TemporalConfig{
			Address:               "localhost:7233",
			Namespace:             "default",
			TaskQueue:             "sfa-dips",
			MaxConcurrentSessions: 1,
		},
		ACTApro: actapro.Config{
			URL:          "http://actapro.example.test",
			Timeout:      actapro.DefaultTimeout,
			PollInterval: actapro.DefaultPollInterval,
		},
	})
}

func TestReadSetsCORSOriginEnvironment(t *testing.T) {
	t.Setenv("SFA_DIPS_API_CORSORIGIN", "")
	tmpDir := fs.NewDir(t, "", fs.WithFile("sfa-dips.toml", `
[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://example.test"
`+validPersistenceConfig+validTemporalConfig+validACTAproConfig))

	var cfg config.Config
	_, _, err := config.Read(&cfg, tmpDir.Join("sfa-dips.toml"))

	assert.NilError(t, err)
	assert.Equal(t, os.Getenv("SFA_DIPS_API_CORSORIGIN"), "https://example.test")
}
