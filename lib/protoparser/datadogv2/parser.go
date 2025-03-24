package datadogv2

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/easyproto"
)

// Request represents DataDog POST request to /api/v2/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Request struct {
	Series []Series `json:"series"`
}

func (req *Request) reset() {
	// recursively reset all the fields in req in order to avoid field value
	// reuse in json.Unmarshal() when the corresponding field is missing
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
	return req.unmarshalProtobuf(b)
}

func (req *Request) unmarshalProtobuf(src []byte) (err error) {
	// message Request {
	//   repeated Series series = 1;
	// }
	//
	// See https://github.com/DataDog/agent-payload/blob/d7c5dcc63970d0e19678a342e7718448dd777062/proto/metrics/agent_payload.proto
	series := req.Series
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read series data")
			}
			if len(series) < cap(series) {
				series = series[:len(series)+1]
			} else {
				series = append(series, Series{})
			}
			s := &series[len(series)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal series: %w", err)
			}
		}
	}
	req.Series = series
	return nil
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

func (s *Series) unmarshalProtobuf(src []byte) (err error) {
	// message MetricSeries {
	//   string metric = 2;
	//   repeated Point points = 4;
	//   repeated Resource resources = 1;
	//   string source_type_name = 7;
	//   repeated string tags = 3;
	// }
	//
	// See https://github.com/DataDog/agent-payload/blob/d7c5dcc63970d0e19678a342e7718448dd777062/proto/metrics/agent_payload.proto
	points := s.Points
	resources := s.Resources
	tags := s.Tags
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal next field: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			metric, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot unmarshal metric")
			}
			s.Metric = metric
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read point data")
			}
			if len(points) < cap(points) {
				points = points[:len(points)+1]
			} else {
				points = append(points, Point{})
			}
			pt := &points[len(points)-1]
			if err := pt.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal point: %s", err)
			}
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read resource data")
			}
			if len(resources) < cap(resources) {
				resources = resources[:len(resources)+1]
			} else {
				resources = append(resources, Resource{})
			}
			r := &resources[len(resources)-1]
			if err := r.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal resource: %w", err)
			}
		case 7:
			sourceTypeName, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot unmarshal source_type_name")
			}
			s.SourceTypeName = sourceTypeName
		case 3:
			tag, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot unmarshal tag")
			}
			tags = append(tags, tag)
		}
	}
	s.Points = points
	s.Resources = resources
	s.Tags = tags
	return nil
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

func (pt *Point) unmarshalProtobuf(src []byte) (err error) {
	// message Point {
	//   double value = 1;
	//   int64 timestamp = 2;
	// }
	//
	// See https://github.com/DataDog/agent-payload/blob/d7c5dcc63970d0e19678a342e7718448dd777062/proto/metrics/agent_payload.proto
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			value, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot unmarshal value")
			}
			pt.Value = value
		case 2:
			timestamp, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot unmarshal timestamp")
			}
			pt.Timestamp = timestamp
		}
	}
	return nil
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

func (r *Resource) unmarshalProtobuf(src []byte) (err error) {
	// message Resource {
	//   string type = 1;
	//   string name = 2;
	// }
	//
	// See https://github.com/DataDog/agent-payload/blob/d7c5dcc63970d0e19678a342e7718448dd777062/proto/metrics/agent_payload.proto
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			typ, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot unmarshal type")
			}
			r.Type = typ
		case 2:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot unmarshal name")
			}
			r.Name = name
		}
	}
	return nil
}
