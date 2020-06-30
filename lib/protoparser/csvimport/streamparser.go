package csvimport

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var (
	trimTimestamp = flag.Duration("csvTrimTimestamp", time.Millisecond, "Trim timestamps when importing csv data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// ParseStream parses csv from req and calls callback for the parsed rows.
//
// The callback can be called multiple times for streamed data from req.
//
// callback shouldn't hold rows after returning.
func ParseStream(req *http.Request, callback func(rows []Row) error) error {
	readCalls.Inc()
	q := req.URL.Query()
	format := q.Get("format")
	cds, err := ParseColumnDescriptors(format)
	if err != nil {
		return fmt.Errorf("cannot parse the provided csv format: %w", err)
	}
	r := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped csv data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	ctx := getStreamContext()
	defer putStreamContext(ctx)
	for ctx.Read(r, cds) {
		if err := callback(ctx.Rows.Rows); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *streamContext) Read(r io.Reader, cds []ColumnDescriptor) bool {
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(r, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read csv data: %w", ctx.err)
		}
		return false
	}
	ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf), cds)
	rowsRead.Add(len(ctx.Rows.Rows))

	rows := ctx.Rows.Rows

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

	return true
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="csvimport"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="csvimport"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="csvimport"}`)
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
