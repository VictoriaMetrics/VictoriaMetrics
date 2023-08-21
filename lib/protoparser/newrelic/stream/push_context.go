package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	maxInsertRequestSize = flagutil.NewBytes("newrelic.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single NewRelic POST request to /infra/v2/metrics/events/bulk")
)

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="newrelic"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="newrelic"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="newrelic"}`)
)

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *pushCtx, cgroup.AvailableCPUs())

type pushCtx struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) Read() error {
	readCalls.Inc()
	lr := io.LimitReader(ctx.br, maxInsertRequestSize.N+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read compressed request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > maxInsertRequestSize.N {
		readErrors.Inc()
		return fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", maxInsertRequestSize.N)
	}
	return nil
}

func (ctx *pushCtx) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
}

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
