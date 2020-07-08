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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

var (
	disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage")
	replicationFactor     = flag.Int("replicationFactor", 1, "Replication factor for the ingested data, i.e. how many copies to make among distinct -storageNode instances. "+
		"Note that vmselect must run with -dedup.minScrapeInterval=1ms for data de-duplication when replicationFactor is greater than 1. "+
		"Higher values for -dedup.minScrapeInterval at vmselect is OK")
)

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
		if err := addToReroutedBufMayBlock(buf, rows); err != nil {
			return fmt.Errorf("%d rows dropped because the current vsmtorage is unavailable and %w", rows, err)
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
	if err := addToReroutedBufMayBlock(buf, rows); err != nil {
		return fmt.Errorf("%d rows dropped because the current vmstorage buf is full and %w", rows, err)
	}
	sn.rowsReroutedFromHere.Add(rows)
	return nil
}

var closedCh = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

func (sn *storageNode) run(stopCh <-chan struct{}, snIdx int) {
	replicas := *replicationFactor
	if replicas <= 0 {
		replicas = 1
	}
	if replicas > len(storageNodes) {
		replicas = len(storageNodes)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var br bufRows
	brLastResetTime := fasttime.UnixTimestamp()
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
		currentTime := fasttime.UnixTimestamp()
		if len(br.buf) < cap(br.buf)/4 && currentTime-brLastResetTime > 10 {
			// Free up capacity space occupied by br.buf in order to reduce memory usage after spikes.
			br.buf = append(br.buf[:0:0], br.buf...)
			brLastResetTime = currentTime
		}
		if len(br.buf) == 0 {
			// Nothing to send. Just check sn health, so it could be returned to non-broken state.
			sn.checkHealth()
			continue
		}

		// Send br to replicas storageNodes starting from snIdx.
		if !sendBufToReplicas(&br, snIdx, replicas) {
			// do not reset br in the hope it will be sent next time.
			continue
		}
		br.reset()
	}
}

func sendBufToReplicas(br *bufRows, snIdx, replicas int) bool {
	usedStorageNodes := make(map[*storageNode]bool, replicas)
	for i := 0; i < replicas; i++ {
		idx := snIdx + i
		attempts := 0
		for {
			attempts++
			if attempts > len(storageNodes) {
				if i == 0 {
					// The data wasn't replicated at all.
					logger.Warnf("cannot push %d bytes with %d rows to storage nodes, since all the nodes are temporarily unavailable; "+
						"re-trying to send the data soon", len(br.buf), br.rows)
					return false
				}
				// The data is partially replicated, so just emit a warning and return true.
				// We could retry sending the data again, but this may result in uncontrolled duplicate data.
				// So it is better returning true.
				rowsIncompletelyReplicatedTotal.Add(br.rows)
				logger.Warnf("cannot make a copy #%d out of %d copies according to -replicationFactor=%d for %d bytes with %d rows, "+
					"since a part of storage nodes is temporarily unavailable", i+1, replicas, *replicationFactor, len(br.buf), br.rows)
				return true
			}
			if idx >= len(storageNodes) {
				idx %= len(storageNodes)
			}
			sn := storageNodes[idx]
			idx++
			if usedStorageNodes[sn] {
				// The br has been already replicated to sn. Skip it.
				continue
			}
			if !sn.sendBufRows(br) {
				// Cannot send data to sn. Go to the next sn.
				continue
			}
			// Successfully sent data to sn.
			usedStorageNodes[sn] = true
			break
		}
	}
	return true
}

func (sn *storageNode) checkHealth() {
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()

	if !sn.isBroken() {
		return
	}
	if sn.bc != nil {
		logger.Panicf("BUG: sn.bc must be nil when sn is broken; got %p", sn.bc)
	}
	bc, err := sn.dial()
	if err != nil {
		logger.Warnf("cannot dial storageNode %q: %s", sn.dialer.Addr(), err)
		return
	}
	sn.bc = bc
	atomic.StoreUint32(&sn.broken, 0)
}

