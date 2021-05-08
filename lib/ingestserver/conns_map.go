package ingestserver

import (
	"net"
	"sync"
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
func (cm *ConnsMap) CloseAll() {
	cm.mu.Lock()
	for c := range cm.m {
		_ = c.Close()
	}
	cm.isClosed = true
	cm.mu.Unlock()
}
