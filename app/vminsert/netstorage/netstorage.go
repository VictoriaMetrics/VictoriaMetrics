package netstorage

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

var (
	disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Whether to disable compression for the data sent from vminsert to vmstorage. This reduces CPU usage at the cost of higher network bandwidth usage")
	replicationFactor     = flag.Int("replicationFactor", 1, "Replication factor for the ingested data, i.e. how many copies to make among distinct -storageNode instances. "+
		"Note that vmselect must run with -dedup.minScrapeInterval=1ms for data de-duplication when replicationFactor is greater than 1. "+
		"Higher values for -dedup.minScrapeInterval at vmselect is OK")
	disableRerouting      = flag.Bool("disableRerouting", true, "Whether to disable re-routing when some of vmstorage nodes accept incoming data at slower speed compared to other storage nodes. Disabled re-routing limits the ingestion rate by the slowest vmstorage node. On the other side, disabled re-routing minimizes the number of active time series in the cluster during rolling restarts and during spikes in series churn rate. See also -dropSamplesOnOverload")
	dropSamplesOnOverload = flag.Bool("dropSamplesOnOverload", false, "Whether to drop incoming samples if the destination vmstorage node is overloaded and/or unavailable. This prioritizes cluster availability over consistency, e.g. the cluster continues accepting all the ingested samples, but some of them may be dropped if vmstorage nodes are temporarily unavailable and/or overloaded")
)

var errStorageReadOnly = errors.New("storage node is read only")

func (sn *storageNode) isReady() bool {
	return atomic.LoadUint32(&sn.broken) == 0 && atomic.LoadUint32(&sn.isReadOnly) == 0
}

// push pushes buf to sn internal bufs.
//
// This function doesn't block on fast path.
// It may block only if storageNodes cannot handle the incoming ingestion rate.
// This blocking provides backpressure to the caller.
//
// The function falls back to sending data to other vmstorage nodes
// if sn is currently unavailable or overloaded.
//
// rows must match the number of rows in the buf.
func (sn *storageNode) push(buf []byte, rows int) error {
	if len(buf) > maxBufSizePerStorageNode {
		logger.Panicf("BUG: len(buf)=%d cannot exceed %d", len(buf), maxBufSizePerStorageNode)
	}
	sn.rowsPushed.Add(rows)
	if sn.trySendBuf(buf, rows) {
		// Fast path - the buffer is successfully sent to sn.
		return nil
	}
	if *dropSamplesOnOverload && atomic.LoadUint32(&sn.isReadOnly) == 0 {
		sn.rowsDroppedOnOverload.Add(rows)
		logger.WithThrottler("droppedSamplesOnOverload", 5*time.Second).Warnf(
			"some rows dropped, because -dropSamplesOnOverload is set and vmstorage %s cannot accept new rows now. "+
				"See vm_rpc_rows_dropped_on_overload_total metric at /metrics page", sn.dialer.Addr())
		return nil
	}
	// Slow path - sn cannot accept buf now, so re-route it to other vmstorage nodes.
	if err := sn.rerouteBufToOtherStorageNodes(buf, rows); err != nil {
		return fmt.Errorf("error when re-routing rows from %s: %w", sn.dialer.Addr(), err)
	}
	return nil
}

