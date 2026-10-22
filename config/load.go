package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	"github.com/pspiagicw/homelabctl/logging"
	"gopkg.in/yaml.v3"
)

func read(configPath string) (*Config, error) {
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file (%s): %v", configPath, err)
	}

	var cfg *Config
	err = yaml.Unmarshal(contents, &cfg)

	if err != nil {
		return nil, fmt.Errorf("failed to load config (%s): %v", configPath, err)
	}

	// An empty file unmarshals to a nil config without an error.
	if cfg == nil {
		return nil, fmt.Errorf("config is empty (%s)", configPath)
	}

	return cfg, nil
}

func New(configPath string) *Config {

	path, err := getConfigPath(configPath)
	if err != nil {
		logging.Fatal("failed to load config", "error", err)
	}

	conf, err := read(path)

	if err != nil {
		logging.Fatal("failed to load config", "error", err)
	}

	err = Validate(conf)

	if err != nil {
		logging.Fatal("failed to load config", "error", err)
	}

	slog.Info("config path validated", "path", path)

	return conf

}

func getConfigPath(configPath string) (string, error) {

	if configPath == "" {
		defaultPath, err := xdg.ConfigFile("homelabctl/config.yaml")
		if err != nil {
			return "", fmt.Errorf("failed to get default config path: %v", err)
		}
		configPath = defaultPath
	}

	return configPath, nil
}
