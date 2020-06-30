package csvimport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fastjson/fastfloat"
)

// ColumnDescriptor represents parsing rules for a single csv column.
//
// The column is transformed to either timestamp, tag or metric value
// depending on the corresponding non-empty field.
//
// If all the fields are empty, then the given column is ignored.
type ColumnDescriptor struct {
	// ParseTimestamp is set to a function, which is used for timestamp
	// parsing from the given column.
	ParseTimestamp func(s string) (int64, error)

	// TagName is set to tag name for tag value, which should be obtained
	// from the given column.
	TagName string

	// MetricName is set to metric name for value obtained from the given column.
	MetricName string
}

const maxColumnsPerRow = 64 * 1024

// ParseColumnDescriptors parses column descriptors from s.
//
// s must have comma-separated list of the following entries:
//
//     <column_pos>:<column_type>:<extension>
//
// Where:
//
//   - <column_pos> is numeric csv column position. The first column has position 1.
//   - <column_type> is one of the following types:
//     - time - the corresponding column contains timestamp. Timestamp format is determined by <extension>. The following formats are supported:
//       - unix_s - unix timestamp in seconds
//       - unix_ms - unix timestamp in milliseconds
//       - unix_ns - unix_timestamp in nanoseconds
//       - rfc3339 - RFC3339 format in the form `2006-01-02T15:04:05Z07:00`
//     - label - the corresponding column contains metric label with the name set in <extension>.
//     - metric - the corresponding column contains metric value with the name set in <extension>.
//
// s must contain at least a single 'metric' column and no more than a single `time` column.
func ParseColumnDescriptors(s string) ([]ColumnDescriptor, error) {
	m := make(map[int]ColumnDescriptor)
	cols := strings.Split(s, ",")
	hasValueCol := false
	hasTimeCol := false
	maxPos := 0
	for i, col := range cols {
		var cd ColumnDescriptor
		a := strings.SplitN(col, ":", 3)
		if len(a) != 3 {
			return nil, fmt.Errorf("entry #%d must have the following form: <column_pos>:<column_type>:<extension>; got %q", i+1, a)
		}
		pos, err := strconv.Atoi(a[0])
		if err != nil {
			return nil, fmt.Errorf("cannot parse <column_pos> part from the entry #%d %q: %w", i+1, col, err)
		}
		if pos <= 0 {
			return nil, fmt.Errorf("<column_pos> cannot be smaller than 1; got %d for entry #%d %q", pos, i+1, col)
		}
		if pos > maxColumnsPerRow {
			return nil, fmt.Errorf("<column_pos> cannot be bigger than %d; got %d for entry #%d %q", maxColumnsPerRow, pos, i+1, col)
		}
		if pos > maxPos {
			maxPos = pos
		}
		typ := a[1]
		switch typ {
		case "time":
			if hasTimeCol {
				return nil, fmt.Errorf("duplicate time column has been found at entry #%d %q for %q", i+1, col, s)
			}
			parseTimestamp, err := parseTimeFormat(a[2])
			if err != nil {
				return nil, fmt.Errorf("cannot parse time format from the entry #%d %q: %w", i+1, col, err)
			}
			cd.ParseTimestamp = parseTimestamp
			hasTimeCol = true
		case "label":
			cd.TagName = a[2]
			if len(cd.TagName) == 0 {
				return nil, fmt.Errorf("label name cannot be empty in the entry #%d %q", i+1, col)
			}
		case "metric":
			cd.MetricName = a[2]
			if len(cd.MetricName) == 0 {
				return nil, fmt.Errorf("metric name cannot be empty in the entry #%d %q", i+1, col)
			}
			hasValueCol = true
		default:
			return nil, fmt.Errorf("unknown <column_type>: %q; allowed values: time, metric, label", typ)
		}
		pos--
		if _, ok := m[pos]; ok {
			return nil, fmt.Errorf("duplicate <column_pos> %d for the entry #%d %q", pos, i+1, col)
		}
		m[pos] = cd
	}
	if !hasValueCol {
		return nil, fmt.Errorf("missing 'metric' column in %q", s)
	}
	cds := make([]ColumnDescriptor, maxPos)
	for pos, cd := range m {
		cds[pos] = cd
	}
	return cds, nil
}

func parseTimeFormat(format string) (func(s string) (int64, error), error) {
	if strings.HasPrefix(format, "custom:") {
		format = format[len("custom:"):]
		return newParseCustomTimeFunc(format), nil
	}
	switch format {
	case "unix_s":
		return parseUnixTimestampSeconds, nil
	case "unix_ms":
		return parseUnixTimestampMilliseconds, nil
	case "unix_ns":
		return parseUnixTimestampNanoseconds, nil
	case "rfc3339":
		return parseRFC3339, nil
	default:
		return nil, fmt.Errorf("unknown format for time parsing: %q; supported formats: unix_s, unix_ms, unix_ns, rfc3339", format)
	}
}

func parseUnixTimestampSeconds(s string) (int64, error) {
	n := fastfloat.ParseInt64BestEffort(s)
	if n > int64(1<<63-1)/1e3 {
		return 0, fmt.Errorf("too big unix timestamp in seconds: %d; must be smaller than %d", n, int64(1<<63-1)/1e3)
	}
	return n * 1e3, nil
}

func parseUnixTimestampMilliseconds(s string) (int64, error) {
	n := fastfloat.ParseInt64BestEffort(s)
	return n, nil
}

func parseUnixTimestampNanoseconds(s string) (int64, error) {
	n := fastfloat.ParseInt64BestEffort(s)
	return n / 1e6, nil
}

func parseRFC3339(s string) (int64, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("cannot parse time in RFC3339 from %q: %w", s, err)
	}
	return t.UnixNano() / 1e6, nil
}

func newParseCustomTimeFunc(format string) func(s string) (int64, error) {
	return func(s string) (int64, error) {
		t, err := time.Parse(format, s)
		if err != nil {
			return 0, fmt.Errorf("cannot parse time in custom format %q from %q: %w", format, s, err)
		}
		return t.UnixNano() / 1e6, nil
	}
}