func (sn *storageNode) rerouteBufToOtherStorageNodes(buf []byte, rows int) error {
	sn.brLock.Lock()
again:
	select {
	case <-storageNodesStopCh:
		sn.brLock.Unlock()
		return fmt.Errorf("cannot send %d rows because of graceful shutdown", rows)
	default:
	}
	if !sn.isReady() {
		if len(storageNodes) == 1 {
			// There are no other storage nodes to re-route to. So wait until the current node becomes healthy.
			sn.brCond.Wait()
			goto again
		}
		sn.brLock.Unlock()
		// The vmstorage node isn't ready for data processing. Re-route buf to healthy vmstorage nodes even if disableRerouting is set.
		rowsProcessed, err := rerouteRowsToReadyStorageNodes(sn, buf)
		rows -= rowsProcessed
		if err != nil {
			return fmt.Errorf("%d rows dropped because the current vsmtorage is unavailable and %w", rows, err)
		}
		return nil
	}
	if len(sn.br.buf)+len(buf) <= maxBufSizePerStorageNode {
		// Fast path: the buf contents fits sn.buf.
		sn.br.buf = append(sn.br.buf, buf...)
		sn.br.rows += rows
		sn.brLock.Unlock()
		return nil
	}
	// Slow path: the buf contents doesn't fit sn.buf, so try re-routing it to other vmstorage nodes.
	if *disableRerouting || len(storageNodes) == 1 {
		sn.brCond.Wait()
		goto again
	}
	sn.brLock.Unlock()
	rowsProcessed, err := rerouteRowsToFreeStorageNodes(sn, buf)
	rows -= rowsProcessed
	if err != nil {
		return fmt.Errorf("%d rows dropped because the current vmstorage buf is full and %w", rows, err)
	}
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

	sn.readOnlyCheckerWG.Add(1)
	go func() {
		defer sn.readOnlyCheckerWG.Done()
		sn.readOnlyChecker(stopCh)
	}()
	defer sn.readOnlyCheckerWG.Wait()

	ticker := time.NewTicker(200 * time.Millisecond)
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
		if bufLen > 0 {
			// Do not sleep if sn.br.buf isn't empty.
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
		sn.brLock.Lock()
		sn.br, br = br, sn.br
		sn.brCond.Broadcast()
		sn.brLock.Unlock()
		currentTime := fasttime.UnixTimestamp()
		if len(br.buf) < cap(br.buf)/4 && currentTime-brLastResetTime > 10 {
			// Free up capacity space occupied by br.buf in order to reduce memory usage after spikes.
			br.buf = append(br.buf[:0:0], br.buf...)
			brLastResetTime = currentTime
		}
		sn.checkHealth()
		if len(br.buf) == 0 {
			// Nothing to send.
			continue
		}
		// Send br to replicas storageNodes starting from snIdx.
		for !sendBufToReplicasNonblocking(&br, snIdx, replicas) {
			t := timerpool.Get(200 * time.Millisecond)
			select {
			case <-stopCh:
				timerpool.Put(t)
				return
			case <-t.C:
				timerpool.Put(t)
				sn.checkHealth()
			}
		}
		br.reset()
	}
}

func sendBufToReplicasNonblocking(br *bufRows, snIdx, replicas int) bool {
	usedStorageNodes := make(map[*storageNode]struct{}, replicas)
	for i := 0; i < replicas; i++ {
		idx := snIdx + i
		attempts := 0
		for {
			attempts++
			if attempts > len(storageNodes) {
				if i == 0 {
					// The data wasn't replicated at all.
					logger.WithThrottler("cannotReplicateDataBecauseNoStorageNodes", 5*time.Second).Warnf(
						"cannot push %d bytes with %d rows to storage nodes, since all the nodes are temporarily unavailable; "+
							"re-trying to send the data soon", len(br.buf), br.rows)
					return false
				}
				// The data is partially replicated, so just emit a warning and return true.
				// We could retry sending the data again, but this may result in uncontrolled duplicate data.
				// So it is better returning true.
				rowsIncompletelyReplicatedTotal.Add(br.rows)
				logger.WithThrottler("incompleteReplication", 5*time.Second).Warnf(
					"cannot make a copy #%d out of %d copies according to -replicationFactor=%d for %d bytes with %d rows, "+
						"since a part of storage nodes is temporarily unavailable", i+1, replicas, *replicationFactor, len(br.buf), br.rows)
				return true
			}
			if idx >= len(storageNodes) {
				idx %= len(storageNodes)
			}
			sn := storageNodes[idx]
			idx++
			if _, ok := usedStorageNodes[sn]; ok {
				// The br has been already replicated to sn. Skip it.
				continue
			}
			if !sn.sendBufRowsNonblocking(br) {
				// Cannot send data to sn. Go to the next sn.
				continue
			}
			// Successfully sent data to sn.
			usedStorageNodes[sn] = struct{}{}
			break
		}
	}
	return true
}

