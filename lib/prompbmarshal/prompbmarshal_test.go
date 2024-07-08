package prompbmarshal_test

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestWriteRequestMarshalProtobuf(t *testing.T) {
	wrm := &prompbmarshal.WriteRequest{
		Timeseries: []prompbmarshal.TimeSeries{
			{
				Labels: []prompbmarshal.Label{
					{
						Name:  "__name__",
						Value: "process_cpu_seconds_total",
					},
					{
						Name:  "instance",
						Value: "host-123:4567",
					},
					{
						Name:  "job",
						Value: "node-exporter",
					},
				},
				Samples: []prompbmarshal.Sample{
					{
						Value:     123.3434,
						Timestamp: 8939432423,
					},
					{
						Value:     -123.3434,
						Timestamp: 18939432423,
					},
				},
			},
		},
	}
	data := wrm.MarshalProtobuf(nil)

	// Verify that the marshaled protobuf is unmarshaled properly
	var wr prompb.WriteRequest
	if err := wr.UnmarshalProtobuf(data); err != nil {
		t.Fatalf("cannot unmarshal protobuf: %s", err)
	}

	// Compare the unmarshaled wr with the original wrm.
	wrm.Reset()
	for _, ts := range wr.Timeseries {
		var labels []prompbmarshal.Label
		for _, label := range ts.Labels {
			labels = append(labels, prompbmarshal.Label{
				Name:  label.Name,
				Value: label.Value,
			})
		}
		var samples []prompbmarshal.Sample
		for _, sample := range ts.Samples {
			samples = append(samples, prompbmarshal.Sample{
				Value:     sample.Value,
				Timestamp: sample.Timestamp,
			})
		}
		wrm.Timeseries = append(wrm.Timeseries, prompbmarshal.TimeSeries{
			Labels:  labels,
			Samples: samples,
		})
	}
	dataResult := wrm.MarshalProtobuf(nil)

	if !bytes.Equal(dataResult, data) {
		t.Fatalf("unexpected data obtained after marshaling\ngot\n%X\nwant\n%X", dataResult, data)
	}
}
