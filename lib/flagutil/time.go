package flagutil

import (
	"flag"
	"fmt"
	"time"
)

// NewTime returns new `time` flag with the given name, defaultValue and description.
//
// DefaultValue is in time.Time.
func NewTime(name string, defaultValue string, description string) *Time {
	t := &Time{}
	if err := t.Set(defaultValue); err != nil {
		panic(fmt.Sprintf("BUG: can not parse default value %s for flag %s", defaultValue, name))
	}
	flag.Var(t, name, description)
	return t
}

// Time is a flag for holding time.Time value.
type Time struct {
	// Timestamp contains parsed duration in milliseconds.
	Timestamp time.Time

	location    *time.Location
	layout      string
	valueString string
}

// String implements flag.Value interface
func (t *Time) String() string {
	return t.valueString
}

// SetLayout sets the Time layout for future parsing
func (t *Time) SetLayout(layout string) {
	t.layout = layout
}

// SetLocation perceived timezone of the to-be parsed time string
func (t *Time) SetLocation(loc *time.Location) {
	t.location = loc
}

// Set implements flag.Value interface
func (t *Time) Set(value string) error {
	var timestamp time.Time
	var err error

	if t.location != nil {
		timestamp, err = time.ParseInLocation(t.layout, value, t.location)
	} else {
		timestamp, err = time.Parse(t.layout, value)
	}

	if err != nil {
		return err
	}

	t.Timestamp = timestamp
	return nil
}
