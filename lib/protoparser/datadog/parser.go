package datadog

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// Request represents DataDog POST request to /api/v1/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Request struct {
	Series []Series `json:"series"`
}

func (req *Request) reset() {
	req.Series = req.Series[:0]
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
	Host string `json:"host"`

	// Do not decode Interval, since it isn't used by VictoriaMetrics
	// Interval int64 `json:"interval"`

	Metric string   `json:"metric"`
	Points []Point  `json:"points"`
	Tags   []string `json:"tags"`

	// Do not decode Type, since it isn't used by VictoriaMetrics
	// Type string `json:"type"`
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
