package flagutil

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metricsql"
)

// NewDuration returns new `duration` flag with the given name, defaultValue and description.
//
// DefaultValue is in months.
func NewDuration(name string, defaultValue string, description string) *Duration {
	description += "\nThe following optional suffixes are supported: s (second), m (minute), h (hour), d (day), w (week), y (year). " +
		"If suffix isn't set, then the duration is counted in months"
	d := &Duration{}
	if err := d.Set(defaultValue); err != nil {
		panic(fmt.Sprintf("BUG: can not parse default value %s for flag %s", defaultValue, name))
	}
	flag.Var(d, name, description)
	return d
}

// Duration is a flag for holding duration.
type Duration struct {
	// msecs contains parsed duration in milliseconds.
	msecs int64

	valueString string
}

// Duration returns d as time.Duration
func (d *Duration) Duration() time.Duration {
	return time.Millisecond * time.Duration(d.msecs)
}

// Milliseconds returns d in milliseconds
func (d *Duration) Milliseconds() int64 {
	return d.msecs
}

// String implements flag.Value interface
func (d *Duration) String() string {
	return d.valueString
}

// Set implements flag.Value interface
func (d *Duration) Set(value string) error {
	if value == "" {
		d.msecs = 0
		d.valueString = ""
		return nil
	}
	// An attempt to parse value in months.
	months, err := strconv.ParseFloat(value, 64)
	if err == nil {
		if months > maxMonths {
			return fmt.Errorf("duration months must be smaller than %d; got %g", maxMonths, months)
		}
		if months < 0 {
			return fmt.Errorf("duration months cannot be negative; got %g", months)
		}
		d.msecs = int64(months * msecsPer31Days)
		d.valueString = value
		return nil
	}
	// Parse duration.
	value = strings.ToLower(value)
	if strings.HasSuffix(value, "m") {
		return fmt.Errorf("duration in months must be set without `m` suffix due to ambiguity with duration in minutes; got %s", value)
	}
	msecs, err := metricsql.PositiveDurationValue(value, 0)
	if err != nil {
		return err
	}
	if msecs/msecsPer31Days > maxMonths {
		return fmt.Errorf("duration must be smaller than %d months; got approx %d months", maxMonths, msecs/msecsPer31Days)
	}
	d.msecs = msecs
	d.valueString = value
	return nil
}

const maxMonths = 12 * 100
const msecsPer31Days = 31 * 24 * 3600 * 1000