func (sn *storageNode) checkHealth() {
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()

	if sn.bc != nil {
		// The sn looks healthy.
		return
	}
	bc, err := sn.dial()
	if err != nil {
		atomic.StoreUint32(&sn.broken, 1)
		sn.brCond.Broadcast()
		if sn.lastDialErr == nil {
			// Log the error only once.
			sn.lastDialErr = err
			logger.Warnf("cannot dial storageNode %q: %s", sn.dialer.Addr(), err)
		}
		return
	}
	logger.Infof("successfully dialed -storageNode=%q", sn.dialer.Addr())
	sn.lastDialErr = nil
	sn.bc = bc
	atomic.StoreUint32(&sn.broken, 0)
	sn.brCond.Broadcast()
}

func (sn *storageNode) sendBufRowsNonblocking(br *bufRows) bool {
	if !sn.isReady() {
		return false
	}
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()

	if sn.bc == nil {
		// Do not call sn.dial() here in order to prevent long blocking on sn.bcLock.Lock(),
		// which can negatively impact data sending in sendBufToReplicasNonblocking().
		// sn.dial() should be called by sn.checkHealth() on unsuccessful call to sendBufToReplicasNonblocking().
		return false
	}
	startTime := time.Now()
	err := sendToConn(sn.bc, br.buf)
	duration := time.Since(startTime)
	sn.sendDurationSeconds.Add(duration.Seconds())
	if err == nil {
		// Successfully sent buf to bc.
		sn.rowsSent.Add(br.rows)
		return true
	}
	if errors.Is(err, errStorageReadOnly) {
		// The vmstorage is transitioned to readonly mode.
		atomic.StoreUint32(&sn.isReadOnly, 1)
		sn.brCond.Broadcast()
		// Signal the caller that the data wasn't accepted by the vmstorage,
		// so it will be re-routed to the remaining vmstorage nodes.
		return false
	}
	// Couldn't flush buf to sn. Mark sn as broken.
	logger.WithThrottler("cannotSendBufRows", 5*time.Second).Warnf(
		"cannot send %d bytes with %d rows to -storageNode=%q: %s; closing the connection to storageNode and "+
			"re-routing this data to healthy storage nodes", len(br.buf), br.rows, sn.dialer.Addr(), err)
	if err = sn.bc.Close(); err != nil {
		logger.WithThrottler("cannotCloseStorageNodeConn", 5*time.Second).Warnf("cannot close connection to storageNode %q: %s", sn.dialer.Addr(), err)
	}
	sn.bc = nil
	atomic.StoreUint32(&sn.broken, 1)
	sn.brCond.Broadcast()
	sn.connectionErrors.Inc()
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

	ackResp := sizeBuf.B[0]
	switch ackResp {
	case 1:
		// ok response, data successfully accepted by vmstorage
	case 2:
		// vmstorage is in readonly mode
		return errStorageReadOnly
	default:
		return fmt.Errorf("unexpected `ack` received from vmstorage; got %d; want 1 or 2", sizeBuf.B[0])
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

// storageNode is a client sending data to vmstorage node.
type storageNode struct {
	// broken is set to non-zero if the given vmstorage node is temporarily unhealthy.
	// In this case the data is re-routed to the remaining healthy vmstorage nodes.
	broken uint32

	// isReadOnly is set to non-zero if the given vmstorage node is read only
	// In this case the data is re-routed to the remaining healthy vmstorage nodes.
	isReadOnly uint32

	// brLock protects br.
	brLock sync.Mutex

	// brCond is used for waiting for free space in br.
	brCond *sync.Cond

	// Buffer with data that needs to be written to the storage node.
	// It must be accessed under brLock.
	br bufRows

	// bcLock protects bc.
	bcLock sync.Mutex

	// waitGroup for readOnlyChecker
	readOnlyCheckerWG sync.WaitGroup

	// bc is a single connection to vmstorage for data transfer.
	// It must be accessed under bcLock.
	bc *handshake.BufferedConn

	dialer *netutil.TCPDialer

	// last error during dial.
	lastDialErr error

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

	// The number of rows dropped on overload if -dropSamplesOnOverload is set.
	rowsDroppedOnOverload *metrics.Counter

	// The number of rows rerouted from the given vmstorage node
	// to healthy nodes when the given node was unhealthy.
	rowsReroutedFromHere *metrics.Counter

	// The number of rows rerouted to the given vmstorage node
	// from other nodes when they were unhealthy.
	rowsReroutedToHere *metrics.Counter

	// The total duration spent for sending data to vmstorage node.
	// This metric is useful for determining the saturation of vminsert->vmstorage link.
	sendDurationSeconds *metrics.FloatCounter
}

// storageNodes contains a list of vmstorage node clients.
var storageNodes []*storageNode

var storageNodesWG sync.WaitGroup

var storageNodesStopCh = make(chan struct{})

// nodesHash is used for consistently selecting a storage node by key.
var nodesHash *consistentHash

// InitStorageNodes initializes vmstorage nodes' connections to the given addrs.
//
// hashSeed is used for changing the distribution of input time series among addrs.
func InitStorageNodes(addrs []string, hashSeed uint64) {
	if len(addrs) == 0 {
		logger.Panicf("BUG: addrs must be non-empty")
	}
	nodesHash = newConsistentHash(addrs, hashSeed)
	storageNodes = storageNodes[:0]
	for _, addr := range addrs {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			// Automatically add missing port.
			addr += ":8400"
		}
		sn := &storageNode{
			dialer: netutil.NewTCPDialer("vminsert", addr),

			dialErrors:            metrics.NewCounter(fmt.Sprintf(`vm_rpc_dial_errors_total{name="vminsert", addr=%q}`, addr)),
			handshakeErrors:       metrics.NewCounter(fmt.Sprintf(`vm_rpc_handshake_errors_total{name="vminsert", addr=%q}`, addr)),
			connectionErrors:      metrics.NewCounter(fmt.Sprintf(`vm_rpc_connection_errors_total{name="vminsert", addr=%q}`, addr)),
			rowsPushed:            metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_pushed_total{name="vminsert", addr=%q}`, addr)),
			rowsSent:              metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_sent_total{name="vminsert", addr=%q}`, addr)),
			rowsDroppedOnOverload: metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_dropped_on_overload_total{name="vminsert", addr=%q}`, addr)),
			rowsReroutedFromHere:  metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_rerouted_from_here_total{name="vminsert", addr=%q}`, addr)),
			rowsReroutedToHere:    metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_rerouted_to_here_total{name="vminsert", addr=%q}`, addr)),
			sendDurationSeconds:   metrics.NewFloatCounter(fmt.Sprintf(`vm_rpc_send_duration_seconds_total{name="vminsert", addr=%q}`, addr)),
		}
		sn.brCond = sync.NewCond(&sn.brLock)
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
		_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_vmstorage_is_reachable{name="vminsert", addr=%q}`, addr), func() float64 {
			if atomic.LoadUint32(&sn.broken) != 0 {
				return 0
			}
			return 1
		})
		_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_vmstorage_is_read_only{name="vminsert", addr=%q}`, addr), func() float64 {
			return float64(atomic.LoadUint32(&sn.isReadOnly))
		})
		storageNodes = append(storageNodes, sn)
	}

	maxBufSizePerStorageNode = memory.Allowed() / 8 / len(storageNodes)
	if maxBufSizePerStorageNode > consts.MaxInsertPacketSizeForVMInsert {
		maxBufSizePerStorageNode = consts.MaxInsertPacketSizeForVMInsert
	}

	for idx, sn := range storageNodes {
		storageNodesWG.Add(1)
		go func(sn *storageNode, idx int) {
			sn.run(storageNodesStopCh, idx)
			storageNodesWG.Done()
		}(sn, idx)
	}
}

// Stop gracefully stops netstorage.
func Stop() {
	close(storageNodesStopCh)
	for _, sn := range storageNodes {
		sn.brCond.Broadcast()
	}
	storageNodesWG.Wait()
}

// rerouteRowsToReadyStorageNodes reroutes src from not ready snSource to ready storage nodes.
//
// The function blocks until src is fully re-routed.
func rerouteRowsToReadyStorageNodes(snSource *storageNode, src []byte) (int, error) {
	reroutesTotal.Inc()
	rowsProcessed := 0
	var idxsExclude, idxsExcludeNew []int
	idxsExclude = getNotReadyStorageNodeIdxsBlocking(idxsExclude[:0], nil)
	var mr storage.MetricRow
	for len(src) > 0 {
		tail, err := mr.UnmarshalX(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal MetricRow: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail
		reroutedRowsProcessed.Inc()
		h := xxhash.Sum64(mr.MetricNameRaw)
		mr.ResetX()
		var sn *storageNode
		for {
			idx := nodesHash.getNodeIdx(h, idxsExclude)
			sn = storageNodes[idx]
			if sn.isReady() {
				break
			}
			// re-generate idxsExclude list, since sn must be put there.
			idxsExclude = getNotReadyStorageNodeIdxsBlocking(idxsExclude[:0], nil)
		}
		if *disableRerouting {
			if !sn.sendBufMayBlock(rowBuf) {
				return rowsProcessed, fmt.Errorf("graceful shutdown started")
			}
			rowsProcessed++
			if sn != snSource {
				snSource.rowsReroutedFromHere.Inc()
				sn.rowsReroutedToHere.Inc()
			}
			continue
		}
	again:
		if sn.trySendBuf(rowBuf, 1) {
			rowsProcessed++
			if sn != snSource {
				snSource.rowsReroutedFromHere.Inc()
				sn.rowsReroutedToHere.Inc()
			}
			continue
		}
		// If the re-routing is enabled, then try sending the row to another storage node.
		idxsExcludeNew = getNotReadyStorageNodeIdxs(idxsExcludeNew[:0], sn)
		idx := nodesHash.getNodeIdx(h, idxsExcludeNew)
		snNew := storageNodes[idx]
		if snNew.trySendBuf(rowBuf, 1) {
			rowsProcessed++
			if snNew != snSource {
				snSource.rowsReroutedFromHere.Inc()
				snNew.rowsReroutedToHere.Inc()
			}
			continue
		}
		// The row cannot be sent to both snSource and the re-routed sn without blocking.
		// Sleep for a while and try sending the row to snSource again.
		time.Sleep(100 * time.Millisecond)
		goto again
	}
	return rowsProcessed, nil
}

// reouteRowsToFreeStorageNodes re-routes src from snSource to other storage nodes.
//
// It is expected that snSource has no enough buffer for sending src.
// It is expected than *dsableRerouting isn't set when calling this function.
func rerouteRowsToFreeStorageNodes(snSource *storageNode, src []byte) (int, error) {
	if *disableRerouting {
		logger.Panicf("BUG: disableRerouting must be disabled when calling rerouteRowsToFreeStorageNodes")
	}
	reroutesTotal.Inc()
	rowsProcessed := 0
	var idxsExclude []int
	idxsExclude = getNotReadyStorageNodeIdxs(idxsExclude[:0], snSource)
	var mr storage.MetricRow
	for len(src) > 0 {
		tail, err := mr.UnmarshalX(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal MetricRow: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail
		reroutedRowsProcessed.Inc()
		h := xxhash.Sum64(mr.MetricNameRaw)
		mr.ResetX()
		// Try sending the row to snSource in order to minimize re-routing.
	again:
		if snSource.trySendBuf(rowBuf, 1) {
			rowsProcessed++
			continue
		}
		// The row couldn't be sent to snSrouce. Try re-routing it to other nodes.
		var sn *storageNode
		for {
			idx := nodesHash.getNodeIdx(h, idxsExclude)
			sn = storageNodes[idx]
			if sn.isReady() {
				break
			}
			// re-generate idxsExclude list, since sn must be put there.
			idxsExclude = getNotReadyStorageNodeIdxs(idxsExclude[:0], snSource)
		}
		if sn.trySendBuf(rowBuf, 1) {
			rowsProcessed++
			snSource.rowsReroutedFromHere.Inc()
			sn.rowsReroutedToHere.Inc()
			continue
		}
		// The row cannot be sent to both snSource and the re-routed sn without blocking.
		// Sleep for a while and try sending the row to snSource again.
		time.Sleep(100 * time.Millisecond)
		goto again
	}
	return rowsProcessed, nil
}

func getNotReadyStorageNodeIdxsBlocking(dst []int, snExtra *storageNode) []int {
	dst = getNotReadyStorageNodeIdxs(dst[:0], snExtra)
	if len(dst) < len(storageNodes) {
		return dst
	}
	logger.WithThrottler("storageNodesUnavailable", 5*time.Second).Warnf(
		"all the vmstorage nodes are unavailable; stopping data processing util at least a single node becomes available")
	for {
		time.Sleep(time.Second)
		dst = getNotReadyStorageNodeIdxs(dst[:0], snExtra)
		if availableNodes := len(storageNodes) - len(dst); availableNodes > 0 {
			logger.WithThrottler("storageNodesBecameAvailable", 5*time.Second).Warnf("%d vmstorage nodes became available, so continue data processing", availableNodes)
			return dst
		}
	}
}

func getNotReadyStorageNodeIdxs(dst []int, snExtra *storageNode) []int {
	dst = dst[:0]
	for i, sn := range storageNodes {
		if sn == snExtra || !sn.isReady() {
			dst = append(dst, i)
		}
	}
	return dst
}

func (sn *storageNode) trySendBuf(buf []byte, rows int) bool {
	sent := false
	sn.brLock.Lock()
	if sn.isReady() && len(sn.br.buf)+len(buf) <= maxBufSizePerStorageNode {
		sn.br.buf = append(sn.br.buf, buf...)
		sn.br.rows += rows
		sent = true
	}
	sn.brLock.Unlock()
	return sent
}

func (sn *storageNode) sendBufMayBlock(buf []byte) bool {
	sn.brLock.Lock()
	for len(sn.br.buf)+len(buf) > maxBufSizePerStorageNode {
		select {
		case <-storageNodesStopCh:
			sn.brLock.Unlock()
			return false
		default:
		}
		sn.brCond.Wait()
	}
	sn.br.buf = append(sn.br.buf, buf...)
	sn.br.rows++
	sn.brLock.Unlock()
	return true
}

func (sn *storageNode) readOnlyChecker(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			sn.checkReadOnlyMode()
		}
	}
}

func (sn *storageNode) checkReadOnlyMode() {
	if atomic.LoadUint32(&sn.isReadOnly) == 0 {
		// fast path - the sn isn't in readonly mode
		return
	}
	// Check whether the storage remains in readonly mode
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()
	if sn.bc == nil {
		return
	}
	// send nil buff to check ack response from storage
	err := sendToConn(sn.bc, nil)
	if err == nil {
		// The storage switched from readonly to non-readonly mode
		atomic.StoreUint32(&sn.isReadOnly, 0)
		return
	}
	if !errors.Is(err, errStorageReadOnly) {
		logger.Errorf("cannot check storage readonly mode for -storageNode=%q: %s", sn.dialer.Addr(), err)
	}
}

var (
	maxBufSizePerStorageNode int

	reroutedRowsProcessed           = metrics.NewCounter(`vm_rpc_rerouted_rows_processed_total{name="vminsert"}`)
	reroutesTotal                   = metrics.NewCounter(`vm_rpc_reroutes_total{name="vminsert"}`)
	rowsIncompletelyReplicatedTotal = metrics.NewCounter(`vm_rpc_rows_incompletely_replicated_total{name="vminsert"}`)
)
