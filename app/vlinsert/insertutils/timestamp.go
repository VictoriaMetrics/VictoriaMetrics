package insertutils

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// ExtractTimestampRFC3339NanoFromFields extracts RFC3339 timestamp in nanoseconds from the field with the name timeField at fields.
//
// The value for the timeField is set to empty string after returning from the function,
// so it could be ignored during data ingestion.
//
// The current timestamp is returned if fields do not contain a field with timeField name or if the timeField value is empty.
func ExtractTimestampRFC3339NanoFromFields(timeField string, fields []logstorage.Field) (int64, error) {
	for i := range fields {
		f := &fields[i]
		if f.Name != timeField {
			continue
		}
		if f.Value == "" || f.Value == "0" {
			return time.Now().UnixNano(), nil
		}
		nsecs, ok := logstorage.TryParseTimestampRFC3339Nano(f.Value)
		if !ok {
			return 0, fmt.Errorf("cannot unmarshal rfc3339 timestamp from %s=%q", timeField, f.Value)
		}
		f.Value = ""
		return nsecs, nil
	}
	return time.Now().UnixNano(), nil
}
