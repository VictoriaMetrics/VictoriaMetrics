package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Streams []EstimatorConfig `yaml:"streams"`
}

type EstimatorConfig struct {
	GroupBy      []string          `yaml:"group_by"`
	GroupLimit   int               `yaml:"group_limit"`
	Labels       map[string]string `yaml:"labels"`
	Interval     time.Duration     `yaml:"interval"`
	Buckets      int               `yaml:"buckets"`
	HLLPrecision uint8             `yaml:"hll_precision"`
	HLLSparse    *bool             `yaml:"hll_sparse"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %q: %w", path, err)
	}
	for _, stream := range cfg.Streams {
		sort.Strings(stream.GroupBy)
		if stream.HLLPrecision != 0 && (stream.HLLPrecision < 4 || stream.HLLPrecision > 18) {
			return nil, fmt.Errorf("invalid precision %d: must be in range [4, 18]", stream.HLLPrecision)
		}
	}

	return &cfg, nil
}
