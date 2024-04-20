package common

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// PushCtx is a context used for populating WriteRequest.
type PushCtx struct {
	// WriteRequest contains the WriteRequest, which must be pushed later to remote storage.
	//
	// The actual labels and samples for the time series are stored in Labels and Samples fields.
	WriteRequest prompbmarshal.WriteRequest

	// Labels contains flat list of all the labels used in WriteRequest.
	Labels []prompbmarshal.Label

	// Samples contains flat list of all the samples used in WriteRequest.
	Samples []prompbmarshal.Sample
}

// Reset resets ctx.
func (ctx *PushCtx) Reset() {
	ctx.WriteRequest.Reset()

	promrelabel.CleanLabels(ctx.Labels)
	ctx.Labels = ctx.Labels[:0]

	ctx.Samples = ctx.Samples[:0]
}

// GetPushCtx returns PushCtx from pool.
//
// Call PutPushCtx when the ctx is no longer needed.
func GetPushCtx() *PushCtx {
	if v := pushCtxPool.Get(); v != nil {
		return v.(*PushCtx)
	}
	return &PushCtx{}
}

// PutPushCtx returns ctx to the pool.
//
// ctx mustn't be used after returning to the pool.
func PutPushCtx(ctx *PushCtx) {
	ctx.Reset()
	pushCtxPool.Put(ctx)
}

var pushCtxPool sync.Pool
