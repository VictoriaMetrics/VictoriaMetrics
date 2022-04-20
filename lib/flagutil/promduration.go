package flagutil

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// NewPromDuration returns new prometheus-like `duration` flag with the given name, defaultValue and description.
func NewPromDuration(name string, defaultValue string, description string) *PromDuration {
	description += "\nThe following optional suffixes are supported: h (hour), d (day), w (week)."
	d := &PromDuration{}
	if err := d.Set(defaultValue); err != nil {
		panic(fmt.Sprintf("BUG: can not parse default value %s for flag %s", defaultValue, name))
	}
	flag.Var(d, name, description)
	return d
}

// PromDuration is a flag for holding prometheus-like duration.
type PromDuration struct {
	d time.Duration

	valueString string
}

// Set implements flag.Value interface
func (pd *PromDuration) Set(value string) error {
	parsed, err := promutils.ParseDuration(value)
	if err != nil {
		return err
	}
	pd.d = parsed
	pd.valueString = value
	return nil
}

// String implements flag.Value interface
func (pd *PromDuration) String() string {
	if pd == nil {
		return ""
	}
	return pd.d.String()
}

// Duration returns duration
func (pd *PromDuration) Duration() time.Duration {
	return pd.d
}
