package netstorage

import (
	"flag"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
	jump "github.com/lithammer/go-jump-consistent-hash"
)

var disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage")

func (sn *storageNode) isBroken() bool {
	return atomic.LoadUint32(&sn.broken) != 0
}

// push pushes buf to sn internal bufs.
//
// This function doesn't block on fast path.
// It may block only if all the storageNodes cannot handle the incoming ingestion rate.
// This blocking provides backpressure to the caller.
//
// The function falls back to sending data to other vmstorage nodes
// if sn is currently unavailable or overloaded.
//
// rows is the number of rows in the buf.
func (sn *storageNode) push(buf []byte, rows int) error {
	if len(buf) > maxBufSizePerStorageNode {
		logger.Panicf("BUG: len(buf)=%d cannot exceed %d", len(buf), maxBufSizePerStorageNode)
	}
	sn.rowsPushed.Add(rows)

	if sn.isBroken() {
		// The vmstorage node is temporarily broken. Re-route buf to healthy vmstorage nodes.
		if err := addToReroutedBuf(buf, rows); err != nil {
			return fmt.Errorf("%d rows dropped because the current vsmtorage is unavailable and %s", rows, err)
		}
		sn.rowsReroutedFromHere.Add(rows)
		return nil
	}

	sn.brLock.Lock()
	if len(sn.br.buf)+len(buf) <= maxBufSizePerStorageNode {
		// Fast path: the buf contents fits sn.buf.
		sn.br.buf = append(sn.br.buf, buf...)
		sn.br.rows += rows
		sn.brLock.Unlock()
		return nil
	}
	sn.brLock.Unlock()

	// Slow path: the buf contents doesn't fit sn.buf.
	// This means that the current vmstorage is slow or will become broken soon.
	// Re-route buf to healthy vmstorage nodes.
	if err := addToReroutedBuf(buf, rows); err != nil {
		return fmt.Errorf("%d rows dropped because the current vmstorage buf is full and %s", rows, err)
	}
	sn.rowsReroutedFromHere.Add(rows)
	return nil
}

var closedCh = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

func (sn *storageNode) run(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var br bufRows
	var bc *handshake.BufferedConn
	var err error
	var waitCh <-chan struct{}
	mustStop := false
	for !mustStop {
		sn.brLock.Lock()
		bufLen := len(sn.br.buf)
		sn.brLock.Unlock()
		waitCh = nil
		if len(br.buf) == 0 && bufLen > maxBufSizePerStorageNode/4 {
			// Do not sleep, since sn.br.buf contains enough data to process.
			waitCh = closedCh
		}
		select {
		case <-stopCh:
			mustStop = true
			// Make sure the sn.buf is flushed last time before returning
			// in order to send the remaining bits of data.
		case <-ticker.C:
		case <-waitCh:
		}
		if len(br.buf) == 0 {
			sn.brLock.Lock()
			sn.br, br = br, sn.br
			sn.brLock.Unlock()
		}
		if bc == nil {
			bc, err = sn.dial()
			if err != nil {
				// Mark sn as broken in order to prevent sending additional data to it until it is recovered.
				atomic.StoreUint32(&sn.broken, 1)
				if len(br.buf) == 0 {
					continue
				}
				logger.Warnf("re-routing %d bytes with %d rows to other storage nodes because cannot dial storageNode %q: %s",
					len(br.buf), br.rows, sn.dialer.Addr(), err)
				if addToReroutedBufNonblock(br.buf, br.rows) {
					sn.rowsReroutedFromHere.Add(br.rows)
					br.reset()
				}
				continue
			}
		}
		if err = sendToConn(bc, br.buf); err == nil {
			// Successfully sent buf to bc. Remove broken flag from sn.
			atomic.StoreUint32(&sn.broken, 0)
			sn.rowsSent.Add(br.rows)
			br.reset()
			continue
		}
		// Couldn't flush buf to sn. Mark sn as broken
		// and try re-routing buf to healthy vmstorage nodes.
		if err = bc.Close(); err != nil {
			logger.Warnf("cannot close connection to storageNode %q: %s", sn.dialer.Addr(), err)
			// continue executing the code below.
		}
		bc = nil
		sn.connectionErrors.Inc()
		atomic.StoreUint32(&sn.broken, 1)
		if addToReroutedBufNonblock(br.buf, br.rows) {
			sn.rowsReroutedFromHere.Add(br.rows)
			br.reset()
		}
	}
}

func sendToConn(bc *handshake.BufferedConn, buf []byte) error {
	if len(buf) == 0 {
		// Nothing to send
		return nil
	}
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
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)
	sizeBuf.B = encoding.MarshalUint64(sizeBuf.B[:0], uint64(len(buf)))
	if _, err := bc.Write(sizeBuf.B); err != nil {
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
	if _, err := io.ReadFull(bc, sizeBuf.B[:1]); err != nil {
		return fmt.Errorf("cannot read `ack` from vmstorage: %s", err)
	}
	if sizeBuf.B[0] != 1 {
		return fmt.Errorf("unexpected `ack` received from vmstorage; got %d; want %d", sizeBuf.B[0], 1)
	}
	return nil
}

