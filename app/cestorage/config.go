package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/VictoriaMetrics/metricsql"
	"gopkg.in/yaml.v2"
)

// Config represents the cestorage configuration.
type Config struct {
	Streams []EstimatorConfig `yaml:"streams"`
}

// EstimatorConfig represents a single cardinality estimator configuration.
type EstimatorConfig struct {
	Filter   string            `yaml:"filter"`   // optional: MetricsQL selector like {job="foo"}
	Group    []string          `yaml:"group"`    // optional: label names to split cardinality by
	Labels   map[string]string `yaml:"labels"`   // optional: extra labels added to output metrics
	Interval Duration          `yaml:"interval"` // optional: how often to rotate (reset) counters; 0 means no rotation
}

// Duration is a time.Duration that unmarshals from a YAML string like "1h", "30m", "5s".
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		*d = 0
		return nil
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("cannot parse duration %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

// labelFilter is a compiled label filter for fast matching.
type labelFilter struct {
	label      string
	value      string
	isNegative bool
	isRegexp   bool
	re         *regexp.Regexp // non-nil when isRegexp is true
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
		sort.Strings(stream.Group)
	}

	return &cfg, nil
}

func compileFilters(filter string) ([]labelFilter, error) {
	if filter == "" {
		return nil, nil
	}
	expr, err := metricsql.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("cannot parse filter %q: %w", filter, err)
	}
	me, ok := expr.(*metricsql.MetricExpr)
	if !ok {
		return nil, fmt.Errorf("filter %q must be a metric selector, got %T", filter, expr)
	}
	if len(me.LabelFilterss) == 0 {
		return nil, nil
	}
	// Use the first group of label filters (OR-groups are not supported).
	lfs := me.LabelFilterss[0]
	result := make([]labelFilter, 0, len(lfs))
	for _, lf := range lfs {
		f := labelFilter{
			label:      lf.Label,
			value:      lf.Value,
			isNegative: lf.IsNegative,
			isRegexp:   lf.IsRegexp,
		}
		if lf.IsRegexp {
			re, err := regexp.Compile("^(?:" + lf.Value + ")$")
			if err != nil {
				return nil, fmt.Errorf("cannot compile regexp %q in filter %q: %w", lf.Value, filter, err)
			}
			f.re = re
		}
		result = append(result, f)
	}
	return result, nil
}
