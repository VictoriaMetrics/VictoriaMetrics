package influx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	influx "github.com/influxdata/influxdb/client/v2"
)

type queryValues struct {
	name   string
	values map[string][]any
}

func parseResult(r influx.Result) ([]queryValues, error) {
	if len(r.Err) > 0 {
		return nil, fmt.Errorf("result error: %s", r.Err)
	}
	qValues := make([]queryValues, len(r.Series))
	for i, row := range r.Series {
		values := make(map[string][]any, len(row.Values))
		for _, value := range row.Values {
			for idx, v := range value {
				key := row.Columns[idx]
				values[key] = append(values[key], v)
			}
		}
		qValues[i] = queryValues{
			name:   row.Name,
			values: values,
		}
	}
	return qValues, nil
}

// parseResultCheckTags check rows return by InfluxDB and remove rows not belong to current series.
// See: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7301.
func parseResultCheckTags(s *Series, r influx.Result) ([]queryValues, error) {
	if len(r.Err) > 0 {
		return nil, fmt.Errorf("result error: %s", r.Err)
	}

	// This map contains all the desired column of the query result for current series.
	// If a column (`Columns[i]` in influx.Result) not exists in map, this column must be declared by other series.
	// A row (`values[i]` in influx.Result) is considered invalid if its unwanted column value is not null.
	wantedColumns := make(map[string]bool)
	for i := range s.LabelPairs {
		wantedColumns[s.LabelPairs[i].Name] = true
	}
	wantedColumns[s.Field] = true
	wantedColumns["time"] = true // const from InfluxDB

	for i := range r.Series {
		if len(s.LabelPairs)+2 > len(r.Series[i].Columns) {
			// column in series fewer than query where condition. should never reach
			return nil, fmt.Errorf(`wrong number of columns in result series, expected: %v, "%s" and "time", got %v`, s.LabelPairs, s.Field, r.Series[i].Columns)
		}
		// prepare a new values slice to replace the existing one.
		values := make([][]interface{}, 0, len(r.Series[i].Values))

		// prepare an unwanted column list
		unwantedColumnIdx := make([]int, 0)
		for idx := range r.Series[i].Columns {
			if !wantedColumns[r.Series[i].Columns[idx]] {
				unwantedColumnIdx = append(unwantedColumnIdx, idx)
			}
		}

		// go through each rows
		for j := range r.Series[i].Values {
			if len(r.Series[i].Columns) != len(r.Series[i].Values[j]) {
				// column in a row does not match series columns. should never reach
				return nil, fmt.Errorf(`wrong number of columns in result row, expected: %d, got: %d`, len(r.Series[i].Columns), len(r.Series[i].Values[j]))
			}

			// skip if value of unwanted column is not null.
			skip := false
			for _, idx := range unwantedColumnIdx {
				if r.Series[i].Values[j][idx] != nil {
					skip = true
					break
				}
			}

			if !skip {
				values = append(values, r.Series[i].Values[j])
			}
		}
		r.Series[i].Values = values
	}

	// if more than 1 row exists after filtering, parse it into []queryValues.
	// otherwise just return nil result.
	for i := range r.Series {
		if len(r.Series[i].Values) > 0 {
			return parseResult(r)
		}
	}
	return nil, nil
}

func toFloat64(v any) (float64, error) {
	switch i := v.(type) {
	case json.Number:
		return i.Float64()
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		return strconv.ParseFloat(i, 64)
	case bool:
		if i {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("unexpected value type %v", i)
	}
}

func parseDate(dateStr string) (int64, error) {
	startTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q: %s", dateStr, err)
	}
	return startTime.UnixNano() / 1e6, nil
}

func stringify(q influx.Query) string {
	return fmt.Sprintf("command: %q; database: %q; retention: %q",
		q.Command, q.Database, q.RetentionPolicy)
}

func (s *Series) unmarshal(v string) error {
	noEscapeChars := strings.IndexByte(v, '\\') < 0
	n := nextUnescapedChar(v, ',', noEscapeChars)
	if n < 0 {
		s.Measurement = unescapeTagValue(v, noEscapeChars)
		return nil
	}
	s.Measurement = unescapeTagValue(v[:n], noEscapeChars)
	var err error
	s.LabelPairs, err = unmarshalTags(v[n+1:], noEscapeChars)
	if err != nil {
		return fmt.Errorf("failed to unmarhsal tags: %s", err)
	}
	return nil
}

func unmarshalTags(s string, noEscapeChars bool) ([]LabelPair, error) {
	var result []LabelPair
	for {
		lp := LabelPair{}
		n := nextUnescapedChar(s, ',', noEscapeChars)
		if n < 0 {
			if err := lp.unmarshal(s, noEscapeChars); err != nil {
				return nil, err
			}
			if len(lp.Name) == 0 || len(lp.Value) == 0 {
				return nil, nil
			}
			result = append(result, lp)
			return result, nil
		}
		if err := lp.unmarshal(s[:n], noEscapeChars); err != nil {
			return nil, err
		}
		s = s[n+1:]
		if len(lp.Name) == 0 || len(lp.Value) == 0 {
			continue
		}
		result = append(result, lp)
	}
}

func (lp *LabelPair) unmarshal(s string, noEscapeChars bool) error {
	n := nextUnescapedChar(s, '=', noEscapeChars)
	if n < 0 {
		return fmt.Errorf("missing tag value for %q", s)
	}
	lp.Name = unescapeTagValue(s[:n], noEscapeChars)
	lp.Value = unescapeTagValue(s[n+1:], noEscapeChars)
	return nil
}

func unescapeTagValue(s string, noEscapeChars bool) string {
	if noEscapeChars {
		// Fast path - no escape chars.
		return s
	}
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		return s
	}

	// Slow path. Remove escape chars.
	dst := make([]byte, 0, len(s))
	for {
		dst = append(dst, s[:n]...)
		s = s[n+1:]
		if len(s) == 0 {
			return string(append(dst, '\\'))
		}
		ch := s[0]
		if ch != ' ' && ch != ',' && ch != '=' && ch != '\\' {
			dst = append(dst, '\\')
		}
		dst = append(dst, ch)
		s = s[1:]
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			return string(append(dst, s...))
		}
	}
}

func nextUnescapedChar(s string, ch byte, noEscapeChars bool) int {
	if noEscapeChars {
		// Fast path: just search for ch in s, since s has no escape chars.
		return strings.IndexByte(s, ch)
	}

	sOrig := s
again:
	n := strings.IndexByte(s, ch)
	if n < 0 {
		return -1
	}
	if n == 0 {
		return len(sOrig) - len(s) + n
	}
	if s[n-1] != '\\' {
		return len(sOrig) - len(s) + n
	}
	nOrig := n
	slashes := 0
	for n > 0 && s[n-1] == '\\' {
		slashes++
		n--
	}
	if slashes&1 == 0 {
		return len(sOrig) - len(s) + nOrig
	}
	s = s[nOrig+1:]
	goto again
}
