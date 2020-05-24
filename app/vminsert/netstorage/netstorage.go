package netstorage

import (
	"flag"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

var disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage")

// push pushes buf to sn.
//
// It falls back to sending data to another vmstorage node if sn is currently
// unavailable.
//
// rows is the number of rows in the buf.
func (sn *storageNode) push(buf []byte, rows int) error {
	if len(buf) > maxBufSizePerStorageNode {
		logger.Panicf("BUG: len(buf)=%d cannot exceed %d", len(buf), maxBufSizePerStorageNode)
	}
	sn.rowsPushed.Add(rows)

	sn.mu.Lock()
	defer sn.mu.Unlock()

	if sn.broken {
		// The vmstorage node is broken. Re-route buf to healthy vmstorage nodes.
		if !addToReroutedBuf(buf, rows) {
			rowsLostTotal.Add(rows)
			return fmt.Errorf("%d rows dropped because of reroutedBuf overflows %d bytes", rows, reroutedBufMaxSize)
		}
		sn.rowsReroutedFromHere.Add(rows)
		return nil
	}

	if len(sn.buf)+len(buf) <= maxBufSizePerStorageNode {
		// Fast path: the buf contents fits sn.buf.
		sn.buf = append(sn.buf, buf...)
		sn.rows += rows
		return nil
	}

	// Slow path: the buf contents doesn't fit sn.buf.
	// Flush sn.buf to vmstorage and then add buf to sn.buf.
	if err := sn.flushBufLocked(); err != nil {
		// Failed to flush or re-route sn.buf to vmstorage nodes.
		// The sn.buf is already dropped by flushBufLocked.
		// Drop buf too, since there is little sense in trying to rescue it.
		rowsLostTotal.Add(rows)
		return err
	}

	// Successful flush.
	sn.buf = append(sn.buf, buf...)
	sn.rows += rows
	return nil
}

func (sn *storageNode) sendReroutedRow(buf []byte) error {
	sn.mu.Lock()
	defer sn.mu.Unlock()

	if sn.broken {
		return errBrokenStorageNode
	}
	if len(sn.buf)+len(buf) > maxBufSizePerStorageNode {
		return fmt.Errorf("cannot put %d bytes into vmstorage buffer, since its size cannot exceed %d bytes", len(sn.buf)+len(buf), maxBufSizePerStorageNode)
	}
	sn.buf = append(sn.buf, buf...)
	sn.rows++
	return nil
}

var errBrokenStorageNode = fmt.Errorf("the vmstorage node is temporarily broken")

func (sn *storageNode) flushBufLocked() error {
	err := sn.sendBufLocked(sn.buf)
	if err == nil {
		// Successful flush. Remove broken flag.
		sn.broken = false
		sn.rowsSent.Add(sn.rows)
		sn.buf = sn.buf[:0]
		sn.rows = 0
		return nil
	}

	// Couldn't flush sn.buf to vmstorage. Mark sn as broken
	// and try re-routing sn.buf to healthy vmstorage nodes.
	sn.broken = true
	if addToReroutedBuf(sn.buf, sn.rows) {
		// Successfully re-routed data to healthy nodes.
		sn.buf = sn.buf[:0]
		sn.rows = 0
		return nil
	}
	// Preserve sn.buf when it cannot be sent to healthy nodes
	// in the hope the error will disappear on the next call to flushBufLocked.
	//
	// This should fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/294 .
	return err
}

func (sn *storageNode) sendBufLocked(buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	if sn.bc == nil {
		if err := sn.dial(); err != nil {
			return fmt.Errorf("cannot dial %q: %s", sn.dialer.Addr(), err)
		}
	}
	if err := sn.sendToConn(sn.bc, buf); err != nil {
		sn.closeBrokenConn()
		return err
	}
	return nil
}

func (sn *storageNode) sendToConn(bc *handshake.BufferedConn, buf []byte) error {
	timeoutSeconds := len(buf) / 3e5
	if timeoutSeconds < 60 {
		timeoutSeconds = 60
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write deadline to %s: %s", deadline, err)
	}
	// sizeBuf guarantees that the rows batch will be either fully
	// read or fully discarded on the vmstorage side.
	// sizeBuf is used for read optimization in vmstorage.
	sn.sizeBuf = encoding.MarshalUint64(sn.sizeBuf[:0], uint64(len(buf)))
	if _, err := bc.Write(sn.sizeBuf); err != nil {
		return fmt.Errorf("cannot write data size %d: %s", len(buf), err)
	}
	if _, err := bc.Write(buf); err != nil {
		return fmt.Errorf("cannot write data with size %d: %s", len(buf), err)
	}
	if err := bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush data with size %d: %s", len(buf), err)
	}

	// Wait for `ack` from vmstorage.
	// This guarantees that the message has been fully received by vmstorage.
	deadline = time.Now().Add(timeout)
	if err := bc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set read deadline for reading `ack` to vmstorage: %s", err)
	}
	if _, err := io.ReadFull(bc, sn.sizeBuf[:1]); err != nil {
		return fmt.Errorf("cannot read `ack` from vmstorage: %s", err)
	}
	if sn.sizeBuf[0] != 1 {
		return fmt.Errorf("unexpected `ack` received from vmstorage; got %d; want %d", sn.sizeBuf[0], 1)
	}
	return nil
}

