package prometheus

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

// ParseStream parses lines with Prometheus exposition format from r and calls callback for the parsed rows.
//
// The callback can be called multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func ParseStream(r io.Reader, isGzipped bool, callback func(rows []Row) error) error {
	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped lines with Prometheus exposition format: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}
	ctx := getStreamContext()
	defer putStreamContext(ctx)
	for ctx.Read(r) {
		if err := callback(ctx.Rows.Rows); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *streamContext) Read(r io.Reader) bool {
	readCalls.Inc()
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(r, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read Prometheus exposition data: %w", ctx.err)
		}
		return false
	}
	ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf))
	rowsRead.Add(len(ctx.Rows.Rows))

	rows := ctx.Rows.Rows

	// Fill missing timestamps with the current timestamp.
	currentTimestamp := int64(time.Now().UnixNano() / 1e6)
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = currentTimestamp
		}
	}
	return true
}

type streamContext struct {
	Rows    Rows
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
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.err = nil
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="prometheus"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="prometheus"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="prometheus"}`)
)

func getStreamContext() *streamContext {
	select {
	case ctx := <-streamContextPoolCh:
		return ctx
	default:
		if v := streamContextPool.Get(); v != nil {
			return v.(*streamContext)
		}
		return &streamContext{}
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
