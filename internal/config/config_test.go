package config_test

import (
	"testing"
	"time"

	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"go.artefactual.dev/ssclient"
	"go.artefactual.dev/tools/bucket"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
)

const testConfig = `# Config
debug = true
verbosity = 2
[temporal]
address = "host:port"
namespace = "default"
[worker]
maxConcurrentSessions = 1
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[preprocessing.bagCreate]
checksumAlgorithm = "md5"
[preprocessing.bagValidate]
cacheDir = "/home/enduro/.cache/bagit-gython"
poolSize = 2
[preprocessing.fileFormat]
allowlistPath = "/home/enduro/.config/allowed_file_formats.csv"
[preprocessing.filevalidate.verapdf]
path = "/opt/verapdf/verapdf"
[poststorage]
workingDir = "/tmp"
[poststorage.apis]
workflowName = "poststorage-apis"
[poststorage.cantons]
workflowName = "poststorage-cantons"
[poststorage.cantons.bucket]
url = "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true"
[poststorage.amss]
baseURL = "http://amss.example.test"
username = "test"
key = "test"
[apis]
enabled = true
url = "http://apis.example.test"
`

const validPoststorageConfig = `
[poststorage]
workingDir = "/tmp"
[poststorage.apis]
workflowName = "poststorage-apis"
[poststorage.cantons]
workflowName = "poststorage-cantons"
[poststorage.cantons.bucket]
url = "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true"
[poststorage.amss]
baseURL = "http://amss.example.test"
username = "test"
key = "test"
`

