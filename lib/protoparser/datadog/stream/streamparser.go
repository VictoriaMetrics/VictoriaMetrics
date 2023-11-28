package stream

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog"
	apiSeriesV1 "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/api/series/v1"
	apiSeriesV2 "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/api/series/v2"
	apiSketchesBeta "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/api/sketches/beta"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	// The maximum request size is defined at https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
	maxInsertRequestSize = flagutil.NewBytes("datadog.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single DataDog POST request to /api/v1/series, /api/v2/series, /api/beta/sketches")

	// If all metrics in Datadog have the same naming schema as custom metrics, then the following rules apply:
	// https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics
	// But there's some hidden behaviour. In addition to what it states in the docs, the following is also done:
	// - Consecutive underscores are replaced with just one underscore
	// - Underscore immediately before or after a dot are removed
	sanitizeMetricName = flag.Bool("datadog.sanitizeMetricName", true, "Sanitize metric names for the ingested DataDog data to comply with DataDog behaviour described at "+
		"https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics")
)

// Parse parses DataDog POST request for /api/v1/series, /api/v2/series, /api/beta/sketches from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func Parse(req *http.Request, callback func(prompbmarshal.TimeSeries) error) error {
	var r io.Reader
	wcr := writeconcurrencylimiter.GetReader(req.Body)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr
	contentEncoding := req.Header.Get("Content-Encoding")

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
	apiVersion := insertApisVersionRegex.ReplaceAllString(req.URL.Path, "${version}")
	apiKind := insertApisVersionRegex.ReplaceAllString(req.URL.Path, "${kind}")

	var ddReq datadog.Request
	switch apiKind {
	case "series":
		switch apiVersion {
		case "v1":
			ddReq = getSeriesV1Request()
			defer putSeriesV1Request(ddReq)
		case "v2":
			ddReq = getSeriesV2Request()
			defer putSeriesV2Request(ddReq)
		default:
			return fmt.Errorf(
				"API version %q of DataDog series endpoint is not supported",
				apiVersion,
			)
		}
	case "sketches":
		switch apiVersion {
		case "beta":
			ddReq = getSketchesBetaRequest()
			defer putSketchesBetaRequest(ddReq)
		default:
			return fmt.Errorf(
				"API version %q of DataDog sketches endpoint is not supported",
				apiVersion,
			)
		}
	default:
		return fmt.Errorf(
			"API kind %q of DataDog API is not supported",
			apiKind,
		)
	}

	if err := ddReq.Unmarshal(ctx.reqBuf.B); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog POST request with size %d bytes: %w", len(ctx.reqBuf.B), err)
	}

	cb := func(series prompbmarshal.TimeSeries) error {
		rowsRead.Add(len(series.Samples))
		return callback(series)
	}

	if err := ddReq.Extract(cb, sanitizeName(*sanitizeMetricName)); err != nil {
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
		return fmt.Errorf("cannot read request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big request; mustn't exceed -datadog.maxInsertRequestSize=%d bytes", maxInsertRequestSize.N)
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

func getSeriesV1Request() *apiSeriesV1.Request {
	v := seriesV1RequestPool.Get()
	if v == nil {
		return &apiSeriesV1.Request{}
	}
	return v.(*apiSeriesV1.Request)
}

func putSeriesV1Request(req datadog.Request) {
	seriesV1RequestPool.Put(req)
}

var seriesV1RequestPool sync.Pool

func getSeriesV2Request() *apiSeriesV2.Request {
	v := seriesV2RequestPool.Get()
	if v == nil {
		return &apiSeriesV2.Request{}
	}
	return v.(*apiSeriesV2.Request)
}

func putSeriesV2Request(req datadog.Request) {
	seriesV2RequestPool.Put(req)
}

var seriesV2RequestPool sync.Pool

func getSketchesBetaRequest() *apiSketchesBeta.Request {
	v := sketchesBetaRequestPool.Get()
	if v == nil {
		return &apiSketchesBeta.Request{}
	}
	return v.(*apiSketchesBeta.Request)
}

func putSketchesBetaRequest(req datadog.Request) {
	sketchesBetaRequestPool.Put(req)
}

var sketchesBetaRequestPool sync.Pool

// sanitizeName performs DataDog-compatible sanitizing for metric names
//
// See https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics
func sanitizeName(sanitize bool) func(string) string {
	if sanitize {
		return func(name string) string {
			return namesSanitizer.Transform(name)
		}
	}
	return func(name string) string {
		return name
	}
}

var namesSanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	s = unsupportedDatadogChars.ReplaceAllString(s, "_")
	s = multiUnderscores.ReplaceAllString(s, "_")
	s = underscoresWithDots.ReplaceAllString(s, ".")
	return s
})

var (
	unsupportedDatadogChars = regexp.MustCompile(`[^0-9a-zA-Z_\.]+`)
	multiUnderscores        = regexp.MustCompile(`_+`)
	underscoresWithDots     = regexp.MustCompile(`_?\._?`)
	insertApisVersionRegex  = regexp.MustCompile(`.*/api/(?P<version>[\w]+)/(?P<kind>[\w]+)`)
)
