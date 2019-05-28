package influx

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var rowsInserted = metrics.NewCounter(`vm_rows_inserted_total{type="influx"}`)

// InsertHandler processes remote write for influx line protocol.
//
// See https://github.com/influxdata/influxdb/blob/4cbdc197b8117fee648d62e2e5be75c6575352f0/tsdb/README.md
func InsertHandler(at *auth.Token, req *http.Request) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(at, req)
	})
}

func insertHandlerInternal(at *auth.Token, req *http.Request) error {
	influxReadCalls.Inc()

	r := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := getGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped influx line protocol data: %s", err)
		}
		defer putGzipReader(zr)
		r = zr
	}

	q := req.URL.Query()
	tsMultiplier := int64(1e6)
	switch q.Get("precision") {
	case "ns":
		tsMultiplier = 1e6
	case "u":
		tsMultiplier = 1e3
	case "ms":
		tsMultiplier = 1
	case "s":
		tsMultiplier = -1e3
	case "m":
		tsMultiplier = -1e3 * 60
	case "h":
		tsMultiplier = -1e3 * 3600
	}

	// Read db tag from https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint
	db := q.Get("db")

	ctx := getPushCtx()
	defer putPushCtx(ctx)
	for ctx.Read(r, tsMultiplier) {
		if err := ctx.InsertRows(at, db); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *pushCtx) InsertRows(at *auth.Token, db string) error {
	rows := ctx.Rows.Rows
	ic := &ctx.Common
	ic.Reset()
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		ic.AddLabel("db", db)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabel(tag.Key, tag.Value)
		}
		ic.MetricNameBuf = storage.MarshalMetricNameRaw(ic.MetricNameBuf[:0], at.AccountID, at.ProjectID, ic.Labels)
		metricNameBufLen := len(ic.MetricNameBuf)
		ctx.metricGroupBuf = append(ctx.metricGroupBuf[:0], r.Measurement...)
		ctx.metricGroupBuf = append(ctx.metricGroupBuf, '.')
		metricGroupPrefixLen := len(ctx.metricGroupBuf)
		ic.Labels = ic.Labels[:0]
		ic.AddLabel("", "placeholder")
		placeholderLabel := &ic.Labels[len(ic.Labels)-1]
		for j := range r.Fields {
			f := &r.Fields[j]
			ctx.metricGroupBuf = append(ctx.metricGroupBuf[:metricGroupPrefixLen], f.Key...)
			metricGroup := bytesutil.ToUnsafeString(ctx.metricGroupBuf)
			ic.Labels = ic.Labels[:len(ic.Labels)-1]
			ic.AddLabel("", metricGroup)
			ic.MetricNameBuf = storage.MarshalMetricLabelRaw(ic.MetricNameBuf[:metricNameBufLen], placeholderLabel)
			storageNodeIdx := ic.GetStorageNodeIdx(at, ic.Labels)
			if err := ic.WriteDataPointExt(at, storageNodeIdx, ic.MetricNameBuf, r.Timestamp, f.Value); err != nil {
				return err
			}
		}
		rowsInserted.Add(len(r.Fields))
	}
	return ic.FlushBufs()
}

func getGzipReader(r io.Reader) (*gzip.Reader, error) {
	v := gzipReaderPool.Get()
	if v == nil {
		return gzip.NewReader(r)
	}
	zr := v.(*gzip.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func putGzipReader(zr *gzip.Reader) {
	_ = zr.Close()
	gzipReaderPool.Put(zr)
}

var gzipReaderPool sync.Pool

func (ctx *pushCtx) Read(r io.Reader, tsMultiplier int64) bool {
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(r, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			influxReadErrors.Inc()
			ctx.err = fmt.Errorf("cannot read influx line protocol data: %s", ctx.err)
		}
		return false
	}
	if err := ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf)); err != nil {
		influxUnmarshalErrors.Inc()
		ctx.err = fmt.Errorf("cannot unmarshal influx line protocol data with size %d: %s", len(ctx.reqBuf), err)
		return false
	}

	// Adjust timestamps according to tsMultiplier
	currentTs := time.Now().UnixNano() / 1e6
	if tsMultiplier >= 1 {
		for i := range ctx.Rows.Rows {
			row := &ctx.Rows.Rows[i]
			if row.Timestamp == 0 {
				row.Timestamp = currentTs
			} else {
				row.Timestamp /= tsMultiplier
			}
		}
	} else if tsMultiplier < 0 {
		tsMultiplier = -tsMultiplier
		for i := range ctx.Rows.Rows {
			row := &ctx.Rows.Rows[i]
			if row.Timestamp == 0 {
				row.Timestamp = currentTs
			} else {
				row.Timestamp *= tsMultiplier
			}
		}
	}
	return true
}

var (
	influxReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="influx"}`)
	influxReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="influx"}`)
	influxUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="influx"}`)
)

type pushCtx struct {
	Rows   Rows
	Common netstorage.InsertCtx

	reqBuf         []byte
	tailBuf        []byte
	metricGroupBuf []byte

	err error
}

func (ctx *pushCtx) Error() error {
	if ctx.err == io.EOF {
		return nil
	}
	return ctx.err
}

func (ctx *pushCtx) reset() {
	ctx.Rows.Reset()
	ctx.Common.Reset()
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.metricGroupBuf = ctx.metricGroupBuf[:0]

	ctx.err = nil
}

func getPushCtx() *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			return v.(*pushCtx)
		}
		return &pushCtx{}
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
var pushCtxPoolCh = make(chan *pushCtx, runtime.GOMAXPROCS(-1))
