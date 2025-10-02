package prompb_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestWriteRequestMarshalUnmarshal(t *testing.T) {
	// Verify that the marshaled protobuf is unmarshalled properly
	f := func(wrm *prompb.WriteRequest) {
		t.Helper()

		data := wrm.MarshalProtobuf(nil)

		wru := &prompb.WriteRequestUnmarshaler{}
		wr, err := wru.UnmarshalProtobuf(data)
		if err != nil {
			t.Fatalf("cannot unmarshal protobuf: %s", err)
		}

		if !reflect.DeepEqual(wrm, wr) {
			t.Fatalf("unmarshaled WriteRequest is not equal to the original\nGot:\n%+v\nWant:\n%+v", wr, wrm)
		}

		dataResult := wrm.MarshalProtobuf(nil)
		if !bytes.Equal(dataResult, data) {
			t.Fatalf("unexpected data obtained after marshaling\ngot\n%X\nwant\n%X", dataResult, data)
		}
	}

	f(&prompb.WriteRequest{})

	f(&prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
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
			},
		},
	})

	f(&prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Samples: []prompb.Sample{
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
	})

	f(&prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
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
				Samples: []prompb.Sample{
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
		Metadata: []prompb.MetricMetadata{
			{
				// COUNTER = 1
				Type:             1,
				MetricFamilyName: "process_cpu_seconds_total",
				Help:             "Total user and system CPU time spent in seconds",
				Unit:             "seconds",

				ProjectID: 123,
				AccountID: 456,
			},
		},
	})

	f(&prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
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
				Samples: []prompb.Sample{
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
			{
				Labels: []prompb.Label{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
				Samples: []prompb.Sample{
					{
						Value: 9873,
					},
				},
			},
		},
		Metadata: []prompb.MetricMetadata{
			{
				// COUNTER = 1
				Type:             1,
				MetricFamilyName: "process_cpu_seconds_total",
				Help:             "Total user and system CPU time spent in seconds",
				Unit:             "seconds",

				ProjectID: 123,
				AccountID: 456,
			},
		},
	})

	// only metadata
	f(&prompb.WriteRequest{
		Metadata: []prompb.MetricMetadata{
			{
				// COUNTER = 1
				Type:             1,
				MetricFamilyName: "process_cpu_seconds_total",
				Help:             "Total user and system CPU time spent in seconds",
				Unit:             "seconds",

				ProjectID: 123,
				AccountID: 456,
			},
		},
	})

	// only metadata several
	f(&prompb.WriteRequest{
		Metadata: []prompb.MetricMetadata{
			{
				// COUNTER = 1
				Type:             1,
				MetricFamilyName: "process_cpu_seconds_total",
				Help:             "Total user and system CPU time spent in seconds",
				Unit:             "seconds",

				ProjectID: 123,
				AccountID: 456,
			},
			{
				// GAUGE = 2
				Type:             2,
				MetricFamilyName: "process_memory_bytes",
				Help:             "Total user and system memory in bytes",
				Unit:             "bytes",

				ProjectID: 1,
				AccountID: 2,
			},
		},
	})
}
