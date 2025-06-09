package flagutil

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metricsql"
)

// NewRetentionDuration returns new `duration` flag with the given name, defaultValue and description.
//
// DefaultValue is in months.
func NewRetentionDuration(name string, defaultValue string, description string) *RetentionDuration {
	description += "\nThe following optional suffixes are supported: s (second), h (hour), d (day), w (week), y (year). " +
		"If suffix isn't set, then the duration is counted in months"
	d := &RetentionDuration{}
	if err := d.Set(defaultValue); err != nil {
		panic(fmt.Sprintf("BUG: can not parse default value %s for flag %s", defaultValue, name))
	}
	flag.Var(d, name, description)
	return d
}

// RetentionDuration is a flag for holding duration for retention period.
type RetentionDuration struct {
	// msecs contains parsed duration in milliseconds.
	msecs int64

	valueString string
}

var (
	_ json.Marshaler   = (*RetentionDuration)(nil)
	_ json.Unmarshaler = (*RetentionDuration)(nil)
)

// MarshalJSON implements json.Marshaler interface
func (d *RetentionDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.valueString)
}

// UnmarshalJSON implements json.Unmarshaler interface
func (d *RetentionDuration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return d.Set(s)
}

// Duration returns d as time.Duration
func (d *RetentionDuration) Duration() time.Duration {
	return time.Millisecond * time.Duration(d.msecs)
}

// Milliseconds returns d in milliseconds
func (d *RetentionDuration) Milliseconds() int64 {
	return d.msecs
}

// String implements flag.Value interface
func (d *RetentionDuration) String() string {
	return d.valueString
}

// Set implements flag.Value interface
// It assumes that value without unit should be parsed as `month` duration.
// It returns an error if value has `m` unit.
func (d *RetentionDuration) Set(value string) error {
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
