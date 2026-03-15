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
	conns := make([]remoteConns, 0, len(cm.m))
	connsByIP := make(map[string]int, len(cm.m))

	// group remote connection by IP address
	// it's needed to properly close multiple opened connections
	// from the same instance at once
	for c := range cm.m {
		remoteIP, _, _ := net.SplitHostPort(c.RemoteAddr().String())
		idx, ok := connsByIP[remoteIP]
		if !ok {
			connsByIP[remoteIP] = len(conns)
			conns = append(conns, remoteConns{remoteIP: remoteIP, clientName: cm.clientName})
			idx = len(conns) - 1
		}
		rcs := &conns[idx]
		rcs.conns = append(rcs.conns, c)
		delete(cm.m, c)
	}
	cm.isClosed = true
	cm.mu.Unlock()

	if shutdownDuration <= 0 {
		// Close all the connections at once.
		for _, c := range conns {
			c.closeAll()
		}
		return
	}
	if len(conns) == 0 {
		return
	}
	if len(conns) == 1 {
		// Simple case - just close a single connection and that's it!
		conns[0].closeAll()
		return
	}

	// Sort conns in order to make the order of closing connections deterministic across clients.
	// This should reduce resource usage spikes at clients during rolling restarts.
	sort.Slice(conns, func(i, j int) bool {
		return conns[i].remoteIP < conns[j].remoteIP
	})

	shutdownInterval := shutdownDuration / time.Duration(len(conns)-1)
	startTime := time.Now()
	logger.Infof("closing %d %s connections with %dms interval between them", len(conns), cm.clientName, shutdownInterval.Milliseconds())
	conns[0].closeAll()
	for _, c := range conns[1:] {
		time.Sleep(shutdownInterval)
		c.closeAll()
	}
	logger.Infof("closed %d %s connections in %s", len(conns), cm.clientName, time.Since(startTime))
}

type remoteConns struct {
	clientName string
	remoteIP   string
	conns      []net.Conn
}

func (rcs *remoteConns) closeAll() {
	for _, c := range rcs.conns {
		remoteAddr := c.RemoteAddr().String()
		_ = c.Close()
		logger.Infof("closed %s connection %s", rcs.clientName, remoteAddr)
	}
}
