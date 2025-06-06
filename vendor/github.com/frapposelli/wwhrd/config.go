package main

import (
	"bytes"
	"gopkg.in/yaml.v2"
)

type OldConfig struct {
	Allowlist  []string `yaml:"whitelist"`
	Denylist   []string `yaml:"blacklist"`
	Exceptions []string `yaml:"exceptions"`
}

type Config struct {
	Allowlist  []string `yaml:"allowlist"`
	Denylist   []string `yaml:"denylist"`
	Exceptions []string `yaml:"exceptions"`
}

func ReadConfig(config []byte) (*Config, error) {

	t := Config{}
	old := OldConfig{}

	// Parse new format
	if err := yaml.NewDecoder(bytes.NewReader(config)).Decode(&t); err != nil {
		return nil, err
	}

	// Parse old format
	if err := yaml.NewDecoder(bytes.NewReader(config)).Decode(&old); err != nil {
		return nil, err
	}

	t.Allowlist = append(t.Allowlist, old.Allowlist...)
	t.Denylist = append(t.Denylist, old.Denylist...)
	t.Exceptions = append(t.Exceptions, old.Exceptions...)

	return &t, nil
}
