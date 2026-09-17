package config

import (
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

func load(configPath string) *Config {
	contents, err := os.ReadFile(configPath)
	if err != nil {
		slog.Error("failed to read file", "file", configPath, "error", err)
		return nil
	}

	var cfg *Config
	err = yaml.Unmarshal(contents, &cfg)

	if err != nil {
		slog.Error("failed to load config", "file", configPath, "error", err)
		return nil
	}

	return cfg
}

func New(configPath string) *Config {
	if configPath == "" {
		defaultPath, err := xdg.ConfigFile("homelabctl/config.yaml")
		if err != nil {
			slog.Error("failed to get default config path", "error", err)
			return nil
		}
		configPath = defaultPath
	}
	slog.Info("config path validated", "path", configPath)

	return load(configPath)
}
