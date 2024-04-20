package stream

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	trimTimestamp = flag.Duration("opentsdbTrimTimestamp", time.Second, "Trim timestamps for OpenTSDB 'telnet put' data to this duration. "+
		"Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data")
)

// Parse parses OpenTSDB lines from r and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func Parse(r io.Reader, callback func(rows []opentsdb.Row) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	ctx := getStreamContext(r)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.ctx = ctx
		uw.callback = callback
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
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(ctx.br, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read OpenTSDB put protocol data: %w", ctx.err)
		}
		return false
	}
	return true
}

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

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="opentsdb"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="opentsdb"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentsdb"}`)
)

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
	rows     opentsdb.Rows
	ctx      *streamContext
	callback func(rows []opentsdb.Row) error
	reqBuf   []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.ctx = nil
	uw.callback = nil
	uw.reqBuf = uw.reqBuf[:0]
}

func (uw *unmarshalWork) runCallback(rows []opentsdb.Row) {
	ctx := uw.ctx
	if err := uw.callback(rows); err != nil {
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

	// Fill in missing timestamps
	currentTimestamp := int64(fasttime.UnixTimestamp())
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = currentTimestamp
		}
	}

	// Convert timestamps in seconds to milliseconds if needed.
	// See http://opentsdb.net/docs/javadoc/net/opentsdb/core/Const.html#SECOND_MASK
	for i := range rows {
		r := &rows[i]
		if r.Timestamp&secondMask == 0 {
			r.Timestamp *= 1e3
		}
	}

	// Trim timestamps if required.
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1000 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}

	uw.runCallback(rows)
	putUnmarshalWork(uw)
}

const secondMask int64 = 0x7FFFFFFF00000000

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
