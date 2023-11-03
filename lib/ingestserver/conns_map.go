package ingestserver

import (
	"net"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ConnsMap is used for tracking active connections.
type ConnsMap struct {
	mu       sync.Mutex
	m        map[net.Conn]struct{}
	isClosed bool
}

// Init initializes cm.
func (cm *ConnsMap) Init() {
	cm.m = make(map[net.Conn]struct{})
	cm.isClosed = false
}

// Add adds c to cm.
func (cm *ConnsMap) Add(c net.Conn) bool {
	cm.mu.Lock()
	ok := !cm.isClosed
	if ok {
		cm.m[c] = struct{}{}
	}
	cm.mu.Unlock()
	return ok
}

// Delete deletes c from cm.
func (cm *ConnsMap) Delete(c net.Conn) {
	cm.mu.Lock()
	delete(cm.m, c)
	cm.mu.Unlock()
}

// CloseAll closes all the added conns.
func (cm *ConnsMap) CloseAll(grace time.Duration) {
	cm.mu.Lock()
	var connCloseInterval time.Duration
	conns := len(cm.m)
	if conns == 0 {
		connCloseInterval = 0
	} else {
		connCloseInterval = time.Duration(grace.Milliseconds()/int64(conns)) * time.Millisecond
	}

	logger.Infof("closing %d connections with grace %s and interval %s", conns, grace, connCloseInterval)
	s := time.Now()

	for c := range cm.m {
		_ = c.Close()
		logger.Infof("closed connection from %q, sleeping %s", c.RemoteAddr(), connCloseInterval)
		time.Sleep(connCloseInterval)
	}
	cm.isClosed = true
	cm.mu.Unlock()

	logger.Infof("all %d connections have been closed in %s", conns, time.Since(s))
}
