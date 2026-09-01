package writeconcurrencylimiter

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxConcurrentInserts = flagutil.NewIntWithDynamicDefault("maxConcurrentInserts", 2*cgroup.AvailableCPUs(), "2*cgroup.AvailableCPUs()",
		"The maximum number of concurrent insert requests. "+
			"Set higher value when clients send data over slow networks. "+
			"Default value depends on the number of available CPU cores. It should work fine in most cases since it minimizes resource usage. "+
			"See also -insert.maxQueueDuration")
	maxQueueDuration = flag.Duration("insert.maxQueueDuration", time.Minute, "The maximum duration to wait in the queue when -maxConcurrentInserts "+
		"concurrent insert requests are executed")
	maxConcurrentStreamReaders = flag.Int("insert.maxConcurrentStreamReaders", 0,
		"The maximum number of concurrent insert requests with allocated per-connection read buffers for parsing streaming protocols, "+
			"such as InfluxDB line protocol, Graphite, OpenTSDB and CSV import. "+
			"Every such buffer occupies about 64KiB of memory, so limiting their count prevents high memory usage during ingestion bursts. "+
			"The default value is calculated automatically depending on the available memory as -memory.allowedBytes divided by 1MiB, but not lower than 64. "+
			"Set this flag to a higher value when the number of concurrent requests over streaming protocols exceeds the default limit. "+
			"See also -insert.maxQueueDuration and -maxConcurrentInserts")
)

// Reader is a reader, which decreases the concurrency before every Read() call
// and increases the concurrency after Read() call.
//
// It effectively limits the number of concurrent goroutines,
// which may process results returned by concurrently processed Reader structs.
//
// The Reader must be obtained via GetReader() call.
type Reader struct {
	r                    io.Reader
	increasedConcurrency bool
}

// GetReader returns the Reader for r.
//
// The PutReader() must be called when the returned Reader is no longer needed.
//
// GetReader additionally acquires a slot from the stream readers limit, which bounds
// the number of concurrently allocated per-connection read buffers used by streaming
// protocol parsers (InfluxDB line protocol, Graphite, OpenTSDB, CSV import, ...).
// Every such buffer occupies about 64KiB of memory, so without this limit a burst of
// concurrent connections could consume hundreds of MiB before -maxConcurrentInserts
// admission kicks in. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11463 .
func GetReader(r io.Reader) (*Reader, error) {
	if err := incStreamReadersConcurrency(); err != nil {
		return nil, err
	}
	if err := IncConcurrency(); err != nil {
		decStreamReadersConcurrency()
		return nil, err
	}

	v := readerPool.Get()
	if v == nil {
		v = &Reader{}
	}
	rr := v.(*Reader)
	rr.r = r
	rr.increasedConcurrency = true

	return rr, nil
}

// PutReader returns the r to the pool.
//
// It decreases the concurrency and releases the slot obtained by GetReader from
// the stream readers limit.
func PutReader(r *Reader) {
	r.r = nil
	if r.increasedConcurrency {
		DecConcurrency()
		r.increasedConcurrency = false
	}
	decStreamReadersConcurrency()
	readerPool.Put(r)
}

var readerPool sync.Pool

func initStreamReadersCh() {
	n := *maxConcurrentStreamReaders
	if n <= 0 {
		// By default, allow one buffered reader per 1MiB of the memory allowed for caches.
		// See -memory.allowedPercent and -memory.allowedBytes.
		n = memory.Allowed() / (1024 * 1024)
		if n < 64 {
			// Keep the limit reasonably high for setups with a small amount of memory.
			n = 64
		}
	}
	streamReadersCh = make(chan struct{}, n)
}

var (
	streamReadersCh     chan struct{}
	streamReadersChOnce sync.Once
)

