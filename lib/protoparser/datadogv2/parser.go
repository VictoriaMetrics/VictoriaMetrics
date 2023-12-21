package datadogv2

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// Request represents DataDog POST request to /api/v2/series
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

// UnmarshalJSON unmarshals JSON DataDog /api/v2/series request body from b to req.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
//
// b shouldn't be modified when req is in use.
func UnmarshalJSON(req *Request, b []byte) error {
	req.reset()
	if err := json.Unmarshal(b, req); err != nil {
		return fmt.Errorf("cannot unmarshal %q: %w", b, err)
	}
	// Set missing timestamps to the current time.
	currentTimestamp := int64(fasttime.UnixTimestamp())
	series := req.Series
	for i := range series {
		points := series[i].Points
		for j := range points {
			pt := &points[j]
			if pt.Timestamp <= 0 {
				pt.Timestamp = currentTimestamp
			}
		}
	}
	return nil
}

// UnmarshalProtobuf unmarshals protobuf DataDog /api/v2/series request body from b to req.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
//
// b shouldn't be modified when req is in use.
func UnmarshalProtobuf(req *Request, b []byte) error {
	req.reset()
	_ = b
	return fmt.Errorf("unimplemented")
}

// Series represents a series item from DataDog POST request to /api/v2/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Series struct {
	// Do not decode Interval, since it isn't used by VictoriaMetrics
	// Interval int64 `json:"interval"`

	// Do not decode Metadata, since it isn't used by VictoriaMetrics
	// Metadata Metadata `json:"metadata"`

	// Metric is the name of the metric
	Metric string `json:"metric"`

	// Points points for the given metric
	Points []Point `json:"points"`

	Resources      []Resource `json:"resources"`
	SourceTypeName string     `json:"source_type_name"`

	Tags []string

	// Do not decode Type, since it isn't used by VictoriaMetrics
	// Type int `json:"type"`

	// Do not decode Unit, since it isn't used by VictoriaMetrics
	// Unit string
}

func (s *Series) reset() {
	s.Metric = ""

	points := s.Points
	for i := range points {
		points[i].reset()
	}
	s.Points = points[:0]

	resources := s.Resources
	for i := range resources {
		resources[i].reset()
	}
	s.Resources = resources[:0]

	s.SourceTypeName = ""

	tags := s.Tags
	for i := range tags {
		tags[i] = ""
	}
	s.Tags = tags[:0]
}

// Point represents a point from DataDog POST request to /api/v2/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Point struct {
	// Timestamp is point timestamp in seconds
	Timestamp int64 `json:"timestamp"`

	// Value is point value
	Value float64 `json:"value"`
}

func (pt *Point) reset() {
	pt.Timestamp = 0
	pt.Value = 0
}

// Resource is series resource from DataDog POST request to /api/v2/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Resource struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (r *Resource) reset() {
	r.Name = ""
	r.Type = ""
}
