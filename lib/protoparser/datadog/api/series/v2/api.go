package datadog

import (
	"encoding/json"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog"
)

// Request represents a sketches item from DataDog POST request to /api/v2/series
type Request struct {
	Series []series `json:"series"`
}

// Unmarshal decodes byte array to series v2 Request struct
func (r *Request) Unmarshal(b []byte) error {
	return json.Unmarshal(b, r)
}

// Extract iterates fn execution over all timeseries from series v2 request
func (r *Request) Extract(fn func(prompbmarshal.TimeSeries) error, sanitizeFn func(string) string) error {
	for i := range r.Series {
		s := r.Series[i]
		samples := make([]prompbmarshal.Sample, 0, len(s.Points))
		for j := range s.Points {
			p := s.Points[j]
			samples = append(samples, prompbmarshal.Sample{
				Timestamp: p.Timestamp * 1000,
				Value:     p.Value,
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

// series represents a series item from DataDog POST request to /api/v2/series
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type series struct {
	Metric    string     `json:"metric"`
	Resources []resource `json:"resources"`
	Points    []point    `json:"points"`
	Tags      []string   `json:"tags"`

	// Do not decode Type, since it isn't used by VictoriaMetrics
	// Type string `json:"type"`
}

type resource struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (s *series) getLabels(sanitizeFn func(string) string) []prompbmarshal.Label {
	labels := []prompbmarshal.Label{{
		Name:  "__name__",
		Value: sanitizeFn(s.Metric),
	}}
	for _, res := range s.Resources {
		labels = append(labels, prompbmarshal.Label{
			Name:  sanitizeFn(res.Type),
			Value: res.Name,
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

// point represents a point from DataDog POST request to /api/v2/series
type point struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}
