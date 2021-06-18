package prometheus

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

// ParseStream parses lines with Prometheus exposition format from r and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func ParseStream(r io.Reader, defaultTimestamp int64, isGzipped bool, callback func(rows []Row) error, errLogger func(string)) error {
	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped lines with Prometheus exposition format: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}
	ctx := getStreamContext(r)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.errLogger = errLogger
		uw.callback = func(rows []Row) {
			if err := callback(rows); err != nil {
				ctx.callbackErrLock.Lock()
				if ctx.callbackErr == nil {
					ctx.callbackErr = fmt.Errorf("error when processing imported data: %w", err)
				}
				ctx.callbackErrLock.Unlock()
			}
			ctx.wg.Done()
		}
		uw.defaultTimestamp = defaultTimestamp
		uw.reqBuf, ctx.reqBuf = ctx.reqBuf, uw.reqBuf
		ctx.wg.Add(1)
		common.ScheduleUnmarshalWork(uw)
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
	select {
	case ctx := <-streamContextPoolCh:
		ctx.br.Reset(r)
		return ctx
	default:
		if v := streamContextPool.Get(); v != nil {
			ctx := v.(*streamContext)
			ctx.br.Reset(r)
			return ctx
		}
		return &streamContext{
			br: bufio.NewReaderSize(r, 64*1024),
		}
	}
}

func putStreamContext(ctx *streamContext) {
	ctx.reset()
	select {
	case streamContextPoolCh <- ctx:
	default:
		streamContextPool.Put(ctx)
	}
}

var streamContextPool sync.Pool
var streamContextPoolCh = make(chan *streamContext, cgroup.AvailableCPUs())

type unmarshalWork struct {
	rows             Rows
	callback         func(rows []Row)
	errLogger        func(string)
	defaultTimestamp int64
	reqBuf           []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.callback = nil
	uw.errLogger = nil
	uw.defaultTimestamp = 0
	uw.reqBuf = uw.reqBuf[:0]
}

// Unmarshal implements common.UnmarshalWork
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
		defaultTimestamp = int64(time.Now().UnixNano() / 1e6)
	}
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = defaultTimestamp
		}
	}

	uw.callback(rows)
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