var sizeBufPool bytesutil.ByteBufferPool

func (sn *storageNode) dial() (*handshake.BufferedConn, error) {
	c, err := sn.dialer.Dial()
	if err != nil {
		sn.dialErrors.Inc()
		return nil, err
	}
	compressionLevel := 1
	if *disableRPCCompression {
		compressionLevel = 0
	}
	bc, err := handshake.VMInsertClient(c, compressionLevel)
	if err != nil {
		_ = c.Close()
		sn.handshakeErrors.Inc()
		return nil, fmt.Errorf("handshake error: %s", err)
	}
	return bc, nil
}

func rerouteWorker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var br bufRows
	var waitCh <-chan struct{}
	mustStop := false
	for !mustStop {
		reroutedBRLock.Lock()
		bufLen := len(reroutedBR.buf)
		reroutedBRLock.Unlock()
		waitCh = nil
		if len(br.buf) == 0 && bufLen > reroutedBufMaxSize/4 {
			// Do not sleep if reroutedBR contains enough data to process.
			waitCh = closedCh
		}
		select {
		case <-stopCh:
			mustStop = true
			// Make sure reroutedBR is re-routed last time before returning
			// in order to reroute the remaining data to healthy vmstorage nodes.
		case <-ticker.C:
		case <-waitCh:
		}
		if len(br.buf) == 0 {
			reroutedBRLock.Lock()
			reroutedBR, br = br, reroutedBR
			reroutedBRLock.Unlock()
		}
		reroutedBRCond.Broadcast()
		if len(br.buf) == 0 {
			// Nothing to re-route.
			continue
		}
		sns := getHealthyStorageNodes()
		if len(sns) == 0 {
			// No more vmstorage nodes to write data to.
			rerouteErrors.Inc()
			logger.Errorf("cannot send rerouted rows because all the storage nodes are unhealthy")
			// Do not reset br in the hope it could be sent next time.
			continue
		}
		spreadReroutedBufToStorageNodes(sns, &br)
		// There is no need in br.reset() here, since it is already done in spreadReroutedBufToStorageNodes.
	}
	// Notify all the blocked addToReroutedBuf callers, so they may finish the work.
	reroutedBRCond.Broadcast()
}

