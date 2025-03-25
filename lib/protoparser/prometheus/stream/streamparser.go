package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses lines with Prometheus exposition format from r and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
//
// limitConcurrency defines whether to control the number of concurrent calls to this function.
// It is recommended setting limitConcurrency=true if the caller doesn't have concurrency limits set,
// like /api/v1/write calls.
func Parse(r io.Reader, defaultTimestamp int64, encoding string, limitConcurrency bool, callback func(rows []prometheus.Row) error, errLogger func(string)) error {
	reader, err := protoparserutil.GetUncompressedReader(r, encoding)
	if err != nil {
		return fmt.Errorf("cannot decode Prometheus text exposition data: %w", err)
	}
	defer protoparserutil.PutUncompressedReader(reader)

	var wcr *writeconcurrencylimiter.Reader
	if limitConcurrency {
		wcr = writeconcurrencylimiter.GetReader(reader)
		defer writeconcurrencylimiter.PutReader(wcr)
		reader = wcr
	}

	ctx := getStreamContext(reader)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.errLogger = errLogger
		uw.ctx = ctx
		uw.callback = callback
		uw.defaultTimestamp = defaultTimestamp
		uw.reqBuf, ctx.reqBuf = ctx.reqBuf, uw.reqBuf
		ctx.wg.Add(1)
		protoparserutil.ScheduleUnmarshalWork(uw)
		if wcr != nil {
			wcr.DecConcurrency()
		}
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
	ctx.reqBuf, ctx.tailBuf, ctx.err = protoparserutil.ReadLinesBlock(ctx.br, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read Prometheus exposition data: %w", ctx.err)
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
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="prometheus"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="prometheus"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="prometheus"}`)
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
	rows             prometheus.Rows
	ctx              *streamContext
	callback         func(rows []prometheus.Row) error
	errLogger        func(string)
	defaultTimestamp int64
	reqBuf           []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.ctx = nil
	uw.callback = nil
	uw.errLogger = nil
	uw.defaultTimestamp = 0
	uw.reqBuf = uw.reqBuf[:0]
}

func (uw *unmarshalWork) runCallback(rows []prometheus.Row) {
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

// Unmarshal implements protoparserutil.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	if uw.errLogger != nil {
		uw.rows.UnmarshalWithErrLogger(bytesutil.ToUnsafeString(uw.reqBuf), uw.errLogger)
	} else {
		uw.rows.Unmarshal(bytesutil.ToUnsafeString(uw.reqBuf))
	}
	rows := uw.rows.Rows
	rowsRead.Add(len(rows))

	// Fill missing timestamps with the current timestamp.
	defaultTimestamp := uw.defaultTimestamp
	if defaultTimestamp <= 0 {
		defaultTimestamp = time.Now().UnixNano() / 1e6
	}
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = defaultTimestamp
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
