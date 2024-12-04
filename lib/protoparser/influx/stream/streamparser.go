package stream

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxLineSize    = flagutil.NewBytes("influx.maxLineSize", 256*1024, "The maximum size in bytes for a single InfluxDB line during parsing. Applicable for stream mode only. See https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf")
	maxRequestSize = flagutil.NewBytes("influx.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single InfluxDB request. Applicable for batch mode only. See https://docs.victoriametrics.com/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf")
	trimTimestamp  = flag.Duration("influxTrimTimestamp", time.Millisecond, "Trim timestamps for InfluxDB line protocol data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// Parse parses r with the given args and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func Parse(r io.Reader, isStreamMode, isGzipped bool, precision, db string, callback func(db string, rows []influx.Row) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped influx line protocol data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	tsMultiplier := int64(0)
	switch precision {
	case "ns":
		tsMultiplier = 1e6
	case "u", "us", "µ":
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

	// processing payload altogether
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7090
	if !isStreamMode {
		ctx := getBatchContext(r)
		defer putBatchContext(ctx)
		err := ctx.Read()
		if err != nil {
			return err
		}
		err = unmarshal(&ctx.rows, ctx.reqBuf.B, tsMultiplier)
		if err != nil {
			return fmt.Errorf("cannot parse influx line protocol data: %s; To skip invalid lines switch to stream mode by passing Stream-Mode: \"1\" header with each request", err)
		}
		return callback(db, ctx.rows.Rows)
	}

	// processing in a streaming fashion, line-by-line
	// invalid lines are skipped
	ctx := getStreamContext(r)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.ctx = ctx
		uw.callback = callback
		uw.db = db
		uw.tsMultiplier = tsMultiplier
		uw.reqBuf, ctx.reqBuf = ctx.reqBuf, uw.reqBuf
		ctx.wg.Add(1)
		common.ScheduleUnmarshalWork(uw)
		wcr.DecConcurrency()
	}
	ctx.wg.Wait()
	if err := ctx.Error(); err != nil {
		return err
	}
	return ctx.callbackErr
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="influx"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="influx"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="influx"}`)
)

type batchContext struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
	rows   influx.Rows
}

func (ctx *batchContext) Read() error {
	readCalls.Inc()
	lr := io.LimitReader(ctx.br, int64(maxRequestSize.IntN()))
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return err
	} else if reqLen > int64(maxRequestSize.IntN()) {
		readErrors.Inc()
		return fmt.Errorf("too big request; mustn't exceed -influx.maxRequestSize=%d bytes", maxRequestSize.N)
	}
	return nil
}

func (ctx *batchContext) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
	ctx.rows.Reset()
}

func getBatchContext(r io.Reader) *batchContext {
	if v := batchContextPool.Get(); v != nil {
		ctx := v.(*batchContext)
		ctx.br.Reset(r)
		return ctx
	}
	return &batchContext{
		br: bufio.NewReaderSize(r, 64*1024),
	}
}

func putBatchContext(ctx *batchContext) {
	ctx.reset()
	batchContextPool.Put(ctx)
}

var batchContextPool sync.Pool

type streamContext struct {
	br      *bufio.Reader
	reqBuf  []byte
	tailBuf []byte
	err     error

	wg              sync.WaitGroup
	callbackErrLock sync.Mutex
	callbackErr     error
}

func (ctx *streamContext) Read() bool {
	readCalls.Inc()
	if ctx.err != nil || ctx.hasCallbackError() {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlockExt(ctx.br, ctx.reqBuf, ctx.tailBuf, maxLineSize.IntN())
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read influx line protocol data: %w", ctx.err)
		}
		return false
	}
	return true
}

func (ctx *streamContext) Error() error {
	if ctx.err == io.EOF {
		return nil
	}
	return ctx.err
}

func (ctx *streamContext) hasCallbackError() bool {
	ctx.callbackErrLock.Lock()
	ok := ctx.callbackErr != nil
	ctx.callbackErrLock.Unlock()
	return ok
}

func (ctx *streamContext) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.err = nil
	ctx.callbackErr = nil
}

func getStreamContext(r io.Reader) *streamContext {
	if v := streamContextPool.Get(); v != nil {
		ctx := v.(*streamContext)
		ctx.br.Reset(r)
		return ctx
	}
	return &streamContext{
		br: bufio.NewReaderSize(r, 64*1024),
	}
}

func putStreamContext(ctx *streamContext) {
	ctx.reset()
	streamContextPool.Put(ctx)
}

var streamContextPool sync.Pool

type unmarshalWork struct {
	rows         influx.Rows
	ctx          *streamContext
	callback     func(db string, rows []influx.Row) error
	db           string
	tsMultiplier int64
	reqBuf       []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.ctx = nil
	uw.callback = nil
	uw.db = ""
	uw.tsMultiplier = 0
	uw.reqBuf = uw.reqBuf[:0]
}

func (uw *unmarshalWork) runCallback() {
	ctx := uw.ctx
	if err := uw.callback(uw.db, uw.rows.Rows); err != nil {
		err = fmt.Errorf("error when processing imported data: %w", err)
		ctx.callbackErrLock.Lock()
		if ctx.callbackErr == nil {
			ctx.callbackErr = err
		}
		ctx.callbackErrLock.Unlock()
	}
	ctx.wg.Done()
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	_ = unmarshal(&uw.rows, uw.reqBuf, uw.tsMultiplier)
	uw.runCallback()
	putUnmarshalWork(uw)
}

func getUnmarshalWork() *unmarshalWork {
	v := unmarshalWorkPool.Get()
	if v == nil {
		v = &unmarshalWork{}
	}
	uw := v.(*unmarshalWork)
	uw.rows.IgnoreErrs = true
	return uw
}

func putUnmarshalWork(uw *unmarshalWork) {
	uw.reset()
	unmarshalWorkPool.Put(uw)
}

var unmarshalWorkPool sync.Pool

func detectTimestamp(ts, currentTs int64) int64 {
	if ts == 0 {
		return currentTs
	}
	if ts >= 1e17 {
		// convert nanoseconds to milliseconds
		return ts / 1e6
	}
	if ts >= 1e14 {
		// convert microseconds to milliseconds
		return ts / 1e3
	}
	if ts >= 1e11 {
		// the ts is in milliseconds
		return ts
	}
	// convert seconds to milliseconds
	return ts * 1e3
}

func unmarshal(rs *influx.Rows, reqBuf []byte, tsMultiplier int64) error {
	// do not return error immediately because rs.Rows could contain
	// successfully parsed rows that needs to be processed below
	err := rs.Unmarshal(bytesutil.ToUnsafeString(reqBuf))
	rows := rs.Rows
	rowsRead.Add(len(rows))

	// Adjust timestamps according to uw.tsMultiplier
	currentTs := time.Now().UnixNano() / 1e6
	if tsMultiplier == 0 {
		// Default precision is 'ns'. See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/#timestamp
		// But it can be in ns, us, ms or s depending on the number of digits in practice.
		for i := range rows {
			tsPtr := &rows[i].Timestamp
			*tsPtr = detectTimestamp(*tsPtr, currentTs)
		}
	} else if tsMultiplier >= 1 {
		for i := range rows {
			row := &rows[i]
			if row.Timestamp == 0 {
				row.Timestamp = currentTs
			} else {
				row.Timestamp /= tsMultiplier
			}
		}
	} else if tsMultiplier < 0 {
		tsMultiplier = -tsMultiplier
		currentTs -= currentTs % tsMultiplier
		for i := range rows {
			row := &rows[i]
			if row.Timestamp == 0 {
				row.Timestamp = currentTs
			} else {
				row.Timestamp *= tsMultiplier
			}
		}
	}

	// Trim timestamps if required.
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}
	return err
}
