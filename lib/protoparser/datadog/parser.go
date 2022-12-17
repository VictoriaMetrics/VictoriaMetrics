package datadog

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// SplitTag splits DataDog tag into tag name and value.
//
// See https://docs.datadoghq.com/getting_started/tagging/#define-tags
func SplitTag(tag string) (string, string) {
	n := strings.IndexByte(tag, ':')
	if n < 0 {
		// No tag value.
		return tag, "no_label_value"
	}
	return tag[:n], tag[n+1:]
}

// Request represents DataDog POST request to /api/v1/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Request struct {
	Series []Series `json:"series"`
}

func (req *Request) reset() {
	// recursively reset all the fields in req in order to avoid field value
	// re-use in json.Unmarshal() when the corresponding field is missing
	// in the unmarshaled JSON.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3432
	series := req.Series
	for i := range series {
		series[i].reset()
	}
	req.Series = series[:0]
}

// Unmarshal unmarshals DataDog /api/v1/series request body from b to req.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
//
// b shouldn't be modified when req is in use.
func (req *Request) Unmarshal(b []byte) error {
	req.reset()
	if err := json.Unmarshal(b, req); err != nil {
		return fmt.Errorf("cannot unmarshal %q: %w", b, err)
	}
	// Set missing timestamps to the current time.
	currentTimestamp := float64(fasttime.UnixTimestamp())
	series := req.Series
	for i := range series {
		points := series[i].Points
		for j := range points {
			if points[j][0] <= 0 {
				points[j][0] = currentTimestamp
			}
		}
	}
	return nil
}

// Series represents a series item from DataDog POST request to /api/v1/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Series struct {
	Metric string `json:"metric"`
	Host   string `json:"host"`

	// The device field does not appear in the datadog docs, but datadog-agent does use it.
	// Datadog agent (v7 at least), removes the tag "device" and adds it as its own field. Why? That I don't know!
	// https://github.com/DataDog/datadog-agent/blob/0ada7a97fed6727838a6f4d9c87123d2aafde735/pkg/metrics/series.go#L84-L105
	Device string `json:"device"`

	// Do not decode Interval, since it isn't used by VictoriaMetrics
	// Interval int64 `json:"interval"`

	Points []Point  `json:"points"`
	Tags   []string `json:"tags"`

	// Do not decode Type, since it isn't used by VictoriaMetrics
	// Type string `json:"type"`
}

func (s *Series) reset() {
	s.Metric = ""
	s.Host = ""
	s.Device = ""

	points := s.Points
	for i := range points {
		points[i] = Point{}
	}
	s.Points = points[:0]

	tags := s.Tags
	for i := range tags {
		tags[i] = ""
	}
	s.Tags = tags[:0]
}

// Point represents a point from DataDog POST request to /api/v1/series
type Point [2]float64

// Timestamp returns timestamp in milliseconds from the given pt.
func (pt *Point) Timestamp() int64 {
	return int64(pt[0] * 1000)
}

// Value returns value from the given pt.
func (pt *Point) Value() float64 {
	return pt[1]
}
