package opentsdb

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	allowedNames     = regexp.MustCompile("^[a-zA-Z_:][a-zA-Z0-9_:]*$")
	allowedFirstChar = regexp.MustCompile("[a-zA-Z]")
	allowedFields    = regexp.MustCompile("^[a-zA-Z0-9_:]*$")
	replaceChars     = regexp.MustCompile("[^a-zA-Z0-9_:]")
	allowedTagKeys   = regexp.MustCompile("[a-zA-Z][a-zA-Z0-9_]*")
)

// Convert an incoming retention "string" into the component parts
func ConvertRetention(retention string) (string, string, string, []timeChunks) {
	/*
	A retention string coming in looks like
	sum-1m-avg:1h:30d
	So we:
	1. split on the :
	2. split on the - in slice 0
	3. create the time ranges we actually need
	*/
	chunks := strings.Split(retention, ":")
	aggregates := strings.Split(chunks[0], "-")
	rowLength, err := time.ParseDuration(chunks[1])
	if err != nil {
		panic
	}
	ttl, err := time.ParseDuration(chunks[2])
	if err != nil {
		panic
	}
	rowSecs := rowLength.Seconds()
	ttlSecs := ttl.Seconds()
	var timeChunks []TimeRange
	for i := 0; i < ttlSecs; i+rowSecs {
		append(timeChunks, TimeRange{Start: i+rowSecs, End: i})
	}
	// FirstOrder, AggTime, SecondOrder, RowSize, TTL
	return aggregates[0], aggregates[1], aggregates[2], timeChunks
}

// This ensures any incoming data from OpenTSDB matches the Prometheus data model
// https://prometheus.io/docs/concepts/data_model
func ModifyData(msg Metric, normalize bool) (Metric, err) {
	finalMsg := Metric{
		Metric: "", Tags: make(map[string]string),
		AggregateTags: make([]string), Dps: &msg.Dps,
	}
	if !allowedFirstChar.MatchString(msg.Metric) {
		return nil, fmt.Errorf("%s has a bad first character", msg.Metric)
	}
	name := msg.Metric
	if normalize {
		name = strings.ToLower(name)
	}
	// replace bad characters in metric name with _ per the data model
	if !allowedNames.MatchString(name) {
		finalMsg.Metric = replaceChars.ReplaceAllString(name, "_")
	} else {
		finalMsg.Metric = name
	}
	// replace bad characters in tag keys with _ per the data model
	for key, value := range msg.Tags {
		if normalize {
			key = strings.ToLower(key)
			value = strings.ToLower(value)
		}
		if !allowedTagKeys.MatchString(key) {
			key = replaceChars.ReplaceAllString(key, "_")
		}
		// tags that start with __ are considered custom stats for internal prometheus stuff, we should drop them
		if !strings.HasPrefix(key, "__") {
			finalMsg.Tags[key] = value
		}
	}
	return finalMsg, nil
}

/*
func Unmarshal(msg string) (
type queryValues struct {
	name   string
	values map[string][]interface{}
}

func parseResult(r influx.Result) ([]queryValues, error) {
	if len(r.Err) > 0 {
		return nil, fmt.Errorf("result error: %s", r.Err)
	}
	qValues := make([]queryValues, len(r.Series))
	for i, row := range r.Series {
		values := make(map[string][]interface{}, len(row.Values))
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

func toFloat64(v interface{}) (float64, error) {
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
}*/
