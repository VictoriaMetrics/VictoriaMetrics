package graphite

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
)

var rowsInserted = metrics.NewCounter(`vm_rows_inserted_total{type="graphite"}`)

// insertHandler processes remote write for graphite plaintext protocol.
//
// See https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol
func insertHandler(at *auth.Token, r io.Reader) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(at, r)
	})
}

func insertHandlerInternal(at *auth.Token, r io.Reader) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)
	for ctx.Read(r) {
		if err := ctx.InsertRows(at); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *pushCtx) InsertRows(at *auth.Token) error {
	rows := ctx.Rows.Rows
	ic := &ctx.Common
	ic.Reset()
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		ic.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabel(tag.Key, tag.Value)
		}
		if err := ic.WriteDataPoint(at, ic.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
	}
	rowsInserted.Add(len(rows))
	return ic.FlushBufs()
}

const maxReadPacketSize = 4 * 1024 * 1024

const flushTimeout = 3 * time.Second

func (ctx *pushCtx) Read(r io.Reader) bool {
	graphiteReadCalls.Inc()
	if ctx.err != nil {
		return false
	}
	if c, ok := r.(net.Conn); ok {
		if err := c.SetReadDeadline(time.Now().Add(flushTimeout)); err != nil {
			graphiteReadErrors.Inc()
			ctx.err = fmt.Errorf("cannot set read deadline: %s", err)
			return false
		}
	}
	lr := io.LimitReader(r, maxReadPacketSize)
	ctx.reqBuf.Reset()
	ctx.reqBuf.B = append(ctx.reqBuf.B[:0], ctx.tailBuf...)
	n, err := io.CopyBuffer(&ctx.reqBuf, lr, ctx.copyBuf[:])
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			// Flush the read data on timeout and try reading again.
		} else {
			graphiteReadErrors.Inc()
			ctx.err = fmt.Errorf("cannot read graphite plaintext protocol data: %s", err)
			return false
		}
	} else if n < maxReadPacketSize {
		// Mark the end of stream.
		ctx.err = io.EOF
	}

	// Parse all the rows until the last newline in ctx.reqBuf.B
	nn := bytes.LastIndexByte(ctx.reqBuf.B, '\n')
	ctx.tailBuf = ctx.tailBuf[:0]
	if nn >= 0 {
		ctx.tailBuf = append(ctx.tailBuf[:0], ctx.reqBuf.B[nn+1:]...)
		ctx.reqBuf.B = ctx.reqBuf.B[:nn]
	}
	if err = ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf.B)); err != nil {
		graphiteUnmarshalErrors.Inc()
		ctx.err = fmt.Errorf("cannot unmarshal graphite plaintext protocol data with size %d: %s", len(ctx.reqBuf.B), err)
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
	Common netstorage.InsertCtx

	reqBuf  bytesutil.ByteBuffer
	tailBuf []byte
	copyBuf [16 * 1024]byte

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
	ctx.Common.Reset()
	ctx.reqBuf.Reset()
	ctx.tailBuf = ctx.tailBuf[:0]

	ctx.err = nil
}

var (
	graphiteReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="graphite"}`)
	graphiteReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="graphite"}`)
	graphiteUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="graphite"}`)
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
