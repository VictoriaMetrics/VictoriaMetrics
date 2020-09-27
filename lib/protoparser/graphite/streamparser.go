package graphite

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var (
	trimTimestamp = flag.Duration("graphiteTrimTimestamp", time.Second, "Trim timestamps for Graphite data to this duration. "+
		"Minimum practical duration is 1s. Higher duration (i.e. 1m) may be used for reducing disk space usage for timestamp data")
)

// ParseStream parses Graphite lines from r and calls callback for the parsed rows.
//
// The callback can be called multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func ParseStream(r io.Reader, callback func(rows []Row) error) error {
	ctx := getStreamContext(r)
	defer putStreamContext(ctx)

	for ctx.Read() {
		if err := callback(ctx.Rows.Rows); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *streamContext) Read() bool {
	readCalls.Inc()
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(ctx.br, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read graphite plaintext protocol data: %w", ctx.err)
		}
		return false
	}
	ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf))
	rowsRead.Add(len(ctx.Rows.Rows))

	rows := ctx.Rows.Rows

	// Fill missing timestamps with the current timestamp rounded to seconds.
	currentTimestamp := int64(fasttime.UnixTimestamp())
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 || r.Timestamp == -1 {
			r.Timestamp = currentTimestamp
		}
	}

	// Convert timestamps from seconds to milliseconds.
	for i := range rows {
		rows[i].Timestamp *= 1e3
	}

	// Trim timestamps if required.
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1000 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}

	return true
}

type streamContext struct {
	Rows    Rows
	br      *bufio.Reader
	reqBuf  []byte
	tailBuf []byte
	err     error
}

func (ctx *streamContext) Error() error {
	if ctx.err == io.EOF {
		return nil
	}
	return ctx.err
}

func (ctx *streamContext) reset() {
	ctx.Rows.Reset()
	ctx.br.Reset(nil)
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.err = nil
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="graphite"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="graphite"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="graphite"}`)
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
var streamContextPoolCh = make(chan *streamContext, runtime.GOMAXPROCS(-1))
