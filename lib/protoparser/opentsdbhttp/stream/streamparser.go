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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxInsertRequestSize = flagutil.NewBytes("opentsdbhttp.maxInsertRequestSize", 32*1024*1024, "The maximum size of OpenTSDB HTTP put request")
	trimTimestamp        = flag.Duration("opentsdbhttpTrimTimestamp", time.Millisecond, "Trim timestamps for OpenTSDB HTTP data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// Parse parses OpenTSDB http lines from req and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from req.
//
// callback shouldn't hold rows after returning.
func Parse(req *http.Request, callback func(rows []opentsdbhttp.Row) error) error {
	wcr := writeconcurrencylimiter.GetReader(req.Body)
	defer writeconcurrencylimiter.PutReader(wcr)
	r := io.Reader(wcr)

	readCalls.Inc()
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			readErrors.Inc()
			return fmt.Errorf("cannot read gzipped http protocol data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	ctx := getStreamContext(r)
	defer putStreamContext(ctx)

	// Read the request in ctx.reqBuf
	lr := io.LimitReader(ctx.br, int64(maxInsertRequestSize.N)+1)
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read HTTP OpenTSDB request: %w", err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big HTTP OpenTSDB request; mustn't exceed `-opentsdbhttp.maxInsertRequestSize=%d` bytes", maxInsertRequestSize.N)
	}

	// Process the request synchronously, since there is no sense in processing a single request asynchronously.
	// Sync code is easier to read and understand.
	p := opentsdbhttp.GetJSONParser()
	defer opentsdbhttp.PutJSONParser(p)
	v, err := p.ParseBytes(ctx.reqBuf.B)
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot parse HTTP OpenTSDB json: %w", err)
	}
	rs := getRows()
	defer putRows(rs)
	rs.Unmarshal(v)
	rows := rs.Rows
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
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}

	if err := callback(rows); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

const secondMask int64 = 0x7FFFFFFF00000000

type streamContext struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
}

func (ctx *streamContext) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="opentsdbhttp"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="opentsdbhttp"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentsdbhttp"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="opentsdbhttp"}`)
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

func getRows() *opentsdbhttp.Rows {
	v := rowsPool.Get()
	if v == nil {
		return &opentsdbhttp.Rows{}
	}
	return v.(*opentsdbhttp.Rows)
}

func putRows(rs *opentsdbhttp.Rows) {
	rs.Reset()
	rowsPool.Put(rs)
}

var rowsPool sync.Pool
