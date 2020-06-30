package vmimport

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var maxLineLen = flag.Int("import.maxLineLen", 100*1024*1024, "The maximum length in bytes of a single line accepted by /api/v1/import")

// ParseStream parses /api/v1/import lines from req and calls callback for the parsed rows.
//
// The callback can be called multiple times for streamed data from req.
//
// callback shouldn't hold rows after returning.
func ParseStream(req *http.Request, callback func(rows []Row) error) error {
	readCalls.Inc()
	r := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped vmimport data: %w", err)
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
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlockExt(r, ctx.reqBuf, ctx.tailBuf, *maxLineLen)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read vmimport data: %w", ctx.err)
		}
		return false
	}
	ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf))
	rowsRead.Add(len(ctx.Rows.Rows))
	return true
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="vmimport"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="vmimport"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="vmimport"}`)
)

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
