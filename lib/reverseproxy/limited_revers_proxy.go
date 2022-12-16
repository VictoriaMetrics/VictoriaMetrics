package reverseproxy

import (
	"fmt"
	"net/http"
)

// LimitedReversProxy represents default revers proxy with configurable limit
type LimitedReversProxy struct {
	reversProxy *ReversProxy
	limiter     chan struct{}
	limit       int
}

// NewLimited inits LimitedReversProxy by defined max connections
func NewLimited(maxConn int) *LimitedReversProxy {
	limitedProxy := &LimitedReversProxy{
		limiter:     make(chan struct{}, maxConn),
		reversProxy: New(),
		limit:       maxConn,
	}
	for i := 0; i < maxConn; i++ {
		limitedProxy.limiter <- struct{}{}
	}
	return limitedProxy
}

// ServeHTTP serve requests by limit. If limit reached http error returns
func (lrr *LimitedReversProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-lrr.limiter:
		lrr.reversProxy.handle(w, r)
		lrr.limiter <- struct{}{}
	default:
		message := fmt.Sprintf("cannot handle more than %d connections", lrr.limit)
		http.Error(w, message, http.StatusTooManyRequests)
	}
}
