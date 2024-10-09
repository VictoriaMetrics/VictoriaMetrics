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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxLineSize    = flagutil.NewBytes("influx.maxLineSize", 256*1024, "The maximum size in bytes for a single InfluxDB line during parsing. Applicable for stream mode only.")
	maxRequestSize = flagutil.NewBytes("influx.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single InfluxDB request. Applicable for batch mode only.")
	trimTimestamp  = flag.Duration("influxTrimTimestamp", time.Millisecond, "Trim timestamps for InfluxDB line protocol data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
	testMode = false
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

	ctx := getProcessingContext(r)
	defer putProcessingContext(ctx)
	if !isStreamMode {
		ctx.Read()
		uw := getUnmarshalWork()
		uw.ctx = ctx
		uw.callback = callback
		uw.isStreamMode = isStreamMode
		uw.db = db
		uw.tsMultiplier = tsMultiplier
		uw.reqBuf, ctx.reqBuf.B = ctx.reqBuf.B, uw.reqBuf
		common.ScheduleUnmarshalWork(uw)
		return ctx.Error()
	}
	for ctx.ReadLine() {
		uw := getUnmarshalWork()
		uw.ctx = ctx
		uw.callback = callback
		uw.db = db
		uw.isStreamMode = isStreamMode
		uw.tsMultiplier = tsMultiplier
		uw.reqBuf, ctx.reqBuf.B = ctx.reqBuf.B, uw.reqBuf
		ctx.wg.Add(1)
		common.ScheduleUnmarshalWork(uw)
		if !testMode {
			wcr.DecConcurrency()
		}
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

type processingContext struct {
	br      *bufio.Reader
	reqBuf  bytesutil.ByteBuffer
	tailBuf []byte
	err     error

	wg              sync.WaitGroup
	callbackErrLock sync.Mutex
	callbackErr     error
}

func (ctx *processingContext) Read() {
	var reqLen int64
	lr := io.LimitReader(ctx.br, int64(maxRequestSize.IntN()))
	reqLen, ctx.err = ctx.reqBuf.ReadFrom(lr)
	if ctx.err != nil && reqLen > int64(maxRequestSize.IntN()) {
		ctx.err = fmt.Errorf("too big request; mustn't exceed -influx.maxRequestSize=%d bytes", maxRequestSize.N)
	}
}

func (ctx *processingContext) ReadLine() bool {
	readCalls.Inc()
	if ctx.err != nil || ctx.hasCallbackError() {
		return false
	}
	ctx.reqBuf.B, ctx.tailBuf, ctx.err = common.ReadLinesBlockExt(ctx.br, ctx.reqBuf.B, ctx.tailBuf, maxLineSize.IntN())
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read influx line protocol data: %w", ctx.err)
		}
		return false
	}
	return true
}

func (ctx *processingContext) Error() error {
	if ctx.err == io.EOF {
		return nil
	}
	return ctx.err
}

func (ctx *processingContext) hasCallbackError() bool {
	ctx.callbackErrLock.Lock()
	ok := ctx.callbackErr != nil
	ctx.callbackErrLock.Unlock()
	return ok
}

func (ctx *processingContext) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.err = nil
	ctx.callbackErr = nil
}

func getProcessingContext(r io.Reader) *processingContext {
	if v := processingContextPool.Get(); v != nil {
		ctx := v.(*processingContext)
		ctx.br.Reset(r)
		return ctx
	}
	return &processingContext{
		br: bufio.NewReaderSize(r, 64*1024),
	}
}

func putProcessingContext(ctx *processingContext) {
	ctx.reset()
	processingContextPool.Put(ctx)
}

var processingContextPool sync.Pool

type unmarshalWork struct {
	rows         influx.Rows
	ctx          *processingContext
	callback     func(db string, rows []influx.Row) error
	db           string
	tsMultiplier int64
	isStreamMode bool
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

func (uw *unmarshalWork) runCallback(rows []influx.Row) {
	ctx := uw.ctx
	if err := uw.callback(uw.db, rows); err != nil {
		err = fmt.Errorf("error when processing imported data: %w", err)
		if !uw.isStreamMode {
			logger.Errorf("failed to parse Influx batch data: %s", err)
			return
		}
		ctx.callbackErrLock.Lock()
		if ctx.callbackErr == nil {
			ctx.callbackErr = err
		}
		ctx.callbackErrLock.Unlock()
	}
	if uw.isStreamMode {
		ctx.wg.Done()
	}
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	uw.rows.Unmarshal(bytesutil.ToUnsafeString(uw.reqBuf))
	rows := uw.rows.Rows
	rowsRead.Add(len(rows))

	// Adjust timestamps according to uw.tsMultiplier
	currentTs := time.Now().UnixNano() / 1e6
	tsMultiplier := uw.tsMultiplier
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

	uw.runCallback(rows)
	putUnmarshalWork(uw)
}

func getUnmarshalWork() *unmarshalWork {
	v := unmarshalWorkPool.Get()
	if v == nil {
		return &unmarshalWork{}
	}
	return v.(*unmarshalWork)
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
