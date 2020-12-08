package common

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// PushCtx is a context used for populating WriteRequest.
type PushCtx struct {
	WriteRequest prompbmarshal.WriteRequest

	// Labels contains flat list of all the labels used in WriteRequest.
	Labels []prompbmarshal.Label

	// Samples contains flat list of all the samples used in WriteRequest.
	Samples []prompbmarshal.Sample
}

// Reset resets ctx.
func (ctx *PushCtx) Reset() {
	tss := ctx.WriteRequest.Timeseries
	for i := range tss {
		ts := &tss[i]
		ts.Labels = nil
		ts.Samples = nil
	}
	ctx.WriteRequest.Timeseries = ctx.WriteRequest.Timeseries[:0]

	promrelabel.CleanLabels(ctx.Labels)
	ctx.Labels = ctx.Labels[:0]

	ctx.Samples = ctx.Samples[:0]
}

// GetPushCtx returns PushCtx from pool.
//
// Call PutPushCtx when the ctx is no longer needed.
func GetPushCtx() *PushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			return v.(*PushCtx)
		}
		return &PushCtx{}
	}
}

// PutPushCtx returns ctx to the pool.
//
// ctx mustn't be used after returning to the pool.
func PutPushCtx(ctx *PushCtx) {
	ctx.Reset()
	select {
	case pushCtxPoolCh <- ctx:
	default:
		pushCtxPool.Put(ctx)
	}
}

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *PushCtx, cgroup.AvailableCPUs())
