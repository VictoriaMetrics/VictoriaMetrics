package remotewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"

	"github.com/VictoriaMetrics/metrics"
)

func BenchmarkRemoteWriteObfuscateLabels(b *testing.B) {
	originValue := *obfuscateLabels
	defer func() {
		*obfuscateLabels = originValue
	}()
	*obfuscateLabels = []string{"ip^^instance"}
	sha256Result := func(str string) string {
		sha256Result := sha256.Sum256([]byte(str))
		return hex.EncodeToString(sha256Result[:])
	}
	expected := []prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "ip", Value: sha256Result("123")},
				{Name: "instance", Value: sha256Result("12345")},
				{Name: "__name__", Value: "http_requests_total"},
			},
			Samples: []prompb.Sample{
				{Value: 1, Timestamp: 0},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "ip", Value: sha256Result("1236")},
				{Name: "instance", Value: sha256Result("some-long-instante-string")},
				{Name: "__name__", Value: "concurrent_requests"},
			},
			Samples: []prompb.Sample{
				{Value: 1, Timestamp: 0},
			},
		},
	}
	defer metrics.UnregisterAllMetrics()

	inputTss := []prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "ip", Value: "123"},
				{Name: "instance", Value: "12345"},
				{Name: "__name__", Value: "http_requests_total"},
			},
			Samples: []prompb.Sample{
				{Value: 1, Timestamp: 0},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "ip", Value: "1236"},
				{Name: "instance", Value: "some-long-instante-string"},
				{Name: "__name__", Value: "concurrent_requests"},
			},
			Samples: []prompb.Sample{
				{Value: 1, Timestamp: 0},
			},
		},
	}
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		rwctx := &remoteWriteCtx{
			idx: 0,
		}
		olctx := &obfuscateLabelsCtx{
			cacheResults: make(map[string]string),
		}
		rwctx.initObfuscateLabels()
		var localTss []prompb.TimeSeries
		for pb.Next() {
			// always make a shallow copy because obfuscate changes input
			localTss = localTss[:0]
			localTss = append(localTss, inputTss...)
			olctx.reset()
			outputTss := olctx.obfuscate(localTss, rwctx.obfuscateLabels)
			if !reflect.DeepEqual(expected, outputTss) {
				b.Fatalf("unexpected output: got: \n%v\n want: \n%v\n", outputTss, expected)
			}
		}
	})

}