func (sn *storageNode) dial() error {
	c, err := sn.dialer.Dial()
	if err != nil {
		sn.dialErrors.Inc()
		return err
	}
	compressionLevel := 1
	if *disableRPCCompression {
		compressionLevel = 0
	}
	bc, err := handshake.VMInsertClient(c, compressionLevel)
	if err != nil {
		_ = c.Close()
		sn.handshakeErrors.Inc()
		return fmt.Errorf("handshake error: %s", err)
	}
	sn.bc = bc
	return nil
}

func (sn *storageNode) closeBrokenConn() {
	if sn.bc == nil {
		return
	}
	_ = sn.bc.Close()
	sn.bc = nil
	sn.connectionErrors.Inc()
}

func (sn *storageNode) run(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	mustStop := false
	for !mustStop {
		select {
		case <-stopCh:
			mustStop = true
			// Make sure flushBufLocked is called last time before returning
			// in order to send the remaining bits of data.
		case <-ticker.C:
		}

		sn.mu.Lock()
		if err := sn.flushBufLocked(); err != nil {
			sn.closeBrokenConn()
			logger.Errorf("cannot flush data to storageNode %q: %s", sn.dialer.Addr(), err)
		}
		sn.mu.Unlock()
	}
}

func rerouteWorker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var buf []byte
	mustStop := false
	for !mustStop {
		select {
		case <-stopCh:
			mustStop = true
			// Make sure spreadReroutedBufToStorageNodes is called last time before returning
			// in order to reroute the remaining data to healthy vmstorage nodes.
		case <-ticker.C:
		}

		var err error
		buf, err = spreadReroutedBufToStorageNodes(buf[:0])
		if err != nil {
			rerouteErrors.Inc()
			logger.Errorf("cannot reroute data among healthy vmstorage nodes: %s", err)
		}
	}
}

// storageNode is a client sending data to vmstorage node.
type storageNode struct {
	mu sync.Mutex

	// Buffer with data that needs to be written to vmstorage node.
	buf []byte

	// The number of rows buf contains at the moment.
	rows int

	// Temporary buffer for encoding marshaled buf size.
	sizeBuf []byte

	// broken is set to true if the given vmstorage node is temporarily unhealthy.
	// In this case the data is re-routed to the remaining healthy vmstorage nodes.
	broken bool

	dialer *netutil.TCPDialer

	bc *handshake.BufferedConn

	// The number of dial errors to vmstorage node.
	dialErrors *metrics.Counter

	// The number of handshake errors to vmstorage node.
	handshakeErrors *metrics.Counter

	// The number of connection errors to vmstorage node.
	connectionErrors *metrics.Counter

	// The number of rows pushed to storageNode with push method.
	rowsPushed *metrics.Counter

	// The number of rows sent to vmstorage node.
	rowsSent *metrics.Counter

	// The number of rows rerouted from the given vmstorage node
	// to healthy nodes when the given node was unhealthy.
	rowsReroutedFromHere *metrics.Counter

	// The number of rows rerouted to the given vmstorage node
	// from other nodes when they were unhealthy.
	rowsReroutedToHere *metrics.Counter
}

