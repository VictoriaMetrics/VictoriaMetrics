package ingestserver

import (
	"net"
	"sort"
	"sync"
	"time"
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

	// Sort addresses in order to make the order of closing connections deterministic.
	addresses := make([]net.Conn, 0)
	for c := range cm.m {
		addresses = append(addresses, c)
	}
	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i].RemoteAddr().String() < addresses[j].RemoteAddr().String()
	})

	for _, c := range addresses {
		_ = c.Close()
		time.Sleep(connCloseInterval)
	}
	cm.isClosed = true
	cm.mu.Unlock()
}
