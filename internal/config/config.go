package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"github.com/spf13/viper"
	"go.artefactual.dev/ssclient"
	"go.artefactual.dev/tools/bucket"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence"
)

type ConfigurationValidator interface {
	Validate() error
}

type Config struct {
	// Debug toggles human readable logs or JSON logs (default).
	Debug bool

	// Verbosity sets the verbosity level of log messages, with 0 (default)
	// logging only critical messages and each higher number increasing the
	// number of messages logged.
	Verbosity int

	// Temporal configures the Temporal client.
	Temporal TemporalConfig

	// Worker configures the Temporal worker.
	Worker WorkerConfig

	// Preprocessing configures the preprocessing workflow.
	Preprocessing PreprocessingConfig

	// Poststorage configures the poststorage workflow.
	Poststorage PoststorageConfig

	// APIS configures the APIS client shared by workflows.
	APIS apis.Config
}

type TemporalConfig struct {
	// Address is the Temporal server host and port (required).
	Address string

	// Namespace is the Temporal client namespace (default: "default").
	Namespace string
}

func (c TemporalConfig) Validate() error {
	var errs error

	if c.Address == "" {
		errs = errors.Join(errs, errRequired("Temporal.Address"))
	}
	if c.Namespace == "" {
		errs = errors.Join(errs, errRequired("Temporal.Namespace"))
	}

	return errs
}

type WorkerConfig struct {
	// MaxConcurrentSessions limits the number of workflow sessions the worker
	// can handle simultaneously (default: 1).
	MaxConcurrentSessions int

	// TaskQueue is the Temporal task queue from which the worker will pull
	// tasks (required).
	TaskQueue string
}

func (c WorkerConfig) Validate() error {
	var errs error

	if c.TaskQueue == "" {
		errs = errors.Join(errs, errRequired("Worker.TaskQueue"))
	}

	// Verify that MaxConcurrentSessions is >= 1.
	if c.MaxConcurrentSessions < 1 {
		errs = errors.Join(errs, fmt.Errorf(
			"Worker.MaxConcurrentSessions: %d is less than the minimum value (1)",
			c.MaxConcurrentSessions,
		))
	}

	return errs
}

type PreprocessingConfig struct {
	// WorkflowName is the preprocessing Temporal workflow name (required).
	WorkflowName string

	// SharedPath is a file path that both Preprocessing and Enduro can access
	// (required).
	//
	// Enduro will deposit transfers in SharedPath for preprocessing.
	// Preprocessing must write transfer updates to SharedPath for retrieval by
	// Enduro and preservation processing.
	SharedPath string

	// CheckDuplicates enables or disables a check for SIPs that have already
	// been processed. When enabled, the persistence configuration below will
	// be required, and a SIP that has already been processed will fail the
	// preprocessing workflow.
	CheckDuplicates bool

	Persistence persistence.Config
	BagCreate   bagcreate.Config

	FileFormat   ffvalidate.Config
	FileValidate fvalidate.Config
}

func (c PreprocessingConfig) Validate() error {
	var errs error

	if c.SharedPath == "" {
		errs = errors.Join(errs, errRequired("Preprocessing.SharedPath"))
	}
	if c.WorkflowName == "" {
		errs = errors.Join(errs, errRequired("Preprocessing.WorkflowName"))
	}

	if err := c.BagCreate.Validate(); err != nil {
		errs = errors.Join(errs, fmt.Errorf("Preprocessing.BagCreate: %v", err))
	}

	if c.CheckDuplicates {
		if c.Persistence.DSN == "" {
			errs = errors.Join(errs, errRequired("Preprocessing.Persistence.DSN"))
		}
		if c.Persistence.Driver == "" {
			errs = errors.Join(errs, errRequired("Preprocessing.Persistence.Driver"))
		}
	}

	return errs
}

