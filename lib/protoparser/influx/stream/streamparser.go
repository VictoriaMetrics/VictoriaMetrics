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
	maxLineSize   = flagutil.NewBytes("influx.maxLineSize", 256*1024, "The maximum size in bytes for a single InfluxDB line during parsing")
	trimTimestamp = flag.Duration("influxTrimTimestamp", time.Millisecond, "Trim timestamps for InfluxDB line protocol data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// Parse parses r with the given args and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func Parse(r io.Reader, isGzipped bool, precision, db string, callback func(db string, rows []influx.Row) error) error {
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

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="influx"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="influx"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="influx"}`)
)

type streamContext struct {
	br      *bufio.Reader
	reqBuf  []byte
	tailBuf []byte
	err     error

	wg              sync.WaitGroup
	callbackErrLock sync.Mutex
	callbackErr     error
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

func (uw *unmarshalWork) runCallback(rows []influx.Row) {
	ctx := uw.ctx
	if err := uw.callback(uw.db, rows); err != nil {
		ctx.callbackErrLock.Lock()
		if ctx.callbackErr == nil {
			ctx.callbackErr = fmt.Errorf("error when processing imported data: %w", err)
		}
		ctx.callbackErrLock.Unlock()
	}
	ctx.wg.Done()
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
