package datadog

import (
	"encoding/json"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog"
)

// Request represents /api/v1/series request
type Request struct {
	Series []series `json:"series"`
}

// Unmarshal decodes byte array to series v1 Request struct
func (r *Request) Unmarshal(b []byte) error {
	return json.Unmarshal(b, r)
}

// Extract iterates fn execution over all timeseries from series v1 Request
func (r *Request) Extract(fn func(prompbmarshal.TimeSeries) error, sanitizeFn func(string) string) error {
	currentTimestamp := int64(fasttime.UnixTimestamp())
	for i := range r.Series {
		s := r.Series[i]
		samples := make([]prompbmarshal.Sample, 0, len(s.Points))
		for j := range s.Points {
			p := s.Points[j]
			ts, val := p[0], p[1]
			if ts <= 0 {
				ts = float64(currentTimestamp)
			}
			samples = append(samples, prompbmarshal.Sample{
				Timestamp: int64(ts * 1000),
				Value:     val,
			})
		}
		ts := prompbmarshal.TimeSeries{
			Samples: samples,
			Labels:  s.getLabels(sanitizeFn),
		}
		if err := fn(ts); err != nil {
			return err
		}
	}
	return nil
}

// series represents a series item from DataDog POST request to /api/v1/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type series struct {
	Metric string `json:"metric"`
	Host   string `json:"host"`

	// The device field does not appear in the datadog docs, but datadog-agent does use it.
	// Datadog agent (v7 at least), removes the tag "device" and adds it as its own field. Why? That I don't know!
	// https://github.com/DataDog/datadog-agent/blob/0ada7a97fed6727838a6f4d9c87123d2aafde735/pkg/metrics/series.go#L84-L105
	Device string `json:"device"`

	// Do not decode Interval, since it isn't used by VictoriaMetrics
	// Interval int64 `json:"interval"`

	Points []point  `json:"points"`
	Tags   []string `json:"tags"`

	// Do not decode type, since it isn't used by VictoriaMetrics
	// Type string `json:"type"`
}

func (s *series) getLabels(sanitizeFn func(string) string) []prompbmarshal.Label {
	labels := []prompbmarshal.Label{{
		Name:  "__name__",
		Value: sanitizeFn(s.Metric),
	}}
	if s.Host != "" {
		labels = append(labels, prompbmarshal.Label{
			Name:  "host",
			Value: s.Host,
		})
	}
	if s.Device != "" {
		labels = append(labels, prompbmarshal.Label{
			Name:  "device",
			Value: s.Device,
		})
	}
	for _, tag := range s.Tags {
		name, value := datadog.SplitTag(tag)
		if name == "host" {
			name = "exported_host"
		}
		labels = append(labels, prompbmarshal.Label{
			Name:  sanitizeFn(name),
			Value: value,
		})
	}
	return labels
}

// point represents a point from DataDog POST request to /api/v1/series
type point [2]float64
