package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

func load(path string) (*Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	var cfg *Config
	err = yaml.Unmarshal(contents, &cfg)

	if err != nil {
		return nil, fmt.Errorf("error reading yaml: %v", err)
	}

	return cfg, nil
}

func New(configPath string) (*Config, error) {
	if configPath == "" {
		defaultPath, err := xdg.ConfigFile("homelabctl/config.yaml")
		if err != nil {
			return nil, fmt.Errorf("error getting default config path: %v", err)
		}
		configPath = defaultPath
	}
	slog.Info("Loading config", "path", configPath)

	return load(configPath)
}
