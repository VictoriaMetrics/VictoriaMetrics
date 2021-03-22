package opentsdb

import (
	"fmt"
	//"log"
	"regexp"
	"strings"
)

var (
	allowedNames     = regexp.MustCompile("^[a-zA-Z_:][a-zA-Z0-9_:]*$")
	allowedFirstChar = regexp.MustCompile("[a-zA-Z]")
	replaceChars     = regexp.MustCompile("[^a-zA-Z0-9_:]")
	allowedTagKeys   = regexp.MustCompile("[a-zA-Z][a-zA-Z0-9_]*")
	oneHour          = float64(3600)
	oneDay           = float64(3600 * 24)
)

// Convert an incoming retention "string" into the component parts
func convertRetention(retention string, offset int64, msecTime bool) (Retention, error) {
	/*
		Our "offset" is the number of days we should step
		back before starting to scan for data
	*/
	offset = offset * 24 * 60 * 60
	if msecTime {
		offset = offset * 1000
	}
	/*
		A retention string coming in looks like
		sum-1m-avg:1h:30d
		So we:
		1. split on the :
		2. split on the - in slice 0
		3. create the time ranges we actually need
	*/
	chunks := strings.Split(retention, ":")
	if len(chunks) < 3 {
		return Retention{}, fmt.Errorf("invalid retention string: %q", retention)
	}
	// default to one hour
	rowLengthDuration, err := ParseDuration(chunks[1])
	if err != nil {
		return Retention{}, fmt.Errorf("Invalid row length (first order) duration string: %q", chunks[1])
	}
	// set length of each row in milliseconds, unless we aren't using millisecond time in OpenTSDB...then use seconds
	rowLength := rowLengthDuration.Milliseconds()
	if !msecTime {
		rowLength = rowLength / 1000
	}
	// default to one day
	ttlDuration, err := ParseDuration(chunks[2])
	if err != nil {
		return Retention{}, fmt.Errorf("Invalid ttl (second order) duration string: %q", chunks[1])
	}
	// set ttl in milliseconds, unless we aren't using millisecond time in OpenTSDB...then use seconds
	ttl := ttlDuration.Milliseconds()
	if !msecTime {
		ttl = ttl / 1000
	}
	// bump by the offset so we don't look at empty ranges any time offset > ttl
	ttl += offset
	var timeChunks []TimeRange
	var i int64
	for i = offset; i <= ttl; i = i + rowLength {
		timeChunks = append(timeChunks, TimeRange{Start: i + rowLength, End: i})
	}
	// first/second order aggregations for queries defined in chunk 0...
	aggregates := strings.Split(chunks[0], "-")

	ret := Retention{FirstOrder: aggregates[0],
		SecondOrder: aggregates[2],
		AggTime:     aggregates[1],
		QueryRanges: timeChunks}
	return ret, nil
}

// This ensures any incoming data from OpenTSDB matches the Prometheus data model
// https://prometheus.io/docs/concepts/data_model
func modifyData(msg Metric, normalize bool) (Metric, error) {
	finalMsg := Metric{
		Metric: "", Tags: make(map[string]string),
		Timestamps: msg.Timestamps, Values: msg.Values,
	}
	// if the metric name has invalid characters, the data model says to drop it
	if !allowedFirstChar.MatchString(msg.Metric) {
		return Metric{}, fmt.Errorf("%s has a bad first character", msg.Metric)
	}
	name := msg.Metric
	// if normalization requested, lowercase the name
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
		// if normalization requested, lowercase the key and value
		if normalize {
			key = strings.ToLower(key)
			value = strings.ToLower(value)
		}
		// replace all explicitly bad characters with _
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
