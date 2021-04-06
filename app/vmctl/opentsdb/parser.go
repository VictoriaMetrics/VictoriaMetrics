package opentsdb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	allowedNames     = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_:]*$")
	allowedFirstChar = regexp.MustCompile("^[a-zA-Z]")
	replaceChars     = regexp.MustCompile("[^a-zA-Z0-9_:]")
	allowedTagKeys   = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_]*$")
)

func convertDuration(duration string) (time.Duration, error) {
	/*
		Golang's time library doesn't support many different
		string formats (year, month, week, day) because they
		aren't consistent ranges. But Java's library _does_.
		Consequently, we'll need to handle all the custom
		time ranges, and, to make the internal API call consistent,
		we'll need to allow for durations that Go supports, too.

		The nice thing is all the "broken" time ranges are > 1 hour,
		so we can just make assumptions to convert them to a range in hours.
		They aren't *good* assumptions, but they're reasonable
		for this function.
	*/
	var actualDuration time.Duration
	var err error
	var timeValue int
	if strings.HasSuffix(duration, "y") {
		timeValue, err = strconv.Atoi(strings.Trim(duration, "y"))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
		timeValue = timeValue * 365 * 24
		actualDuration, err = time.ParseDuration(fmt.Sprintf("%vh", timeValue))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else if strings.HasSuffix(duration, "w") {
		timeValue, err = strconv.Atoi(strings.Trim(duration, "w"))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
		timeValue = timeValue * 7 * 24
		actualDuration, err = time.ParseDuration(fmt.Sprintf("%vh", timeValue))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else if strings.HasSuffix(duration, "d") {
		timeValue, err = strconv.Atoi(strings.Trim(duration, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
		timeValue = timeValue * 24
		actualDuration, err = time.ParseDuration(fmt.Sprintf("%vh", timeValue))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else if strings.HasSuffix(duration, "h") || strings.HasSuffix(duration, "m") || strings.HasSuffix(duration, "s") || strings.HasSuffix(duration, "ms") {
		actualDuration, err = time.ParseDuration(duration)
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else {
		return 0, fmt.Errorf("invalid time duration string: %q", duration)
	}
	return actualDuration, nil
}

// Convert an incoming retention "string" into the component parts
func convertRetention(retention string, offset int64, msecTime bool) (Retention, error) {
	/*
		A retention string coming in looks like
		sum-1m-avg:1h:30d
		So we:
		1. split on the :
		2. split on the - in slice 0
		3. create the time ranges we actually need
	*/
	chunks := strings.Split(retention, ":")
	if len(chunks) != 3 {
		return Retention{}, fmt.Errorf("invalid retention string: %q", retention)
	}
	rowLengthDuration, err := convertDuration(chunks[1])
	if err != nil {
		return Retention{}, fmt.Errorf("invalid row length (first order) duration string: %q: %s", chunks[1], err)
	}
	// set length of each row in milliseconds, unless we aren't using millisecond time in OpenTSDB...then use seconds
	rowLength := rowLengthDuration.Milliseconds()
	if !msecTime {
		rowLength = rowLength / 1000
	}
	ttlDuration, err := convertDuration(chunks[2])
	if err != nil {
		return Retention{}, fmt.Errorf("invalid ttl (second order) duration string: %q: %s", chunks[2], err)
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
	if len(aggregates) != 3 {
		return Retention{}, fmt.Errorf("invalid aggregation string: %q", chunks[0])
	}

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
	/*
		replace bad characters in metric name with _ per the data model
		only replace if needed to reduce string processing time
	*/
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
		/*
			replace all explicitly bad characters with _
			only replace if needed to reduce string processing time
		*/
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
