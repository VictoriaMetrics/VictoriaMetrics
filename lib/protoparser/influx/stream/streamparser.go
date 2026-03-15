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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxLineSize = flagutil.NewBytes("influx.maxLineSize", 256*1024, "The maximum size in bytes for a single InfluxDB line during parsing. Applicable for stream mode only. "+
		"See https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/")
	maxRequestSize = flagutil.NewBytes("influx.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single InfluxDB request. Applicable for batch mode only. "+
		"See https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/")
	trimTimestamp = flag.Duration("influxTrimTimestamp", time.Millisecond, "Trim timestamps for InfluxDB line protocol data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
	forceStreamMode = flag.Bool("influx.forceStreamMode", false, "Force stream mode parsing for ingested data. "+
		"See https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/")
)

// Parse parses r with the given args and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func Parse(r io.Reader, encoding string, isStreamMode bool, precision, db string, callback func(db string, rows []influx.Row) error) error {
	tsMultiplier := getTimestampMultiplier(precision)

	if *forceStreamMode || isStreamMode {
		// Process lines in a streaming fashion. Invalid lines are skipped.
		return parseStreamMode(r, encoding, tsMultiplier, db, callback)
	}

	// Process the whole request in one go.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7090
	readCalls.Inc()
	err := protoparserutil.ReadUncompressedData(r, encoding, maxRequestSize, func(data []byte) error {
		ctx := getBatchContext()
		defer putBatchContext(ctx)

		if err := unmarshal(&ctx.rows, data, tsMultiplier, false); err != nil {
			return err
		}
		return callback(db, ctx.rows.Rows)
	})
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot process influx line protocol data: %w; see https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/", err)
	}
	return nil
}

func parseStreamMode(r io.Reader, encoding string, tsMultiplier int64, db string, callback func(db string, rows []influx.Row) error) error {
	wcr, err := writeconcurrencylimiter.GetReader(r)
	if err != nil {
		return err
	}
	defer writeconcurrencylimiter.PutReader(wcr)

	reader, err := protoparserutil.GetUncompressedReader(wcr, encoding)
	if err != nil {
		return fmt.Errorf("cannot decode influx line protocol data: %w; see https://docs.victoriametrics.com/victoriametrics/integrations/influxdb/", err)
	}
	defer protoparserutil.PutUncompressedReader(reader)

	ctx := getStreamContext(reader)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.ctx = ctx
		uw.callback = callback
		uw.db = db
		uw.tsMultiplier = tsMultiplier
		uw.reqBuf, ctx.reqBuf = ctx.reqBuf, uw.reqBuf
		ctx.wg.Add(1)
		protoparserutil.ScheduleUnmarshalWork(uw)
	}
	ctx.wg.Wait()
	if err := ctx.Error(); err != nil {
		return err
	}
	return ctx.callbackErr
}

func getTimestampMultiplier(precision string) int64 {
	switch precision {
	case "ns":
		return 1e6
	case "u", "us", "µ":
		return 1e3
	case "ms":
		return 1
	case "s":
		return -1e3
	case "m":
		return -1e3 * 60
	case "h":
		return -1e3 * 3600
	default:
		return 0
	}
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="influx"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="influx"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="influx"}`)
)

type batchContext struct {
	rows influx.Rows
}

func (ctx *batchContext) reset() {
	ctx.rows.Reset()
}

func getBatchContext() *batchContext {
	if v := batchContextPool.Get(); v != nil {
		ctx := v.(*batchContext)
		return ctx
	}
	return &batchContext{}
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
	ctx.reqBuf, ctx.tailBuf, ctx.err = protoparserutil.ReadLinesBlockExt(ctx.br, ctx.reqBuf, ctx.tailBuf, maxLineSize.IntN())
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

// Unmarshal implements protoparserutil.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	if err := unmarshal(&uw.rows, uw.reqBuf, uw.tsMultiplier, true); err != nil {
		logger.Panicf("BUG: unexpected non-nil error when rows must be ignored: %s", err)
	}

	uw.runCallback()
	putUnmarshalWork(uw)
}

func getUnmarshalWork() *unmarshalWork {
	v := unmarshalWorkPool.Get()
	if v == nil {
		v = &unmarshalWork{}
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

func unmarshal(rs *influx.Rows, reqBuf []byte, tsMultiplier int64, skipInvalidLines bool) error {
	if err := rs.Unmarshal(bytesutil.ToUnsafeString(reqBuf), skipInvalidLines); err != nil {
		return err
	}

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

	return nil
}
