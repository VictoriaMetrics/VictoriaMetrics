package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"

	"github.com/VictoriaMetrics/metrics"
)

func TestRemoteWriteObfuscateLabels(t *testing.T) {
	f := func(obfuscateLabelList string, inputTss []prompb.TimeSeries, expectedTss []prompb.TimeSeries) {
		t.Helper()
		rwctx := &remoteWriteCtx{
			idx: 0,
		}
		olctx := &obfuscateLabelsCtx{
			cacheResults: make(map[string]string),
		}
		defer metrics.UnregisterAllMetrics()
		originValue := *obfuscateLabels
		defer func() {
			*obfuscateLabels = originValue
		}()
		*obfuscateLabels = []string{obfuscateLabelList}
		rwctx.initObfuscateLabels()

		outputTss := olctx.obfuscate(inputTss, rwctx.obfuscateLabels)

		if !reflect.DeepEqual(expectedTss, outputTss) {
			t.Fatalf("unexpected samples;\ngot\n%v\nwant\n%v", outputTss, expectedTss)
		}
	}

	sha256Result := func(str string) string {
		sha256Result := sha256.Sum256([]byte(str))
		return hex.EncodeToString(sha256Result[:])
	}

	// 1. obfuscation is not set.
	f("",
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
	)

	// 1. obfuscation is set for another rwctx.
	f(",ip",
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
	)

	// 2. obfuscate the value of "ip" label
	f("ip",
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: sha256Result("123")},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
	)

	// 3. obfuscate the values of "ip" and "instance"
	f("ip^^instance",
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
					{Name: "instance", Value: "1234"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "job", Value: "123"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: sha256Result("123")},
					{Name: "instance", Value: sha256Result("1234")},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "job", Value: "123"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
	)

	// 4. duplicate label names in config must produce single SHA-256, not double
	f("ip^^ip",
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: "123"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "ip", Value: sha256Result("123")},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 0},
				},
			},
		},
	)
}
