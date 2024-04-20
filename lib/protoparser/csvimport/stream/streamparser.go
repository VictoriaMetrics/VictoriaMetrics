package stream

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	trimTimestamp = flag.Duration("csvTrimTimestamp", time.Millisecond, "Trim timestamps when importing csv data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// Parse parses csv from req and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from req.
//
// callback shouldn't hold rows after returning.
func Parse(req *http.Request, callback func(rows []csvimport.Row) error) error {
	wcr := writeconcurrencylimiter.GetReader(req.Body)
	defer writeconcurrencylimiter.PutReader(wcr)
	r := io.Reader(wcr)

	q := req.URL.Query()
	format := q.Get("format")
	cds, err := csvimport.ParseColumnDescriptors(format)
	if err != nil {
		return fmt.Errorf("cannot parse the provided csv format: %w", err)
	}
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped csv data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}
	ctx := getStreamContext(r)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.ctx = ctx
		uw.callback = callback
		uw.cds = cds
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
			ctx.err = fmt.Errorf("cannot read csv data: %w", ctx.err)
		}
		return false
	}
	return true
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="csvimport"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="csvimport"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="csvimport"}`)
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
	rows     csvimport.Rows
	ctx      *streamContext
	callback func(rows []csvimport.Row) error
	cds      []csvimport.ColumnDescriptor
	reqBuf   []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.ctx = nil
	uw.callback = nil
	uw.cds = nil
	uw.reqBuf = uw.reqBuf[:0]
}

func (uw *unmarshalWork) runCallback(rows []csvimport.Row) {
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
	uw.rows.Unmarshal(bytesutil.ToUnsafeString(uw.reqBuf), uw.cds)
	rows := uw.rows.Rows
	rowsRead.Add(len(rows))

	// Set missing timestamps
	currentTs := time.Now().UnixNano() / 1e6
	for i := range rows {
		row := &rows[i]
		if row.Timestamp == 0 {
			row.Timestamp = currentTs
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
