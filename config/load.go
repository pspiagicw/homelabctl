package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	var cfg *Config
	err = yaml.Unmarshal(contents, cfg)

	if err != nil {
		return nil, fmt.Errorf("error reading yaml: %v", err)
	}

	return cfg, nil
}
