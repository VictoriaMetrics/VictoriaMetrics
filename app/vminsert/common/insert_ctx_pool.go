package common

import (
	"sync"
)

// GetInsertCtx returns InsertCtx from the pool.
//
// Call PutInsertCtx for returning it to the pool.
func GetInsertCtx() *InsertCtx {
	if v := insertCtxPool.Get(); v != nil {
		return v.(*InsertCtx)
	}
	return &InsertCtx{}
}

// PutInsertCtx returns ctx to the pool.
//
// ctx cannot be used after the call.
func PutInsertCtx(ctx *InsertCtx) {
	ctx.Reset(0)
	insertCtxPool.Put(ctx)
}

var insertCtxPool sync.Pool
