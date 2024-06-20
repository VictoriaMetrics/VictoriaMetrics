package ingestserver

import (
	"net"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ConnsMap is used for tracking active connections.
type ConnsMap struct {
	clientName string

	mu       sync.Mutex
	m        map[net.Conn]struct{}
	isClosed bool
}

// Init initializes cm.
func (cm *ConnsMap) Init(clientName string) {
	cm.clientName = clientName
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

// CloseAll gradually closes all the cm conns with during the given shutdownDuration.
//
// If shutdownDuration <= 0, then all the connections are closed simultaneously.
func (cm *ConnsMap) CloseAll(shutdownDuration time.Duration) {
	cm.mu.Lock()
	conns := make([]net.Conn, 0, len(cm.m))
	for c := range cm.m {
		conns = append(conns, c)
		delete(cm.m, c)
	}
	cm.isClosed = true
	cm.mu.Unlock()

	if shutdownDuration <= 0 {
		// Close all the connections at once.
		for _, c := range conns {
			_ = c.Close()
		}
		return
	}
	if len(conns) == 0 {
		return
	}
	if len(conns) == 1 {
		// Simple case - just close a single connection and that's it!
		_ = conns[0].Close()
		return
	}

	// Sort conns in order to make the order of closing connections deterministic across clients.
	// This should reduce resource usage spikes at clients during rolling restarts.
	sort.Slice(conns, func(i, j int) bool {
		return conns[i].RemoteAddr().String() < conns[j].RemoteAddr().String()
	})

	shutdownInterval := shutdownDuration / time.Duration(len(conns)-1)
	startTime := time.Now()
	logger.Infof("closing %d %s connections with %dms interval between them", len(conns), cm.clientName, shutdownInterval.Milliseconds())
	remoteAddr := conns[0].RemoteAddr().String()
	_ = conns[0].Close()
	logger.Infof("closed %s connection %s", cm.clientName, remoteAddr)
	for _, c := range conns[1:] {
		time.Sleep(shutdownInterval)
		remoteAddr := c.RemoteAddr().String()
		_ = c.Close()
		logger.Infof("closed %s connection %s", cm.clientName, remoteAddr)
	}
	logger.Infof("closed %d %s connections in %s", len(conns), cm.clientName, time.Since(startTime))
}
