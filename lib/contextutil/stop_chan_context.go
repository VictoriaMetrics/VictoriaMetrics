package contextutil

import (
	"context"
	"time"
)

// NewStopChanContext returns new context for the given stopCh, together with cancel function.
//
// The returned context is canceled on the following events:
//
//   - when stopCh is closed
//   - when the returned CancelFunc is called
//
// The caller must call the returned CancelFunc when the context is no longer needed.
func NewStopChanContext(stopCh <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx := &stopChanContext{
		stopCh: stopCh,
	}
	return context.WithCancel(ctx)
}

// stopChanContext implements context.Context for stopCh passed to newStopChanContext.
type stopChanContext struct {
	stopCh <-chan struct{}
}

func (ctx *stopChanContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (ctx *stopChanContext) Done() <-chan struct{} {
	return ctx.stopCh
}

func (ctx *stopChanContext) Err() error {
	select {
	case <-ctx.stopCh:
		return context.Canceled
	default:
		return nil
	}
}

func (ctx *stopChanContext) Value(_ any) any {
	return nil
}
