package promutils

import (
	"time"

	"github.com/VictoriaMetrics/metricsql"
)

// Duration is duration, which must be used in Prometheus-compatible yaml configs.
type Duration struct {
	D time.Duration
}

// NewDuration returns Duration for given d.
func NewDuration(d time.Duration) *Duration {
	return &Duration{
		D: d,
	}
}

// MarshalYAML implements yaml.Marshaler interface.
func (pd Duration) MarshalYAML() (interface{}, error) {
	return pd.D.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (pd *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	ms, err := metricsql.DurationValue(s, 0)
	if err != nil {
		return err
	}
	pd.D = time.Duration(ms) * time.Millisecond
	return nil
}

// Duration returns duration for pd.
func (pd *Duration) Duration() time.Duration {
	if pd == nil {
		return 0
	}
	return pd.D
}

// ParseDuration parses duration string in Prometheus format
func ParseDuration(s string) (time.Duration, error) {
	ms, err := metricsql.DurationValue(s, 0)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms) * time.Millisecond, nil
}
