package promutil

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
func (pd Duration) MarshalYAML() (any, error) {
	return pd.D.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (pd *Duration) UnmarshalYAML(unmarshal func(any) error) error {
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