// storageNode is a client sending data to vmstorage node.
type storageNode struct {
	// broken is set to non-zero if the given vmstorage node is temporarily unhealthy.
	// In this case the data is re-routed to the remaining healthy vmstorage nodes.
	broken uint32

	// brLock protects br.
	brLock sync.Mutex

	// Buffer with data that needs to be written to the storage node.
	br bufRows

	dialer *netutil.TCPDialer

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
			sn.brLock.Lock()
			n := sn.br.rows
			sn.brLock.Unlock()
			return float64(n)
		})
		_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_buf_pending_bytes{name="vminsert", addr=%q}`, addr), func() float64 {
			sn.brLock.Lock()
			n := len(sn.br.buf)
			sn.brLock.Unlock()
			return float64(n)
		})
		storageNodes = append(storageNodes, sn)
		storageNodesWG.Add(1)
		go func(addr string) {
			sn.run(storageNodesStopCh)
			storageNodesWG.Done()
		}(addr)
	}

	maxBufSizePerStorageNode = memory.Allowed() / 8 / len(storageNodes)
	if maxBufSizePerStorageNode > consts.MaxInsertPacketSize {
		maxBufSizePerStorageNode = consts.MaxInsertPacketSize
	}
	reroutedBufMaxSize = memory.Allowed() / 16
	if reroutedBufMaxSize < maxBufSizePerStorageNode {
		reroutedBufMaxSize = maxBufSizePerStorageNode
	}
	if reroutedBufMaxSize > maxBufSizePerStorageNode*len(storageNodes) {
		reroutedBufMaxSize = maxBufSizePerStorageNode * len(storageNodes)
	}
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

// addToReroutedBuf adds buf to reroutedBR.
//
// It waits until the reroutedBR has enough space for buf or if Stop is called.
// This guarantees backpressure if the ingestion rate exceeds vmstorage nodes'
// ingestion rate capacity.
//
// It returns non-nil error only in the following cases:
//
//   - if all the storage nodes are unhealthy.
//   - if Stop is called.
func addToReroutedBuf(buf []byte, rows int) error {
	if len(buf) > reroutedBufMaxSize {
		logger.Panicf("BUG: len(buf)=%d cannot exceed reroutedBufMaxSize=%d", len(buf), reroutedBufMaxSize)
	}

	reroutedBRLock.Lock()
	defer reroutedBRLock.Unlock()

	for len(reroutedBR.buf)+len(buf) > reroutedBufMaxSize {
		if getHealthyStorageNodesCount() == 0 {
			rowsLostTotal.Add(rows)
			return fmt.Errorf("all the vmstorage nodes are unavailable and reroutedBR has no enough space for storing %d bytes; only %d bytes left in reroutedBR",
				len(buf), reroutedBufMaxSize-len(reroutedBR.buf))
		}
		select {
		case <-rerouteWorkerStopCh:
			rowsLostTotal.Add(rows)
			return fmt.Errorf("rerouteWorker cannot send the data since it is stopped")
		default:
		}

		// The reroutedBR.buf has no enough space for len(buf). Wait while the reroutedBR.buf is be sent by rerouteWorker.
		reroutedBufWaits.Inc()
		reroutedBRCond.Wait()
	}
	reroutedBR.buf = append(reroutedBR.buf, buf...)
	reroutedBR.rows += rows
	reroutesTotal.Inc()
	return nil
}

// addToReroutedBufNonblock adds buf to reroutedBR.
//
// It returns true if buf has been successfully added to reroutedBR.
func addToReroutedBufNonblock(buf []byte, rows int) bool {
	if len(buf) > reroutedBufMaxSize {
		logger.Panicf("BUG: len(buf)=%d cannot exceed reroutedBufMaxSize=%d", len(buf), reroutedBufMaxSize)
	}
	reroutedBRLock.Lock()
	ok := len(reroutedBR.buf)+len(buf) <= reroutedBufMaxSize
	if ok {
		reroutedBR.buf = append(reroutedBR.buf, buf...)
		reroutedBR.rows += rows
		reroutesTotal.Inc()
	}
	reroutedBRLock.Unlock()
	return ok
}

func getHealthyStorageNodesCount() int {
	n := 0
	for _, sn := range storageNodes {
		if !sn.isBroken() {
			n++
		}
	}
	return n
}

func getHealthyStorageNodes() []*storageNode {
	sns := make([]*storageNode, 0, len(storageNodes)-1)
	for _, sn := range storageNodes {
		if !sn.isBroken() {
			sns = append(sns, sn)
		}
	}
	return sns
}

func spreadReroutedBufToStorageNodes(sns []*storageNode, br *bufRows) {
	var mr storage.MetricRow
	rowsProcessed := 0
	src := br.buf
	for len(src) > 0 {
		tail, err := mr.Unmarshal(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal MetricRow from reroutedBR.buf: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail

		idx := uint64(0)
		if len(sns) > 1 {
			h := xxhash.Sum64(mr.MetricNameRaw)
			idx = uint64(jump.Hash(h, int32(len(sns))))
		}
		attempts := 0
		for {
			sn := sns[idx]
			if sn.sendReroutedRow(rowBuf) {
				// The row has been successfully re-routed to sn.
				break
			}

			// Cannot re-route data to sn. Try sending to the next vmstorage node.
			idx++
			if idx >= uint64(len(sns)) {
				idx = 0
			}
			attempts++
			if attempts < len(sns) {
				continue
			}

			// There is no enough buffer space in all the vmstorage nodes.
			// Return the remaining data to br.buf, so it may be processed later.
			br.buf = append(br.buf[:0], rowBuf...)
			br.buf = append(br.buf, src...)
			br.rows -= rowsProcessed
			return
		}
		rowsProcessed++
	}
	if rowsProcessed != br.rows {
		logger.Panicf("BUG: unexpected number of rows processed; got %d; want %d", rowsProcessed, br.rows)
	}
	reroutedRowsProcessed.Add(rowsProcessed)
	br.reset()
}

func (sn *storageNode) sendReroutedRow(buf []byte) bool {
	if sn.isBroken() {
		return false
	}
	sn.brLock.Lock()
	ok := len(sn.br.buf)+len(buf) <= maxBufSizePerStorageNode
	if ok {
		sn.br.buf = append(sn.br.buf, buf...)
		sn.br.rows++
		sn.rowsReroutedToHere.Inc()
	}
	sn.brLock.Unlock()
	return ok
}

var (
	maxBufSizePerStorageNode int

	reroutedBR         bufRows
	reroutedBRLock     sync.Mutex
	reroutedBRCond     = sync.NewCond(&reroutedBRLock)
	reroutedBufMaxSize int

	reroutedRowsProcessed = metrics.NewCounter(`vm_rpc_rerouted_rows_processed_total{name="vminsert"}`)
	reroutedBufWaits      = metrics.NewCounter(`vm_rpc_rerouted_buf_waits_total{name="vminsert"}`)
	reroutesTotal         = metrics.NewCounter(`vm_rpc_reroutes_total{name="vminsert"}`)
	_                     = metrics.NewGauge(`vm_rpc_rerouted_rows_pending{name="vminsert"}`, func() float64 {
		reroutedBRLock.Lock()
		n := reroutedBR.rows
		reroutedBRLock.Unlock()
		return float64(n)
	})
	_ = metrics.NewGauge(`vm_rpc_rerouted_buf_pending_bytes{name="vminsert"}`, func() float64 {
		reroutedBRLock.Lock()
		n := len(reroutedBR.buf)
		reroutedBRLock.Unlock()
		return float64(n)
	})

	rerouteErrors = metrics.NewCounter(`vm_rpc_reroute_errors_total{name="vminsert"}`)
	rowsLostTotal = metrics.NewCounter(`vm_rpc_rows_lost_total{name="vminsert"}`)
)
