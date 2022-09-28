package datadog

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var (
	// The maximum request size is defined at https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
	maxInsertRequestSize = flagutil.NewBytes("datadog.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single DataDog POST request to /api/v1/series")

	// If all metrics in Datadog have the same naming schema as custom metrics, then the following rules apply:
	// https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics
	// But there's some hidden behaviour. In addition to what it states in the docs, the following is also done:
	// - Consecutive underscores are replaced with just one underscore
	// - Underscore immediately before or after a dot are removed
	sanitizeMetricName = flag.Bool("datadog.sanitizeMetricName", true, "Sanitize metric names for the ingested DataDog data to comply with DataDog behaviour described at "+
		"https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics")
)

// ParseStream parses DataDog POST request for /api/v1/series from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func ParseStream(r io.Reader, contentEncoding string, callback func(series []Series) error) error {
	switch contentEncoding {
	case "gzip":
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped DataDog data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	case "deflate":
		zlr, err := common.GetZlibReader(r)
		if err != nil {
			return fmt.Errorf("cannot read deflated DataDog data: %w", err)
		}
		defer common.PutZlibReader(zlr)
		r = zlr
	}
	ctx := getPushCtx(r)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}
	req := getRequest()
	defer putRequest(req)
	if err := req.Unmarshal(ctx.reqBuf.B); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog POST request with size %d bytes: %s", len(ctx.reqBuf.B), err)
	}
	rows := 0
	series := req.Series
	for i := range series {
		rows += len(series[i].Points)
		if *sanitizeMetricName {
			series[i].Metric = sanitizeName(series[i].Metric)
		}
	}
	rowsRead.Add(rows)

	if err := callback(series); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

type pushCtx struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
}

func (ctx *pushCtx) Read() error {
	readCalls.Inc()
	lr := io.LimitReader(ctx.br, int64(maxInsertRequestSize.N)+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read compressed request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", maxInsertRequestSize.N)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadog"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadog"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadog"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadog"}`)
)

func getPushCtx(r io.Reader) *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		ctx.br.Reset(r)
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			ctx := v.(*pushCtx)
			ctx.br.Reset(r)
			return ctx
		}
		return &pushCtx{
			br: bufio.NewReaderSize(r, 64*1024),
		}
	}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	select {
	case pushCtxPoolCh <- ctx:
	default:
		pushCtxPool.Put(ctx)
	}
}

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *pushCtx, cgroup.AvailableCPUs())

func getRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

func putRequest(req *Request) {
	requestPool.Put(req)
}

var requestPool sync.Pool

// sanitizeName performs DataDog-compatible santizing for metric names
//
// See https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics
func sanitizeName(name string) string {
	m := sanitizedNames.Load().(*sync.Map)
	v, ok := m.Load(name)
	if ok {
		// Fast path - the sanitized name is found in the cache.
		sp := v.(*string)
		return *sp
	}
	// Slow path - sanitize name and store it in the cache.
	sanitizedName := unsupportedDatadogChars.ReplaceAllString(name, "_")
	sanitizedName = multiUnderscores.ReplaceAllString(sanitizedName, "_")
	sanitizedName = underscoresWithDots.ReplaceAllString(sanitizedName, ".")
	// Make a copy of name in order to limit memory usage to the name length,
	// since the name may point to bigger string.
	s := string(append([]byte{}, name...))
	if sanitizedName == name {
		// point sanitizedName to just allocated s, since it may point to name,
		// which, in turn, can point to bigger string.
		sanitizedName = s
	}
	sp := &sanitizedName
	m.Store(s, sp)
	n := atomic.AddUint64(&sanitizedNamesLen, 1)
	if n > 100e3 {
		atomic.StoreUint64(&sanitizedNamesLen, 0)
		sanitizedNames.Store(&sync.Map{})
	}
	return sanitizedName
}

var (
	sanitizedNames    atomic.Value
	sanitizedNamesLen uint64

	unsupportedDatadogChars = regexp.MustCompile(`[^0-9a-zA-Z_\.]+`)
	multiUnderscores        = regexp.MustCompile(`_+`)
	underscoresWithDots     = regexp.MustCompile(`_?\._?`)
)

func init() {
	sanitizedNames.Store(&sync.Map{})
}