// storageNodes contains a list of vmstorage node clients.
var storageNodes []*storageNode

var (
	storageNodesWG  sync.WaitGroup
	rerouteWorkerWG sync.WaitGroup
)

var (
	storageNodesStopCh  = make(chan struct{})
	rerouteWorkerStopCh = make(chan struct{})
)

// InitStorageNodes initializes vmstorage nodes' connections to the given addrs.
func InitStorageNodes(addrs []string) {
	if len(addrs) == 0 {
		logger.Panicf("BUG: addrs must be non-empty")
	}
	if len(addrs) > 255 {
		logger.Panicf("BUG: too much addresses: %d; max supported %d addresses", len(addrs), 255)
	}

	for _, addr := range addrs {
		sn := &storageNode{
			dialer: netutil.NewTCPDialer("vminsert", addr),

			dialErrors:           metrics.NewCounter(fmt.Sprintf(`vm_rpc_dial_errors_total{name="vminsert", addr=%q}`, addr)),
			handshakeErrors:      metrics.NewCounter(fmt.Sprintf(`vm_rpc_handshake_errors_total{name="vminsert", addr=%q}`, addr)),
			connectionErrors:     metrics.NewCounter(fmt.Sprintf(`vm_rpc_connection_errors_total{name="vminsert", addr=%q}`, addr)),
			rowsPushed:           metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_pushed_total{name="vminsert", addr=%q}`, addr)),
			rowsSent:             metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_sent_total{name="vminsert", addr=%q}`, addr)),
			rowsReroutedFromHere: metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_rerouted_from_here_total{name="vminsert", addr=%q}`, addr)),
			rowsReroutedToHere:   metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_rerouted_to_here_total{name="vminsert", addr=%q}`, addr)),
		}
		_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_rows_pending{name="vminsert", addr=%q}`, addr), func() float64 {
			sn.mu.Lock()
			n := sn.rows
			sn.mu.Unlock()
			return float64(n)
		})
		_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_buf_pending_bytes{name="vminsert", addr=%q}`, addr), func() float64 {
			sn.mu.Lock()
			n := len(sn.buf)
			sn.mu.Unlock()
			return float64(n)
		})
		storageNodes = append(storageNodes, sn)
		storageNodesWG.Add(1)
		go func(addr string) {
			sn.run(storageNodesStopCh)
			storageNodesWG.Done()
		}(addr)
	}

	maxBufSizePerStorageNode = memory.Allowed() / 4 / len(storageNodes)
	if maxBufSizePerStorageNode > consts.MaxInsertPacketSize {
		maxBufSizePerStorageNode = consts.MaxInsertPacketSize
	}
	reroutedBufMaxSize = memory.Allowed() / 16
	rerouteWorkerWG.Add(1)
	go func() {
		rerouteWorker(rerouteWorkerStopCh)
		rerouteWorkerWG.Done()
	}()
}

// Stop gracefully stops netstorage.
func Stop() {
	close(rerouteWorkerStopCh)
	rerouteWorkerWG.Wait()

	close(storageNodesStopCh)
	storageNodesWG.Wait()
}

func addToReroutedBuf(buf []byte, rows int) bool {
	reroutedLock.Lock()
	defer reroutedLock.Unlock()
	if len(reroutedBuf)+len(buf) > reroutedBufMaxSize {
		reroutedBufOverflows.Inc()
		return false
	}
	reroutedBuf = append(reroutedBuf, buf...)
	reroutedRows += rows
	reroutesTotal.Inc()
	return true
}

