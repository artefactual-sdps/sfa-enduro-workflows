package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Debug     bool
	Verbosity int
	API       APIConfig
}

type APIConfig struct {
	Listen     string
	Debug      bool
	CORSOrigin string
}

func Read(config *Config, configFile string) (found bool, configFileUsed string, err error) {
	v := viper.New()

	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/")
	v.AddConfigPath("/etc")
	v.SetConfigName("sfa-dips")
	v.SetEnvPrefix("SFA_DIPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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

	return true, v.ConfigFileUsed(), nil
}