func TestConfig(t *testing.T) {
	t.Parallel()

	type test struct {
		name            string
		configFile      string
		toml            string
		wantFound       bool
		wantCfg         config.Config
		wantErr         string
		wantErrContains string
	}

	for _, tc := range []test{
		{
			name:       "Loads configuration from a TOML file",
			configFile: "sfa-enduro-worker.toml",
			toml:       testConfig,
			wantFound:  true,
			wantCfg: config.Config{
				Debug:     true,
				Verbosity: 2,
				Temporal: config.TemporalConfig{
					Address:   "host:port",
					Namespace: "default",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
					TaskQueue:             "sfa-enduro",
				},
				APIS: apis.Config{
					Enabled:      true,
					URL:          "http://apis.example.test",
					Timeout:      apis.DefaultTimeout,
					PollInterval: apis.DefaultPollInterval,
				},
				Preprocessing: config.PreprocessingConfig{
					WorkflowName: "preprocessing",
					SharedPath:   "/home/enduro/shared",
					BagCreate: bagcreate.Config{
						ChecksumAlgorithm: "md5",
					},
					BagValidate: config.BagValidator{
						CacheDir: "/home/enduro/.cache/bagit-gython",
						PoolSize: 2,
					},
					FileFormat: ffvalidate.Config{
						AllowlistPath: "/home/enduro/.config/allowed_file_formats.csv",
					},
					FileValidate: fvalidate.Config{
						VeraPDF: fvalidate.VeraPDFConfig{
							Path: "/opt/verapdf/verapdf",
						},
					},
				},
				Poststorage: config.PoststorageConfig{
					WorkingDir: "/tmp",
					APIS: config.PoststorageAPISConfig{
						WorkflowName: "poststorage-apis",
					},
					Cantons: config.PoststorageCantonsConfig{
						WorkflowName: "poststorage-cantons",
						Bucket: bucket.Config{
							URL: "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true",
						},
					},
					AMSS: ssclient.Config{
						BaseURL:  "http://amss.example.test",
						Username: "test",
						Key:      "test",
					},
				},
			},
		},
		{
			name:       "Errors when configuration values are not valid",
			configFile: "sfa-enduro-worker.toml",
			toml: `# override default values to trigger validation errors
[temporal]
namespace = ""
[preprocessing.bagValidate]
poolSize = 0
`,
			wantFound: true,
			wantErr: `invalid configuration
Temporal.Address: missing required value
Temporal.Namespace: missing required value
Worker.TaskQueue: missing required value
Preprocessing.SharedPath: missing required value
Preprocessing.WorkflowName: missing required value
Preprocessing.BagValidate: PoolSize: 0 is less than the minimum value (1)
Poststorage.WorkingDir: missing required value
Poststorage.AMSS.BaseURL: missing required value
Poststorage.AMSS.Username: missing required value
Poststorage.AMSS.Key: missing required value
Poststorage.Cantons.WorkflowName: missing required value
Poststorage.Cantons.Bucket: missing required value`,
		},
		{
			name:       "Errors when MaxConcurrentSessions is less than 1",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
maxConcurrentSessions = -1
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
` + validPoststorageConfig,
			wantFound: true,
			wantErr: `invalid configuration
Worker.MaxConcurrentSessions: -1 is less than the minimum value (1)`,
		},
		{
			name:       "Errors when bagcreate checksumAlgorithm is invalid",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[preprocessing.bagCreate]
checksumAlgorithm = "unknown"
` + validPoststorageConfig,
			wantFound: true,
			wantErr: `invalid configuration
Preprocessing.BagCreate: ChecksumAlgorithm: invalid value "unknown", must be one of (md5, sha1, sha256, sha512)`,
		},
		{
			name:       "Loads APIS defaults when only URL is configured",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[apis]
enabled = true
url = "http://apis.example.test"
` + validPoststorageConfig,
			wantFound: true,
			wantCfg: config.Config{
				Temporal: config.TemporalConfig{
					Address:   "host:port",
					Namespace: "default",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
					TaskQueue:             "sfa-enduro",
				},
				APIS: apis.Config{
					Enabled:      true,
					URL:          "http://apis.example.test",
					Timeout:      apis.DefaultTimeout,
					PollInterval: apis.DefaultPollInterval,
				},
				Preprocessing: config.PreprocessingConfig{
					WorkflowName: "preprocessing",
					SharedPath:   "/home/enduro/shared",
					BagCreate: bagcreate.Config{
						ChecksumAlgorithm: "sha512",
					},
					BagValidate: config.BagValidator{
						PoolSize: 1,
					},
				},
				Poststorage: config.PoststorageConfig{
					WorkingDir: "/tmp",
					APIS: config.PoststorageAPISConfig{
						WorkflowName: "poststorage-apis",
					},
					Cantons: config.PoststorageCantonsConfig{
						WorkflowName: "poststorage-cantons",
						Bucket: bucket.Config{
							URL: "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true",
						},
					},
					AMSS: ssclient.Config{
						BaseURL:  "http://amss.example.test",
						Username: "test",
						Key:      "test",
					},
				},
			},
		},
		{
			name:       "Loads APIS poststorage config without Cantons config when APIS is enabled",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[poststorage]
workingDir = "/tmp"
[poststorage.apis]
workflowName = "poststorage-apis"
[poststorage.amss]
baseURL = "http://amss.example.test"
username = "test"
key = "test"
[apis]
enabled = true
url = "http://apis.example.test"
`,
			wantFound: true,
			wantCfg: config.Config{
				Temporal: config.TemporalConfig{
					Address:   "host:port",
					Namespace: "default",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
					TaskQueue:             "sfa-enduro",
				},
				APIS: apis.Config{
					Enabled:      true,
					URL:          "http://apis.example.test",
					Timeout:      apis.DefaultTimeout,
					PollInterval: apis.DefaultPollInterval,
				},
				Preprocessing: config.PreprocessingConfig{
					WorkflowName: "preprocessing",
					SharedPath:   "/home/enduro/shared",
					BagCreate: bagcreate.Config{
						ChecksumAlgorithm: "sha512",
					},
					BagValidate: config.BagValidator{
						PoolSize: 1,
					},
				},
				Poststorage: config.PoststorageConfig{
					WorkingDir: "/tmp",
					APIS: config.PoststorageAPISConfig{
						WorkflowName: "poststorage-apis",
					},
					AMSS: ssclient.Config{
						BaseURL:  "http://amss.example.test",
						Username: "test",
						Key:      "test",
					},
				},
			},
		},
		{
			name:       "Loads Cantons poststorage config without APIS poststorage config when APIS is disabled",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[poststorage]
workingDir = "/tmp"
[poststorage.cantons]
workflowName = "poststorage-cantons"
[poststorage.cantons.bucket]
url = "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true"
[poststorage.amss]
baseURL = "http://amss.example.test"
username = "test"
key = "test"
`,
			wantFound: true,
			wantCfg: config.Config{
				Temporal: config.TemporalConfig{
					Address:   "host:port",
					Namespace: "default",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
					TaskQueue:             "sfa-enduro",
				},
				APIS: apis.Config{
					Timeout:      apis.DefaultTimeout,
					PollInterval: apis.DefaultPollInterval,
				},
				Preprocessing: config.PreprocessingConfig{
					WorkflowName: "preprocessing",
					SharedPath:   "/home/enduro/shared",
					BagCreate: bagcreate.Config{
						ChecksumAlgorithm: "sha512",
					},
					BagValidate: config.BagValidator{
						PoolSize: 1,
					},
				},
				Poststorage: config.PoststorageConfig{
					WorkingDir: "/tmp",
					Cantons: config.PoststorageCantonsConfig{
						WorkflowName: "poststorage-cantons",
						Bucket: bucket.Config{
							URL: "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true",
						},
					},
					AMSS: ssclient.Config{
						BaseURL:  "http://amss.example.test",
						Username: "test",
						Key:      "test",
					},
				},
			},
		},
		{
			name:       "Errors when APIS URL is missing",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[apis]
enabled = true
` + validPoststorageConfig,
			wantFound: true,
			wantErr: `invalid configuration
APIS.URL: missing required value`,
		},
		{
			name:       "Errors when APIS poststorage workflow name is missing and APIS is enabled",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[poststorage]
workingDir = "/tmp"
[poststorage.amss]
baseURL = "http://amss.example.test"
username = "test"
key = "test"
[apis]
enabled = true
url = "http://apis.example.test"
`,
			wantFound: true,
			wantErr: `invalid configuration
Poststorage.APIS.WorkflowName: missing required value`,
		},
		{
			name:       "Errors when Cantons poststorage is missing and APIS is disabled",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[poststorage]
workingDir = "/tmp"
[poststorage.amss]
baseURL = "http://amss.example.test"
username = "test"
key = "test"
[apis]
enabled = false
`,
			wantFound: true,
			wantErr: `invalid configuration
Poststorage.Cantons.WorkflowName: missing required value
Poststorage.Cantons.Bucket: missing required value`,
		},
		{
			name:       "Errors when APIS timeout is invalid",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[apis]
enabled = true
url = "http://apis.example.test"
timeout = "-1s"
` + validPoststorageConfig,
			wantFound: true,
			wantErr: `invalid configuration
APIS.Timeout: value -1s is less than 0`,
		},
		{
			name:       "Errors when APIS poll interval is invalid",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[apis]
enabled = true
url = "http://apis.example.test"
pollInterval = "-1s"
` + validPoststorageConfig,
			wantFound: true,
			wantErr: `invalid configuration
APIS.PollInterval: value -1s is less than or equal to 0`,
		},
		{
			name:       "Loads explicit APIS timeout and poll interval",
			configFile: "sfa-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "sfa-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[apis]
enabled = true
url = "http://apis.example.test"
timeout = "45s"
pollInterval = "2m"
token = "mock-token"
` + validPoststorageConfig,
			wantFound: true,
			wantCfg: config.Config{
				Temporal: config.TemporalConfig{
					Address:   "host:port",
					Namespace: "default",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
					TaskQueue:             "sfa-enduro",
				},
				APIS: apis.Config{
					Enabled:      true,
					URL:          "http://apis.example.test",
					Timeout:      45 * time.Second,
					PollInterval: 2 * time.Minute,
					Token:        "mock-token",
				},
				Preprocessing: config.PreprocessingConfig{
					WorkflowName: "preprocessing",
					SharedPath:   "/home/enduro/shared",
					BagCreate: bagcreate.Config{
						ChecksumAlgorithm: "sha512",
					},
					BagValidate: config.BagValidator{
						PoolSize: 1,
					},
				},
				Poststorage: config.PoststorageConfig{
					WorkingDir: "/tmp",
					APIS: config.PoststorageAPISConfig{
						WorkflowName: "poststorage-apis",
					},
					Cantons: config.PoststorageCantonsConfig{
						WorkflowName: "poststorage-cantons",
						Bucket: bucket.Config{
							URL: "file:///home/enduro/cantons?metadata=skip&no_tmp_dir=true&create_dir=true",
						},
					},
					AMSS: ssclient.Config{
						BaseURL:  "http://amss.example.test",
						Username: "test",
						Key:      "test",
					},
				},
			},
		},
		{
			name:       "Errors when TOML is invalid",
			configFile: "sfa-enduro-worker.toml",
			toml:       "bad TOML",
			wantFound:  true,
			wantErr:    "failed to read configuration file: While parsing config: toml: expected character =",
		},
		{
			name:            "Errors when no config file is found in the default paths",
			wantFound:       false,
			wantErrContains: "Config File \"sfa-enduro-worker\" Not Found in \"[",
		},
		{
			name:            "Errors when the given configFile is not found",
			configFile:      "missing.toml",
			wantFound:       false,
			wantErrContains: "configuration file not found: ",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := fs.NewDir(t, "sfa-enduro-worker-test", fs.WithFile("sfa-enduro-worker.toml", tc.toml))

			configFile := ""
			if tc.configFile != "" {
				configFile = tmpDir.Join(tc.configFile)
			}

			var c config.Config
			found, configFileUsed, err := config.Read(&c, configFile)
			if tc.wantErr != "" {
				assert.Equal(t, found, tc.wantFound)
				assert.Error(t, err, tc.wantErr)
				return
			}
			if tc.wantErrContains != "" {
				assert.Equal(t, found, tc.wantFound)
				assert.ErrorContains(t, err, tc.wantErrContains)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, found, true)
			assert.Equal(t, configFileUsed, configFile)
			assert.DeepEqual(t, c, tc.wantCfg)
		})
	}
}