func spreadReroutedBufToStorageNodes(swapBuf []byte) ([]byte, error) {
	healthyStorageNodes := getHealthyStorageNodes()
	if len(healthyStorageNodes) == 0 {
		// No more vmstorage nodes to write data to.
		return swapBuf, fmt.Errorf("all the storage nodes are unhealthy")
	}

	reroutedLock.Lock()
	reroutedBuf, swapBuf = swapBuf[:0], reroutedBuf
	rows := reroutedRows
	reroutedRows = 0
	reroutedLock.Unlock()

	if len(swapBuf) == 0 {
		// Nothing to re-route.
		return swapBuf, nil
	}

	var mr storage.MetricRow
	src := swapBuf
	rowsProcessed := 0
	for len(src) > 0 {
		tail, err := mr.Unmarshal(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal recently marshaled MetricRow: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail

		// Use non-consistent hashing instead of jump hash in order to re-route rows
		// equally among healthy vmstorage nodes.
		// This should spread the increased load among healthy vmstorage nodes.
		h := xxhash.Sum64(mr.MetricNameRaw)
		idx := h % uint64(len(healthyStorageNodes))
		attempts := 0
		for {
			sn := healthyStorageNodes[idx]
			err := sn.sendReroutedRow(rowBuf)
			if err == nil {
				sn.rowsReroutedToHere.Inc()
				break
			}

			// Cannot send data to sn. Try sending to the next vmstorage node.
			idx++
			if idx >= uint64(len(healthyStorageNodes)) {
				idx = 0
			}
			attempts++
			if attempts < len(healthyStorageNodes) {
				continue
			}
			// There are no healthy nodes.
			// Try returning the remaining data to reroutedBuf if it has enough free space.
			rowsRemaining := rows - rowsProcessed
			recovered := false
			reroutedLock.Lock()
			if len(rowBuf)+len(tail)+len(reroutedBuf) <= reroutedBufMaxSize {
				swapBuf = append(swapBuf[:0], rowBuf...)
				swapBuf = append(swapBuf, tail...)
				swapBuf = append(swapBuf, reroutedBuf...)
				reroutedBuf, swapBuf = swapBuf, reroutedBuf[:0]
				reroutedRows += rowsRemaining
				recovered = true
			}
			reroutedLock.Unlock()
			if recovered {
				return swapBuf, nil
			}
			rowsLostTotal.Add(rowsRemaining)
			return swapBuf, fmt.Errorf("all the %d vmstorage nodes are unavailable; lost %d rows; last error: %s", len(storageNodes), rowsRemaining, err)
		}
		rowsProcessed++
	}
	if rowsProcessed != rows {
		logger.Panicf("BUG: unexpected number of rows processed; got %d; want %d", rowsProcessed, rows)
	}
	reroutedRowsProcessed.Add(rowsProcessed)
	return swapBuf, nil
}

var (
	maxBufSizePerStorageNode int

	reroutedLock       sync.Mutex
	reroutedBuf        []byte
	reroutedRows       int
	reroutedBufMaxSize int

	reroutedRowsProcessed = metrics.NewCounter(`vm_rpc_rerouted_rows_processed_total{name="vminsert"}`)
	reroutedBufOverflows  = metrics.NewCounter(`vm_rpc_rerouted_buf_overflows_total{name="vminsert"}`)
	reroutesTotal         = metrics.NewCounter(`vm_rpc_reroutes_total{name="vminsert"}`)
	_                     = metrics.NewGauge(`vm_rpc_rerouted_rows_pending{name="vminsert"}`, func() float64 {
		reroutedLock.Lock()
		n := reroutedRows
		reroutedLock.Unlock()
		return float64(n)
	})
	_ = metrics.NewGauge(`vm_rpc_rerouted_buf_pending_bytes{name="vminsert"}`, func() float64 {
		reroutedLock.Lock()
		n := len(reroutedBuf)
		reroutedLock.Unlock()
		return float64(n)
	})

	rerouteErrors = metrics.NewCounter(`vm_rpc_reroute_errors_total{name="vminsert"}`)
	rowsLostTotal = metrics.NewCounter(`vm_rpc_rows_lost_total{name="vminsert"}`)
)

func getHealthyStorageNodes() []*storageNode {
	sns := make([]*storageNode, 0, len(storageNodes)-1)
	for _, sn := range storageNodes {
		sn.mu.Lock()
		if !sn.broken {
			sns = append(sns, sn)
		}
		sn.mu.Unlock()
	}
	return sns
}
