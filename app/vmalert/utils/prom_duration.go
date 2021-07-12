package utils

import (
	"time"

	"github.com/VictoriaMetrics/metricsql"
)

// PromDuration is Prometheus duration.
type PromDuration struct {
	milliseconds int64
}

// NewPromDuration returns PromDuration for given d.
func NewPromDuration(d time.Duration) PromDuration {
	return PromDuration{
		milliseconds: d.Milliseconds(),
	}
}

// MarshalYAML implements yaml.Marshaler interface.
func (pd PromDuration) MarshalYAML() (interface{}, error) {
	return pd.Duration().String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (pd *PromDuration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	ms, err := metricsql.DurationValue(s, 0)
	if err != nil {
		return err
	}
	pd.milliseconds = ms
	return nil
}

// Duration returns duration for pd.
func (pd *PromDuration) Duration() time.Duration {
	return time.Duration(pd.milliseconds) * time.Millisecond
}
