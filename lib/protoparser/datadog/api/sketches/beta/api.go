package datadog

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/api/sketches/beta/pb"
)

// Request represents a sketches item from DataDog POST request to /api/beta/sketches
type Request struct {
	*pb.SketchPayload
}

// Unmarshal is a wrapper around SketchesPayload Unmarshal method which decodes byte array to SketchPayload struct
func (r *Request) Unmarshal(b []byte) error {
	if r.SketchPayload == nil {
		r.SketchPayload = new(pb.SketchPayload)
	}
	return r.SketchPayload.UnmarshalVT(b)
}

// Extract iterates fn function execution over all timeseries from a sketch payload
func (r *Request) Extract(fn func(prompbmarshal.TimeSeries) error, sanitizeFn func(string) string) error {
	var err error
	for _, sketch := range r.SketchPayload.Sketches {
		sketchSeries := make([]prompbmarshal.TimeSeries, 5)
		for _, point := range sketch.Dogsketches {
			timestamp := point.Ts * 1000
			updateSeries(sketchSeries, sanitizeFn(sketch.Metric), timestamp, map[string]float64{
				"max": point.Max,
				"min": point.Min,
				"cnt": float64(point.Cnt),
				"avg": point.Avg,
				"sum": point.Sum,
			})
		}
		for _, point := range sketch.Distributions {
			timestamp := point.Ts * 1000
			updateSeries(sketchSeries, sanitizeFn(sketch.Metric), timestamp, map[string]float64{
				"max": point.Max,
				"min": point.Min,
				"cnt": float64(point.Cnt),
				"avg": point.Avg,
				"sum": point.Sum,
			})
		}
		labels := getLabels(sketch, sanitizeFn)
		for i := range sketchSeries {
			sketchSeries[i].Labels = append(sketchSeries[i].Labels, labels...)
			if err = fn(sketchSeries[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func getLabels(sketch *pb.SketchPayload_Sketch, sanitizeFn func(string) string) []prompbmarshal.Label {
	labels := []prompbmarshal.Label{}
	if sketch.Host != "" {
		labels = append(labels, prompbmarshal.Label{
			Name:  "host",
			Value: sketch.Host,
		})
	}
	for _, tag := range sketch.Tags {
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

func updateSeries(series []prompbmarshal.TimeSeries, metric string, timestamp int64, values map[string]float64) {
	index := 0
	for suffix, value := range values {
		s := series[index]
		s.Samples = append(s.Samples, prompbmarshal.Sample{
			Timestamp: timestamp,
			Value:     value,
		})
		if len(s.Labels) == 0 {
			s.Labels = append(s.Labels, prompbmarshal.Label{
				Name:  "",
				Value: metric + "_" + suffix,
			})
		}
		index++
	}
}
