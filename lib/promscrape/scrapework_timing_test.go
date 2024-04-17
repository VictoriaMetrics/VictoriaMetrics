package promscrape

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
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

func BenchmarkScrapeWorkScrapeInternal(b *testing.B) {
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
	readDataFunc := func(dst *bytesutil.ByteBuffer) error {
		dst.B = append(dst.B, data...)
		return nil
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		var sw scrapeWork
		sw.Config = &ScrapeWork{}
		sw.ReadData = readDataFunc
		sw.PushData = func(_ *auth.Token, _ *prompbmarshal.WriteRequest) {}
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
