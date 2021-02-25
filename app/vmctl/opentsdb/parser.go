package opentsdb

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var (
	allowedNames     = regexp.MustCompile("^[a-zA-Z_:][a-zA-Z0-9_:]*$")
	allowedFirstChar = regexp.MustCompile("[a-zA-Z]")
	replaceChars     = regexp.MustCompile("[^a-zA-Z0-9_:]")
	allowedTagKeys   = regexp.MustCompile("[a-zA-Z][a-zA-Z0-9_]*")
)

// Convert an incoming retention "string" into the component parts
func convertRetention(retention string, offset int) (string, string, string, []TimeRange) {
	/*
		Our "offset" is the number of days we should step
		back before starting to scan for data
	*/
	offset = offset * 24 * 60 * 60
	/*
		A retention string coming in looks like
		sum-1m-avg:1h:30d
		So we:
		1. split on the :
		2. split on the - in slice 0
		3. create the time ranges we actually need
	*/
	chunks := strings.Split(retention, ":")
	log.Println("Retention strings to process: ", chunks)
	aggregates := strings.Split(chunks[0], "-")
	rowLength, err := time.ParseDuration(chunks[1])
	if err != nil {
		panic(fmt.Sprintf("Failed to parse duration, %v", err))
	}
	ttl, err := time.ParseDuration(chunks[2])
	if err != nil {
		panic(fmt.Sprintf("Failed to parse duration: %v", err))
	}
	rowSecs := rowLength.Seconds()
	ttlSecs := ttl.Seconds()
	var timeChunks []TimeRange
	var i int64
	for i = int64(offset); i <= int64(ttlSecs); i = i + int64(rowSecs) {
		timeChunks = append(timeChunks, TimeRange{Start: i + int64(rowSecs), End: i})
	}
	// FirstOrder, AggTime, SecondOrder, RowSize, TTL
	return aggregates[0], aggregates[1], aggregates[2], timeChunks
}

// This ensures any incoming data from OpenTSDB matches the Prometheus data model
// https://prometheus.io/docs/concepts/data_model
func modifyData(msg Metric, normalize bool) (Metric, error) {
	finalMsg := Metric{
		Metric: "", Tags: make(map[string]string),
		Timestamps: msg.Timestamps, Values: msg.Values,
	}
	if !allowedFirstChar.MatchString(msg.Metric) {
		return Metric{}, fmt.Errorf("%s has a bad first character", msg.Metric)
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
