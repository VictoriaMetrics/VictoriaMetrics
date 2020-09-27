package vmimport

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var maxLineLen = flagutil.NewBytes("import.maxLineLen", 100*1024*1024, "The maximum length in bytes of a single line accepted by /api/v1/import; "+
	"the line length can be limited with `max_rows_per_line` query arg passed to /api/v1/export")

// ParseStream parses /api/v1/import lines from req and calls callback for the parsed rows.
//
// The callback can be called multiple times for streamed data from req.
//
// callback shouldn't hold rows after returning.
// callback is called from multiple concurrent goroutines.
func ParseStream(req *http.Request, callback func(rows []Row) error) error {
	r := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped vmimport data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	// Start gomaxprocs workers for processing the parsed data in parallel.
	gomaxprocs := runtime.GOMAXPROCS(-1)
	workCh := make(chan *unmarshalWork, 8*gomaxprocs)
	var wg sync.WaitGroup
	defer func() {
		close(workCh)
		wg.Wait()
	}()
	wg.Add(gomaxprocs)
	for i := 0; i < gomaxprocs; i++ {
		go func() {
			defer wg.Done()
			for uw := range workCh {
				uw.rows.Unmarshal(bytesutil.ToUnsafeString(uw.reqBuf))
				rows := uw.rows.Rows
				for i := range rows {
					row := &rows[i]
					rowsRead.Add(len(row.Timestamps))
				}
				if err := callback(rows); err != nil {
					logger.Errorf("error when processing imported data: %s", err)
					putUnmarshalWork(uw)
					continue
				}
				putUnmarshalWork(uw)
			}
		}()
	}

	ctx := getStreamContext(r)
	defer putStreamContext(ctx)
	for ctx.Read() {
		uw := getUnmarshalWork()
		uw.reqBuf = append(uw.reqBuf[:0], ctx.reqBuf...)
		workCh <- uw
	}
	return ctx.Error()
}

func (ctx *streamContext) Read() bool {
	readCalls.Inc()
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlockExt(ctx.br, ctx.reqBuf, ctx.tailBuf, maxLineLen.N)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read vmimport data: %w", ctx.err)
		}
		return false
	}
	return true
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="vmimport"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="vmimport"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="vmimport"}`)
)

type streamContext struct {
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
	ctx.br.Reset(nil)
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.err = nil
}

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

type unmarshalWork struct {
	rows   Rows
	reqBuf []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.reqBuf = uw.reqBuf[:0]
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
