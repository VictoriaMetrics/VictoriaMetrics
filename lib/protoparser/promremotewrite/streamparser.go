package promremotewrite

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var maxInsertRequestSize = flagutil.NewBytes("maxInsertRequestSize", 32*1024*1024, "The maximum size in bytes of a single Prometheus remote_write API request")

// ParseStream parses Prometheus remote_write message req and calls callback for the parsed timeseries.
//
// callback shouldn't hold timeseries after returning.
func ParseStream(req *http.Request, callback func(timeseries []prompb.TimeSeries) error) error {
	ctx := getPushCtx(req.Body)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}
	return callback(ctx.wr.Timeseries)
}

type pushCtx struct {
	wr     prompb.WriteRequest
	br     *bufio.Reader
	reqBuf []byte
}

func (ctx *pushCtx) reset() {
	ctx.wr.Reset()
	ctx.br.Reset(nil)
	ctx.reqBuf = ctx.reqBuf[:0]
}

func (ctx *pushCtx) Read() error {
	readCalls.Inc()
	var err error

	ctx.reqBuf, err = readSnappy(ctx.reqBuf[:0], ctx.br)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read prompb.WriteRequest: %w", err)
	}
	if err = ctx.wr.Unmarshal(ctx.reqBuf); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal prompb.WriteRequest with size %d bytes: %w", len(ctx.reqBuf), err)
	}

	rows := 0
	tss := ctx.wr.Timeseries
	for i := range tss {
		rows += len(tss[i].Samples)
	}
	rowsRead.Add(rows)

	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="promremotewrite"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="promremotewrite"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="promremotewrite"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="promremotewrite"}`)
)

func getPushCtx(r io.Reader) *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		ctx.br.Reset(r)
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			ctx := v.(*pushCtx)
			ctx.br.Reset(r)
			return ctx
		}
		return &pushCtx{
			br: bufio.NewReaderSize(r, 64*1024),
		}
	}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	select {
	case pushCtxPoolCh <- ctx:
	default:
		pushCtxPool.Put(ctx)
	}
}

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *pushCtx, runtime.GOMAXPROCS(-1))

func readSnappy(dst []byte, r io.Reader) ([]byte, error) {
	lr := io.LimitReader(r, int64(maxInsertRequestSize.N)+1)
	bb := bodyBufferPool.Get()
	reqLen, err := bb.ReadFrom(lr)
	if err != nil {
		bodyBufferPool.Put(bb)
		return dst, fmt.Errorf("cannot read compressed request: %w", err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		return dst, fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", maxInsertRequestSize.N)
	}

	buf := dst[len(dst):cap(dst)]
	buf, err = snappy.Decode(buf, bb.B)
	bodyBufferPool.Put(bb)
	if err != nil {
		err = fmt.Errorf("cannot decompress request with length %d: %w", reqLen, err)
		return dst, err
	}
	if len(buf) > maxInsertRequestSize.N {
		return dst, fmt.Errorf("too big unpacked request; mustn't exceed `-maxInsertRequestSize=%d` bytes; got %d bytes", maxInsertRequestSize.N, len(buf))
	}
	if len(buf) > 0 && len(dst) < cap(dst) && &buf[0] == &dst[len(dst):cap(dst)][0] {
		dst = dst[:len(dst)+len(buf)]
	} else {
		dst = append(dst, buf...)
	}
	return dst, nil
}

var bodyBufferPool bytesutil.ByteBufferPool
