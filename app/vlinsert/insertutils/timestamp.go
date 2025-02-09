package insertutils

import (
	"fmt"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// ExtractTimestampFromFields extracts timestamp in nanoseconds from the field with the name timeField at fields.
//
// The value for the timeField is set to empty string after returning from the function,
// so it could be ignored during data ingestion.
//
// The current timestamp is returned if fields do not contain a field with timeField name or if the timeField value is empty.
func ExtractTimestampFromFields(timeField string, fields []logstorage.Field) (int64, error) {
	for i := range fields {
		f := &fields[i]
		if f.Name != timeField {
			continue
		}
		nsecs, err := parseTimestamp(f.Value)
		if err != nil {
			return 0, fmt.Errorf("cannot parse timestamp from field %q: %s", timeField, err)
		}
		f.Value = ""
		if nsecs == 0 {
			nsecs = time.Now().UnixNano()
		}
		return nsecs, nil
	}
	return time.Now().UnixNano(), nil
}

func parseTimestamp(s string) (int64, error) {
	if s == "" || s == "0" {
		return time.Now().UnixNano(), nil
	}
	if len(s) <= len("YYYY") || s[len("YYYY")] != '-' {
		return ParseUnixTimestamp(s)
	}
	nsecs, ok := logstorage.TryParseTimestampRFC3339Nano(s)
	if !ok {
		return 0, fmt.Errorf("cannot unmarshal rfc3339 timestamp %q", s)
	}
	return nsecs, nil
}

// ParseUnixTimestamp parses s as unix timestamp in seconds, milliseconds, microseconds or nanoseconds and returns the parsed timestamp in nanoseconds.
func ParseUnixTimestamp(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse unix timestamp from %q: %w", s, err)
	}
	if n < (1<<31) && n >= (-1<<31) {
		// The timestamp is in seconds.
		return n * 1e9, nil
	}
	if n < 1e3*(1<<31) && n >= 1e3*(-1<<31) {
		// The timestamp is in milliseconds.
		return n * 1e6, nil
	}
	if n < 1e6*(1<<31) && n >= 1e6*(-1<<31) {
		// The timestamp is in microseconds.
		return n * 1e3, nil
	}
	// The timestamp is in nanoseconds
	return n, nil
}
