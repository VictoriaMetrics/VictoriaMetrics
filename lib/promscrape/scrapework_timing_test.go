package promscrape

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/chunkedbuffer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
)

func BenchmarkIsAutoMetricMiss(b *testing.B) {
	metrics := []string{
		"process_cpu_seconds_total",
		"process_resident_memory_bytes",
		"vm_tcplistener_read_calls_total",
		"http_requests_total",
		"node_cpu_seconds_total",
	}
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, metric := range metrics {
				if isAutoMetric(metric) {
					panic(fmt.Errorf("BUG: %q mustn't be detected as auto metric", metric))
				}
			}
		}
	})
}

func BenchmarkIsAutoMetricHit(b *testing.B) {
	metrics := []string{
		"up",
		"scrape_duration_seconds",
		"scrape_series_current",
		"scrape_samples_scraped",
		"scrape_series_added",
	}
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, metric := range metrics {
				if !isAutoMetric(metric) {
					panic(fmt.Errorf("BUG: %q must be detected as auto metric", metric))
				}
			}
		}
	})
}

func BenchmarkScrapeWorkScrapeInternalOneShot(b *testing.B) {
	data := `
vm_tcplistener_accepts_total{name="http", addr=":80"} 1443
vm_tcplistener_accepts_total{name="https", addr=":443"} 12801
vm_tcplistener_conns{name="http", addr=":80"} 0
vm_tcplistener_conns{name="https", addr=":443"} 2
vm_tcplistener_errors_total{name="http", addr=":80", type="accept"} 0
vm_tcplistener_errors_total{name="http", addr=":80", type="close"} 0
vm_tcplistener_errors_total{name="http", addr=":80", type="read"} 97
vm_tcplistener_errors_total{name="http", addr=":80", type="write"} 2
vm_tcplistener_errors_total{name="https", addr=":443", type="accept"} 0
vm_tcplistener_errors_total{name="https", addr=":443", type="close"} 0
vm_tcplistener_errors_total{name="https", addr=":443", type="read"} 243
vm_tcplistener_errors_total{name="https", addr=":443", type="write"} 285
vm_tcplistener_read_bytes_total{name="http", addr=":80"} 879339
vm_tcplistener_read_bytes_total{name="https", addr=":443"} 19453340
vm_tcplistener_read_calls_total{name="http", addr=":80"} 7780
vm_tcplistener_read_calls_total{name="https", addr=":443"} 70323
vm_tcplistener_read_timeouts_total{name="http", addr=":80"} 673
vm_tcplistener_read_timeouts_total{name="https", addr=":443"} 12353
vm_tcplistener_write_calls_total{name="http", addr=":80"} 3996
vm_tcplistener_write_calls_total{name="https", addr=":443"} 132356
`
	pushData := func(_ *auth.Token, _ *prompbmarshal.WriteRequest) {}
	benchmarkScrapeWorkScrapeInternal(b, []byte(data), false, pushData)
}

func BenchmarkScrapeWorkScrapeInternalStream(b *testing.B) {
	data := `
vm_tcplistener_accepts_total{name="http", addr=":80"} 1443
vm_tcplistener_accepts_total{name="https", addr=":443"} 12801
vm_tcplistener_conns{name="http", addr=":80"} 0
vm_tcplistener_conns{name="https", addr=":443"} 2
vm_tcplistener_errors_total{name="http", addr=":80", type="accept"} 0
vm_tcplistener_errors_total{name="http", addr=":80", type="close"} 0
vm_tcplistener_errors_total{name="http", addr=":80", type="read"} 97
vm_tcplistener_errors_total{name="http", addr=":80", type="write"} 2
vm_tcplistener_errors_total{name="https", addr=":443", type="accept"} 0
vm_tcplistener_errors_total{name="https", addr=":443", type="close"} 0
vm_tcplistener_errors_total{name="https", addr=":443", type="read"} 243
vm_tcplistener_errors_total{name="https", addr=":443", type="write"} 285
vm_tcplistener_read_bytes_total{name="http", addr=":80"} 879339
vm_tcplistener_read_bytes_total{name="https", addr=":443"} 19453340
vm_tcplistener_read_calls_total{name="http", addr=":80"} 7780
vm_tcplistener_read_calls_total{name="https", addr=":443"} 70323
vm_tcplistener_read_timeouts_total{name="http", addr=":80"} 673
vm_tcplistener_read_timeouts_total{name="https", addr=":443"} 12353
vm_tcplistener_write_calls_total{name="http", addr=":80"} 3996
vm_tcplistener_write_calls_total{name="https", addr=":443"} 132356
`
	pushData := func(_ *auth.Token, _ *prompbmarshal.WriteRequest) {}
	benchmarkScrapeWorkScrapeInternal(b, []byte(data), true, pushData)
}

func BenchmarkScrapeWorkScrapeInternalStreamBigData(b *testing.B) {
	generateScrape := func(n int) []byte {
		var b []byte
		for i := 0; i < n; i++ {
			b = append(b, "fooooo_"...)
			b = strconv.AppendInt(b, int64(i), 10)
			b = append(b, " 1\n"...)
		}
		return b
	}

	data := generateScrape(200_000)
	pushData := func(_ *auth.Token, _ *prompbmarshal.WriteRequest) {
		// simulates a delay to highlight the difference between lock-based and lock-free algorithms.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8515
		time.Sleep(time.Millisecond)
	}
	benchmarkScrapeWorkScrapeInternal(b, data, true, pushData)
}

func benchmarkScrapeWorkScrapeInternal(b *testing.B, data []byte, streamParse bool, pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest)) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	readData := func(dst *chunkedbuffer.Buffer) (bool, error) {
		dst.MustWrite(data)
		return false, nil
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	b.RunParallel(func(pb *testing.PB) {
		var sw scrapeWork
		sw.Config = &ScrapeWork{
			StreamParse: streamParse,
		}
		sw.ReadData = readData
		sw.PushData = pushData
		tsmGlobal.Register(&sw)
		timestamp := int64(0)
		for pb.Next() {
			if err := sw.scrapeInternal(timestamp, timestamp); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			timestamp++
		}
		tsmGlobal.Unregister(&sw)
	})
}

func BenchmarkScrapeWorkGetLabelsHash(b *testing.B) {
	labels := make([]prompbmarshal.Label, 100)
	for i := range labels {
		labels[i] = prompbmarshal.Label{
			Name:  fmt.Sprintf("name%d", i),
			Value: fmt.Sprintf("value%d", i),
		}
	}

	b.ReportAllocs()
	b.SetBytes(1)

	b.RunParallel(func(pb *testing.PB) {
		var hSum uint64
		for pb.Next() {
			h := getLabelsHash(labels)
			hSum += h
		}
		Sink.Add(hSum)
	})
}

var Sink atomic.Uint64