func (sn *storageNode) sendBufRows(br *bufRows) bool {
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()

	if sn.bc == nil {
		bc, err := sn.dial()
		if err != nil {
			// Mark sn as broken in order to prevent sending additional data to it until it is recovered.
			atomic.StoreUint32(&sn.broken, 1)
			logger.Warnf("cannot dial storageNode %q: %s", sn.dialer.Addr(), err)
			return false
		}
		sn.bc = bc
	}
	err := sendToConn(sn.bc, br.buf)
	if err == nil {
		// Successfully sent buf to bc. Remove broken flag from sn.
		atomic.StoreUint32(&sn.broken, 0)
		sn.rowsSent.Add(br.rows)
		return true
	}
	// Couldn't flush buf to sn. Mark sn as broken.
	logger.Warnf("cannot send %d bytes with %d rows to %q: %s; re-routing this data to healthy storage nodes", len(br.buf), br.rows, sn.dialer.Addr(), err)
	if err = sn.bc.Close(); err != nil {
		logger.Warnf("cannot close connection to storageNode %q: %s", sn.dialer.Addr(), err)
	}
	sn.bc = nil
	sn.connectionErrors.Inc()
	atomic.StoreUint32(&sn.broken, 1)
	return false
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
		return fmt.Errorf("cannot set write deadline to %s: %w", deadline, err)
	}
	// sizeBuf guarantees that the rows batch will be either fully
	// read or fully discarded on the vmstorage side.
	// sizeBuf is used for read optimization in vmstorage.
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)
	sizeBuf.B = encoding.MarshalUint64(sizeBuf.B[:0], uint64(len(buf)))
	if _, err := bc.Write(sizeBuf.B); err != nil {
		return fmt.Errorf("cannot write data size %d: %w", len(buf), err)
	}
	if _, err := bc.Write(buf); err != nil {
		return fmt.Errorf("cannot write data with size %d: %w", len(buf), err)
	}
	if err := bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush data with size %d: %w", len(buf), err)
	}

	// Wait for `ack` from vmstorage.
	// This guarantees that the message has been fully received by vmstorage.
	deadline = time.Now().Add(timeout)
	if err := bc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set read deadline for reading `ack` to vmstorage: %w", err)
	}
	if _, err := io.ReadFull(bc, sizeBuf.B[:1]); err != nil {
		return fmt.Errorf("cannot read `ack` from vmstorage: %w", err)
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
		return nil, fmt.Errorf("handshake error: %w", err)
	}
	return bc, nil
}

func rerouteWorker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var br bufRows
	brLastResetTime := fasttime.UnixTimestamp()
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
		currentTime := fasttime.UnixTimestamp()
		if len(br.buf) < cap(br.buf)/4 && currentTime-brLastResetTime > 10 {
			// Free up capacity space occupied by br.buf in order to reduce memory usage after spikes.
			br.buf = append(br.buf[:0:0], br.buf...)
			brLastResetTime = currentTime
		}
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
	// Notify all the blocked addToReroutedBufMayBlock callers, so they may finish the work.
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
	// It must be accessed under brLock.
	br bufRows

	// bcLock protects bc.
	bcLock sync.Mutex

	// bc is a single connection to vmstorage for data transfer.
	// It must be accessed under bcLock.
	bc *handshake.BufferedConn

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

	storageNodes = storageNodes[:0]
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

	for idx, sn := range storageNodes {
		storageNodesWG.Add(1)
		go func(sn *storageNode, idx int) {
			sn.run(storageNodesStopCh, idx)
			storageNodesWG.Done()
		}(sn, idx)
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

// addToReroutedBufMayBlock adds buf to reroutedBR.
//
// It waits until the reroutedBR has enough space for buf or if Stop is called.
// This guarantees backpressure if the ingestion rate exceeds vmstorage nodes'
// ingestion rate capacity.
//
// It returns non-nil error only in the following cases:
//
//   - if all the storage nodes are unhealthy.
//   - if Stop is called.
func addToReroutedBufMayBlock(buf []byte, rows int) error {
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
	defer reroutedRowsProcessed.Add(rowsProcessed)

	src := br.buf
	dst := br.buf[:0]
	dstRows := 0
	for len(src) > 0 {
		tail, err := mr.Unmarshal(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal MetricRow from reroutedBR.buf: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail
		rowsProcessed++

		idx := uint64(0)
		if len(sns) > 1 {
			h := xxhash.Sum64(mr.MetricNameRaw)
			// Do not use jump.Hash(h, int32(len(sns))) here,
			// since this leads to uneven distribution of rerouted rows among sns -
			// they all go to the original or to the next sn.
			idx = h % uint64(len(sns))
		}
		attempts := 0
		for {
			sn := sns[idx]
			idx++
			if idx >= uint64(len(sns)) {
				idx = 0
			}
			attempts++
			if attempts > len(sns) {
				// All the storage nodes are broken.
				// Return the remaining data to br.buf, so it may be processed later.
				dst = append(dst, rowBuf...)
				dst = append(dst, src...)
				br.buf = dst
				br.rows = dstRows + (br.rows - rowsProcessed + 1)
				return
			}
			if sn.isBroken() {
				// The sn is broken. Go to the next one.
				continue
			}
			if !sn.sendReroutedRow(rowBuf) {
				// The row cannot be re-routed to sn. Return it back to the buf for rerouting.
				// Do not re-route the row to the remaining storage nodes,
				// since this may result in increased resource usage (CPU, memory, disk IO) on these nodes,
				// because they'll have to accept and register new time series (this is resource-intensive operation).
				dst = append(dst, rowBuf...)
				dstRows++
			}
			break
		}
	}
	br.buf = dst
	br.rows = dstRows
}

func (sn *storageNode) sendReroutedRow(buf []byte) bool {
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

	rerouteErrors                   = metrics.NewCounter(`vm_rpc_reroute_errors_total{name="vminsert"}`)
	rowsLostTotal                   = metrics.NewCounter(`vm_rpc_rows_lost_total{name="vminsert"}`)
	rowsIncompletelyReplicatedTotal = metrics.NewCounter(`vm_rpc_rows_incompletely_replicated_total{name="vminsert"}`)
)
