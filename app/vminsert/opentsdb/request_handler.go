package opentsdb

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
)

var rowsInserted = metrics.NewCounter(`vm_rows_inserted_total{type="opentsdb"}`)

// insertHandler processes remote write for OpenTSDB put protocol.
//
// See http://opentsdb.net/docs/build/html/api_telnet/put.html
func insertHandler(r io.Reader) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(r)
	})
}

func insertHandlerInternal(r io.Reader) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)
	for ctx.Read(r) {
		if err := ctx.InsertRows(); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *pushCtx) InsertRows() error {
	rows := ctx.Rows.Rows
	ic := &ctx.Common
	ic.Reset(len(rows))
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		ic.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabel(tag.Key, tag.Value)
		}
		ic.WriteDataPoint(nil, ic.Labels, r.Timestamp, r.Value)
	}
	rowsInserted.Add(len(rows))
	return ic.FlushBufs()
}

const maxReadPacketSize = 4 * 1024 * 1024

const flushTimeout = 3 * time.Second

func (ctx *pushCtx) Read(r io.Reader) bool {
	opentsdbReadCalls.Inc()
	if ctx.err != nil {
		return false
	}
	if c, ok := r.(net.Conn); ok {
		if err := c.SetReadDeadline(time.Now().Add(flushTimeout)); err != nil {
			opentsdbReadErrors.Inc()
			ctx.err = fmt.Errorf("cannot set read deadline: %s", err)
			return false
		}
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(r, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ne, ok := ctx.err.(net.Error); ok && ne.Timeout() {
			// Flush the read data on timeout and try reading again.
			ctx.err = nil
		} else {
			if ctx.err != io.EOF {
				opentsdbReadErrors.Inc()
				ctx.err = fmt.Errorf("cannot read OpenTSDB put protocol data: %s", ctx.err)
			}
			return false
		}
	}
	if err := ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf)); err != nil {
		opentsdbUnmarshalErrors.Inc()
		ctx.err = fmt.Errorf("cannot unmarshal OpenTSDB put protocol data with size %d: %s", len(ctx.reqBuf), err)
		return false
	}

	// Convert timestamps from seconds to milliseconds
	for i := range ctx.Rows.Rows {
		ctx.Rows.Rows[i].Timestamp *= 1e3
	}
	return true
}

type pushCtx struct {
	Rows   Rows
	Common common.InsertCtx

	reqBuf  []byte
	tailBuf []byte

	err error
}

func (ctx *pushCtx) Error() error {
	if ctx.err == io.EOF {
		return nil
	}
	return ctx.err
}

func (ctx *pushCtx) reset() {
	ctx.Rows.Reset()
	ctx.Common.Reset(0)
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]

	ctx.err = nil
}

var (
	opentsdbReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="opentsdb"}`)
	opentsdbReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="opentsdb"}`)
	opentsdbUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="opentsdb"}`)
)

func getPushCtx() *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			return v.(*pushCtx)
		}
		return &pushCtx{}
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
