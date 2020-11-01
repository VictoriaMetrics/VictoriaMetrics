package opentsdbhttp

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxInsertRequestSize = flagutil.NewBytes("opentsdbhttp.maxInsertRequestSize", 32*1024*1024, "The maximum size of OpenTSDB HTTP put request")
	trimTimestamp        = flag.Duration("opentsdbhttpTrimTimestamp", time.Millisecond, "Trim timestamps for OpenTSDB HTTP data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// ParseStream parses OpenTSDB http lines from req and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from req.
// The callback can be called after ParseStream returns.
//
// callback shouldn't hold rows after returning.
func ParseStream(req *http.Request, callback func(rows []Row) error) error {
	readCalls.Inc()
	r := req.Body
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

	uw := getUnmarshalWork()
	uw.callback = callback
	uw.reqBuf, ctx.reqBuf.B = ctx.reqBuf.B, uw.reqBuf
	common.ScheduleUnmarshalWork(uw)
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
	rows     Rows
	callback func(rows []Row) error
	reqBuf   []byte
}

func (uw *unmarshalWork) reset() {
	uw.rows.Reset()
	uw.callback = nil
	uw.reqBuf = uw.reqBuf[:0]
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	p := getJSONParser()
	defer putJSONParser(p)
	v, err := p.ParseBytes(uw.reqBuf)
	if err != nil {
		unmarshalErrors.Inc()
		logger.Errorf("cannot parse HTTP OpenTSDB json: %s", err)
		return
	}
	uw.rows.Unmarshal(v)
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
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}

	if err := uw.callback(rows); err != nil {
		logger.Errorf("error when processing imported data: %s", err)
		putUnmarshalWork(uw)
		return
	}
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
