package flagutil

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/metricsql"
)

// NewDuration returns new `duration` flag with the given name, defaultValue and description.
//
// DefaultValue is in months.
func NewDuration(name string, defaultValue float64, description string) *Duration {
	description += "\nThe following optional suffixes are supported: h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months"
	d := Duration{
		Msecs:       int64(defaultValue * msecsPerMonth),
		valueString: fmt.Sprintf("%g", defaultValue),
	}
	flag.Var(&d, name, description)
	return &d
}

// Duration is a flag for holding duration.
type Duration struct {
	// Msecs contains parsed duration in milliseconds.
	Msecs int64

	valueString string
}

// String implements flag.Value interface
func (d *Duration) String() string {
	return d.valueString
}

// Set implements flag.Value interface
func (d *Duration) Set(value string) error {
	// An attempt to parse value in months.
	months, err := strconv.ParseFloat(value, 64)
	if err == nil {
		if months > maxMonths {
			return fmt.Errorf("duration months must be smaller than %d; got %g", maxMonths, months)
		}
		if months < 0 {
			return fmt.Errorf("duration months cannot be negative; got %g", months)
		}
		d.Msecs = int64(months * msecsPerMonth)
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
	d.Msecs = msecs
	d.valueString = value
	return nil
}

const maxMonths = 12 * 100

const msecsPerMonth = 31 * 24 * 3600 * 1000