// incStreamReadersConcurrency obtains a slot from the -insert.maxConcurrentStreamReaders limit.
//
// The obtained slot must be returned back via decStreamReadersConcurrency() call.
func incStreamReadersConcurrency() error {
	streamReadersChOnce.Do(initStreamReadersCh)

	select {
	case streamReadersCh <- struct{}{}:
		return nil
	default:
	}

	streamReadersLimitReached.Inc()
	t := timerpool.Get(*maxQueueDuration)
	defer timerpool.Put(t)
	select {
	case streamReadersCh <- struct{}{}:
		return nil
	case <-t.C:
		return &httpserver.ErrorWithStatusCode{
			Err: fmt.Errorf("cannot process insert request for %.3f seconds because %d concurrent insert requests already have allocated read buffers. "+
				"Possible solutions: to reduce the number of concurrent insert requests over streaming protocols; "+
				"to increase -insert.maxConcurrentStreamReaders; to increase -insert.maxQueueDuration",
				maxQueueDuration.Seconds(), cap(streamReadersCh)),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
}

// decStreamReadersConcurrency returns the slot obtained via incStreamReadersConcurrency(), so other connections could obtain it.
func decStreamReadersConcurrency() {
	<-streamReadersCh
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	DecConcurrency()
	r.increasedConcurrency = false

	n, err := r.r.Read(p)

	if errC := IncConcurrency(); errC != nil {
		return n, errC
	}
	r.increasedConcurrency = true

	if errors.Is(err, io.ErrUnexpectedEOF) {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8704
		err = fmt.Errorf("%w: while reading the request body. This might be caused by a timeout on the client side. "+
			"Possible solutions: to lower -insert.maxQueueDuration below the client’s timeout; to increase the client-side timeout; "+
			"to increase compute resources at the server; to increase -maxConcurrentInserts", err)
	}

	return n, err
}

func initConcurrencyLimitCh() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrentInserts)
}

var (
	concurrencyLimitCh     chan struct{}
	concurrencyLimitChOnce sync.Once
)

// IncConcurrency obtains a concurrency token from -maxConcurrentInserts.
//
// The obtained token must be returned back via DecConcurrency() call.
func IncConcurrency() error {
	concurrencyLimitChOnce.Do(initConcurrencyLimitCh)

	select {
	case concurrencyLimitCh <- struct{}{}:
		return nil
	default:
	}

	concurrencyLimitReached.Inc()
	t := timerpool.Get(*maxQueueDuration)
	defer timerpool.Put(t)
	select {
	case concurrencyLimitCh <- struct{}{}:
		return nil
	case <-t.C:
		concurrencyLimitTimeout.Inc()
		return &httpserver.ErrorWithStatusCode{
			Err: fmt.Errorf("cannot process insert request for %.3f seconds because %d concurrent insert requests are executed. "+
				"Possible solutions: to reduce workload; to increase compute resources at the server; "+
				"to increase -insert.maxQueueDuration; to increase -maxConcurrentInserts",
				maxQueueDuration.Seconds(), *maxConcurrentInserts),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
}

// DecConcurrency returns the token obtained via IncConcurrency(), so other goroutines could obtain it.
func DecConcurrency() {
	<-concurrencyLimitCh
}

var (
	concurrencyLimitReached = metrics.NewCounter(`vm_concurrent_insert_limit_reached_total`)
	concurrencyLimitTimeout = metrics.NewCounter(`vm_concurrent_insert_limit_timeout_total`)

	_ = metrics.NewGauge(`vm_concurrent_insert_capacity`, func() float64 {
		concurrencyLimitChOnce.Do(initConcurrencyLimitCh)
		return float64(cap(concurrencyLimitCh))
	})
	_ = metrics.NewGauge(`vm_concurrent_insert_current`, func() float64 {
		concurrencyLimitChOnce.Do(initConcurrencyLimitCh)
		return float64(len(concurrencyLimitCh))
	})

	streamReadersLimitReached = metrics.NewCounter(`vm_concurrent_stream_readers_limit_reached_total`)

	_ = metrics.NewGauge(`vm_concurrent_stream_readers_capacity`, func() float64 {
		streamReadersChOnce.Do(initStreamReadersCh)
		return float64(cap(streamReadersCh))
	})
	_ = metrics.NewGauge(`vm_concurrent_stream_readers_current`, func() float64 {
		streamReadersChOnce.Do(initStreamReadersCh)
		return float64(len(streamReadersCh))
	})
)
