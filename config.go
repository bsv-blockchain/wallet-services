package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/bsv-blockchain/arcade/config"
)

// loadConfig reads configuration from file and environment variables.
// It searches for config.yaml in the current directory, $HOME/.bws, and /etc/bws.
// Environment variables use the BWS_ prefix with dots replaced by underscores,
// e.g. BWS_TERANODE_BROADCAST_URLS maps to teranode.broadcast_urls.
func loadConfig() (*config.Config, error) {
	v := viper.New()

	cfg := &config.Config{}
	cfg.SetDefaults(v, "")

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.bws")
	v.AddConfigPath("/etc/bws")

	v.SetEnvPrefix("BWS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var cfgErr viper.ConfigFileNotFoundError
		if !errors.As(err, &cfgErr) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return cfg, nil
}
