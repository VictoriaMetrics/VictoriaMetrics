package promscrape

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

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
	readDataFunc := func(dst []byte) ([]byte, error) {
		return append(dst, data...), nil
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		var sw scrapeWork
		sw.ReadData = readDataFunc
		sw.PushData = func(wr *prompbmarshal.WriteRequest) {}
		timestamp := int64(0)
		for pb.Next() {
			if err := sw.scrapeInternal(timestamp); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			timestamp++
		}
	})
}
