package graphite

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

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="graphite"}`)
	rowsPerInsert = metrics.NewSummary(`vm_rows_per_insert{type="graphite"}`)
)

// insertHandler processes remote write for graphite plaintext protocol.
//
// See https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol
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
	rowsPerInsert.Update(float64(len(rows)))
	return ic.FlushBufs()
}

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
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(r, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ne, ok := ctx.err.(net.Error); ok && ne.Timeout() {
			// Flush the read data on timeout and try reading again.
			ctx.err = nil
		} else {
			if ctx.err != io.EOF {
				graphiteReadErrors.Inc()
				ctx.err = fmt.Errorf("cannot read graphite plaintext protocol data: %s", ctx.err)
			}
			return false
		}
	}
	ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf))

	// Fill missing timestamps with the current timestamp rounded to seconds.
	currentTimestamp := time.Now().Unix()
	rows := ctx.Rows.Rows
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = currentTimestamp
		}
	}

	// Convert timestamps from seconds to milliseconds.
	for i := range rows {
		rows[i].Timestamp *= 1e3
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
	graphiteReadCalls  = metrics.NewCounter(`vm_read_calls_total{name="graphite"}`)
	graphiteReadErrors = metrics.NewCounter(`vm_read_errors_total{name="graphite"}`)
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
