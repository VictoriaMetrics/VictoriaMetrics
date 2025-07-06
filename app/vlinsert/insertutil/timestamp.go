package insertutil

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// ExtractTimestampFromFields extracts timestamp in nanoseconds from the first field the name from timeFields at fields.
//
// The value for the corresponding timeFields is set to empty string after returning from the function,
// so it could be ignored during data ingestion.
//
// The current timestamp is returned if fields do not contain a field with timeField name or if the timeField value is empty.
func ExtractTimestampFromFields(timeFields []string, fields []logstorage.Field) (int64, error) {
	for _, timeField := range timeFields {
		for i := range fields {
			f := &fields[i]
			if f.Name != timeField {
				continue
			}
			nsecs, err := parseTimestamp(f.Value)
			if err != nil {
				return 0, fmt.Errorf("cannot parse timestamp from field %q: %s", f.Name, err)
			}
			f.Value = ""
			if nsecs == 0 {
				nsecs = time.Now().UnixNano()
			}
			return nsecs, nil
		}
	}
	return time.Now().UnixNano(), nil
}

func parseTimestamp(s string) (int64, error) {
	// "-" is a nil timestamp value, if the syslog
	// application is incapable of obtaining system time
	// https://datatracker.ietf.org/doc/html/rfc5424#section-6.2.3
	if s == "" || s == "0" || s == "-" {
		return time.Now().UnixNano(), nil
	}
	if len(s) <= len("YYYY") || s[len("YYYY")] != '-' {
		nsecs, ok := timeutil.TryParseUnixTimestamp(s)
		if !ok {
			return 0, fmt.Errorf("cannot parse unix timestamp %q", s)
		}
		return nsecs, nil
	}
	nsecs, ok := logstorage.TryParseTimestampRFC3339Nano(s)
	if !ok {
		return 0, fmt.Errorf("cannot unmarshal rfc3339 timestamp %q", s)
	}
	return nsecs, nil
}