type PoststorageConfig struct {
	// WorkingDir is used to download and prepare poststorage files (required).
	WorkingDir string

	// APIS configures the APIS poststorage workflow.
	APIS PoststorageAPISConfig

	// Cantons configures the Cantons poststorage workflow.
	Cantons PoststorageCantonsConfig

	AMSS ssclient.Config
}

type PoststorageAPISConfig struct {
	// WorkflowName is the APIS poststorage Temporal workflow name (required).
	WorkflowName string
}

type PoststorageCantonsConfig struct {
	// WorkflowName is the Cantons poststorage Temporal workflow name (required).
	WorkflowName string

	// Bucket is the destination for Cantons metadata bundles.
	Bucket bucket.Config
}

func (c PoststorageConfig) Validate() error {
	var errs error

	if c.WorkingDir == "" {
		errs = errors.Join(errs, errRequired("Poststorage.WorkingDir"))
	}
	if c.AMSS.BaseURL == "" {
		errs = errors.Join(errs, errRequired("Poststorage.AMSS.BaseURL"))
	}
	if c.AMSS.Username == "" {
		errs = errors.Join(errs, errRequired("Poststorage.AMSS.Username"))
	}
	if c.AMSS.Key == "" {
		errs = errors.Join(errs, errRequired("Poststorage.AMSS.Key"))
	}

	return errs
}

func (c PoststorageAPISConfig) Validate() error {
	var errs error

	if c.WorkflowName == "" {
		errs = errors.Join(errs, errRequired("Poststorage.APIS.WorkflowName"))
	}

	return errs
}

func (c PoststorageCantonsConfig) Validate() error {
	var errs error

	if c.WorkflowName == "" {
		errs = errors.Join(errs, errRequired("Poststorage.Cantons.WorkflowName"))
	}
	if c.Bucket.URL == "" && c.Bucket.Endpoint == "" {
		errs = errors.Join(errs, errRequired("Poststorage.Cantons.Bucket"))
	}

	return errs
}

func (c Config) Validate() error {
	errs := errors.Join(
		c.Temporal.Validate(),
		c.Worker.Validate(),
		c.Preprocessing.Validate(),
		c.Poststorage.Validate(),
		c.APIS.Validate(),
	)

	if c.APIS.Enabled {
		errs = errors.Join(errs, c.Poststorage.APIS.Validate())
	} else {
		errs = errors.Join(errs, c.Poststorage.Cantons.Validate())
	}

	return errs
}

func Read(config *Config, configFile string) (found bool, configFileUsed string, err error) {
	v := viper.New()

	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/")
	v.AddConfigPath("/etc")
	v.SetConfigName("sfa-enduro-worker")
	v.SetEnvPrefix("SFA_ENDURO_WORKER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults.
	v.SetDefault("APIS.Timeout", apis.DefaultTimeout)
	v.SetDefault("APIS.PollInterval", apis.DefaultPollInterval)
	v.SetDefault("Temporal.Namespace", "default")
	v.SetDefault("Worker.MaxConcurrentSessions", 1)
	v.SetDefault("Preprocessing.BagCreate.ChecksumAlgorithm", "sha512")

	if configFile != "" {
		// Viper will not return a viper.ConfigFileNotFoundError error when
		// SetConfigFile() is passed a path to a file that doesn't exist, so we
		// need to check ourselves.
		if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
			return false, "", fmt.Errorf("configuration file not found: %s", configFile)
		}

		v.SetConfigFile(configFile)
	}

	if err = v.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			return false, "", err
		default:
			return true, "", fmt.Errorf("failed to read configuration file: %w", err)
		}
	}

	err = v.Unmarshal(config)
	if err != nil {
		return true, "", fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := config.Validate(); err != nil {
		return true, "", errors.Join(errors.New("invalid configuration"), err)
	}

	return true, v.ConfigFileUsed(), nil
}

func errRequired(name string) error {
	return fmt.Errorf("%s: missing required value", name)
}
