package common

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

// GetInsertCtx returns InsertCtx from the pool.
//
// Call PutInsertCtx for returning it to the pool.
func GetInsertCtx() *InsertCtx {
	select {
	case ctx := <-insertCtxPoolCh:
		return ctx
	default:
		if v := insertCtxPool.Get(); v != nil {
			return v.(*InsertCtx)
		}
		return &InsertCtx{}
	}
}

// PutInsertCtx returns ctx to the pool.
//
// ctx cannot be used after the call.
func PutInsertCtx(ctx *InsertCtx) {
	ctx.Reset(0)
	select {
	case insertCtxPoolCh <- ctx:
	default:
		insertCtxPool.Put(ctx)
	}
}

var (
	insertCtxPool   sync.Pool
	insertCtxPoolCh = make(chan *InsertCtx, cgroup.AvailableCPUs())
)
