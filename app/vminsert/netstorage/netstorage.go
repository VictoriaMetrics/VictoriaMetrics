package netstorage

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consistenthash"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
)

var (
	disableCompression = flag.Bool("rpc.disableCompression", false, "Flag is deprecated and kept for backward compatibility, vminsert performs per block compression instead of streaming compression on RPC connection")
	replicationFactor  = flag.Int("replicationFactor", 1, "Replication factor for the ingested data, i.e. how many copies to make among distinct -storageNode instances. "+
		"Note that vmselect must run with -dedup.minScrapeInterval=1ms for data de-duplication when replicationFactor is greater than 1. "+
		"Higher values for -dedup.minScrapeInterval at vmselect is OK")
	disableRerouting      = flag.Bool("disableRerouting", true, "Whether to disable re-routing when some of vmstorage nodes accept incoming data at slower speed compared to other storage nodes. Disabled re-routing limits the ingestion rate by the slowest vmstorage node. On the other side, disabled re-routing minimizes the number of active time series in the cluster during rolling restarts and during spikes in series churn rate. See also -disableReroutingOnUnavailable and -dropSamplesOnOverload")
	dropSamplesOnOverload = flag.Bool("dropSamplesOnOverload", false, "Whether to drop incoming samples if the destination vmstorage node is overloaded and/or unavailable. This prioritizes cluster availability over consistency, e.g. the cluster continues accepting all the ingested samples, but some of them may be dropped if vmstorage nodes are temporarily unavailable and/or overloaded. The drop of samples happens before the replication, so it's not recommended to use this flag with -replicationFactor enabled.")
	vmstorageDialTimeout  = flag.Duration("vmstorageDialTimeout", 3*time.Second, "Timeout for establishing RPC connections from vminsert to vmstorage. "+
		"See also -vmstorageUserTimeout")
	vmstorageUserTimeout = flag.Duration("vmstorageUserTimeout", 3*time.Second, "Network timeout for RPC connections from vminsert to vmstorage (Linux only). "+
		"Lower values speed up re-rerouting recovery when some of vmstorage nodes become unavailable because of networking issues. "+
		"Read more about TCP_USER_TIMEOUT at https://blog.cloudflare.com/when-tcp-sockets-refuse-to-die/ . "+
		"See also -vmstorageDialTimeout")
	disableReroutingOnUnavailable = flag.Bool("disableReroutingOnUnavailable", false, "Whether to disable re-routing when some of vmstorage nodes are unavailable. "+
		"Disabled re-routing stops ingestion when some storage nodes are unavailable. "+
		"On the other side, disabled re-routing minimizes the number of active time series in the cluster "+
		"during rolling restarts and during spikes in series churn rate. "+
		"See also -disableRerouting")
)

const unsupportedRPCRetrySeconds = 120

func (sn *storageNode) isReady() bool {
	return !sn.isBroken.Load() && !sn.isReadOnly.Load()
}

// push pushes buf to sn internal bufs.
//
// This function doesn't block on fast path.
// It may block only if storage nodes cannot handle the incoming ingestion rate.
// This blocking provides backpressure to the caller.
//
// The function falls back to sending data to other vmstorage nodes
// if sn is currently unavailable or overloaded.
//
// rows must match the number of rows in the buf.
func (sn *storageNode) push(snb *storageNodesBucket, buf []byte, rows int, getRowHasher func() rowHasher) error {
	if len(buf) > maxBufSizePerStorageNode {
		logger.Panicf("BUG: len(buf)=%d cannot exceed %d", len(buf), maxBufSizePerStorageNode)
	}
	sn.rowsPushed.Add(rows)
	if sn.trySendBuf(buf, rows) {
		// Fast path - the buffer is successfully sent to sn.
		return nil
	}
	if sn.dropRowsOnOverload && !sn.isReadOnly.Load() {
		sn.rowsDroppedOnOverload.Add(rows)
		dropSamplesOnOverloadLogger.Warnf("some rows are dropped, because -dropSamplesOnOverload is set and vmstorage %s cannot accept new rows now. "+
			"See vm_rpc_rows_dropped_on_overload_total metric at /metrics page", sn.dialer.Addr())
		return nil
	}
	// Slow path - sn cannot accept buf now, so re-route it to other vmstorage nodes.
	if err := sn.rerouteBufToOtherStorageNodes(snb, buf, rows, getRowHasher); err != nil {
		return fmt.Errorf("error when re-routing rows from %s: %w", sn.dialer.Addr(), err)
	}
	return nil
}

var dropSamplesOnOverloadLogger = logger.WithThrottler("droppedSamplesOnOverload", 5*time.Second)

func (sn *storageNode) rerouteBufToOtherStorageNodes(snb *storageNodesBucket, buf []byte, rows int, getRowHasher func() rowHasher) error {
	sns := snb.sns
	sn.brLock.Lock()
again:
	select {
	case <-sn.stopCh:
		sn.brLock.Unlock()
		return fmt.Errorf("cannot send %d rows because of graceful shutdown", rows)
	default:
	}

	if !sn.isReady() {
		if len(sns) == 1 {
			// There are no other storage nodes to re-route to. So wait until the current node becomes healthy.
			sn.brCond.Wait()
			goto again
		}
		if *disableReroutingOnUnavailable {
			// We should not send rows from currently unavailable storage to alive storage nodes.
			sn.brCond.Wait()
			goto again
		}
		sn.brLock.Unlock()

		// The vmstorage node isn't ready for data processing. Re-route buf to healthy vmstorage nodes even if disableRerouting is set.
		rowsProcessed, err := rerouteRowsToReadyStorageNodes(snb, sn, buf, getRowHasher)
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
	if !allowRerouting(sn, sns) {
		sn.brCond.Wait()
		goto again
	}

	sn.brLock.Unlock()
	rowsProcessed, err := rerouteRowsToFreeStorageNodes(snb, sn, buf, getRowHasher)
	rows -= rowsProcessed
	if err != nil {
		return fmt.Errorf("%d rows dropped because the current vmstorage buf is full and %w", rows, err)
	}
	return nil
}

func (sn *storageNode) run(snb *storageNodesBucket, snIdx int) {
	replicas := *replicationFactor
	if replicas <= 0 {
		replicas = 1
	}
	sns := snb.sns
	if replicas > len(sns) {
		replicas = len(sns)
	}

	sn.readOnlyCheckerWG.Go(sn.readOnlyChecker)
	defer sn.readOnlyCheckerWG.Wait()

	d := timeutil.AddJitterToDuration(time.Millisecond * 200)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	var br bufRows
	brLastResetTime := fasttime.UnixTimestamp()
	mustStop := false
	for !mustStop {
		sn.brLock.Lock()
		waitForNewData := len(sn.br.buf) == 0
		sn.brLock.Unlock()
		if waitForNewData {
			select {
			case <-sn.stopCh:
				mustStop = true
				// Make sure the br bufs are flushed last time before returning
				// in order to send the remaining bits of data.
			case <-ticker.C:
			}
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
		// Send br to replicas storage nodes starting from snIdx.
		for !sendBufToReplicasNonblocking(snb, &br, snIdx, replicas) {
			d := timeutil.AddJitterToDuration(time.Millisecond * 200)
			t := timerpool.Get(d)
			select {
			case <-sn.stopCh:
				timerpool.Put(t)
				logger.Errorf("dropping %d rows on graceful shutdown, since all the vmstorage nodes are unavailable", br.rows)
				return
			case <-t.C:
				timerpool.Put(t)
				sn.checkHealth()
			}
		}
		br.reset()
	}
}

func sendBufToReplicasNonblocking(snb *storageNodesBucket, br *bufRows, snIdx, replicas int) bool {
	usedStorageNodes := make(map[*storageNode]struct{}, replicas)
	sns := snb.sns
	for i := range replicas {
		idx := snIdx + i
		attempts := 0
		for {
			attempts++
			if attempts > len(sns) {
				if i == 0 {
					// The data wasn't replicated at all.
					cannotReplicateLogger.Warnf("cannot push %d bytes with %d rows to storage nodes, since all the nodes are temporarily unavailable; "+
						"re-trying to send the data soon", len(br.buf), br.rows)
					return false
				}
				// The data is partially replicated, so just emit a warning and return true.
				// We could retry sending the data again, but this may result in uncontrolled duplicate data.
				// So it is better returning true.
				rowsIncompletelyReplicatedTotal.Add(br.rows)
				incompleteReplicationLogger.Warnf("cannot make a copy #%d out of %d copies according to -replicationFactor=%d for %d bytes with %d rows, "+
					"since a part of storage nodes is temporarily unavailable", i+1, replicas, *replicationFactor, len(br.buf), br.rows)
				return true
			}
			if idx >= len(sns) {
				idx %= len(sns)
			}
			sn := sns[idx]
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

var (
	cannotReplicateLogger       = logger.WithThrottler("cannotReplicateDataBecauseNoStorageNodes", 5*time.Second)
	incompleteReplicationLogger = logger.WithThrottler("incompleteReplication", 5*time.Second)
)

func (sn *storageNode) checkHealth() {
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()

	if sn.bc != nil {
		// The sn looks healthy.
		return
	}
	if deadline := sn.rpcIsNotSupportedDeadline.Load(); deadline > 0 {
		if deadline > fasttime.UnixTimestamp() {
			// do not attemp to re-connect
			return
		}
	}
	bc, err := sn.dial()
	if err != nil {
		sn.isBroken.Store(true)
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
	sn.isBroken.Store(false)
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
	var err error
	if sn.bc.IsLegacy {
		err = vminsertapi.SendToConn(sn.bc, br.buf)
	} else {
		err = vminsertapi.SendRPCRequestToConn(sn.bc, sn.rpcCall.VersionedName, br.buf)
	}
	duration := time.Since(startTime)
	sn.sendDurationSeconds.Add(duration.Seconds())

	now := time.Now()
	saturation := float64(now.Sub(startTime)) / float64(now.Sub(sn.lastSendTime))
	sn.avgSaturation.Add(saturation)
	sn.lastSendTime = now

	if err == nil {
		if deadline := sn.rpcIsNotSupportedDeadline.Load(); deadline > 0 {
			sn.rpcIsNotSupportedDeadline.Store(0)
		}
		// Successfully sent buf to bc.
		sn.rowsSent.Add(br.rows)
		return true
	}
	if errors.Is(err, storage.ErrReadOnly) {
		// The vmstorage is transitioned to readonly mode.
		sn.isReadOnly.Store(true)
		sn.brCond.Broadcast()
		// Signal the caller that the data wasn't accepted by the vmstorage,
		// so it will be re-routed to the remaining vmstorage nodes.
		return false
	}
	if errors.Is(err, vminsertapi.ErrRpcIsNotSupported) {
		sn.rpcIsNotSupportedDeadline.Store(unsupportedRPCRetrySeconds + fasttime.UnixTimestamp())
	}
	// Couldn't flush buf to sn. Mark sn as broken.
	cannotSendBufsLogger.Warnf("cannot send %d bytes with %d rows to -storageNode=%q: %s; closing the connection to storageNode and "+
		"re-routing this data to healthy storage nodes", len(br.buf), br.rows, sn.dialer.Addr(), err)
	if err = sn.bc.Close(); err != nil {
		cannotCloseStorageNodeConnLogger.Warnf("cannot close connection to storageNode %q: %s", sn.dialer.Addr(), err)
	}
	sn.bc = nil
	sn.isBroken.Store(true)
	sn.brCond.Broadcast()
	sn.connectionErrors.Inc()
	return false
}

var cannotCloseStorageNodeConnLogger = logger.WithThrottler("cannotCloseStorageNodeConn", 5*time.Second)

var cannotSendBufsLogger = logger.WithThrottler("cannotSendBufRows", 5*time.Second)

func (sn *storageNode) dial() (*handshake.BufferedConn, error) {

	compression := 1
	if *disableCompression {
		compression = 0
	}
	var dialError error
	bc, err := handshake.VMInsertClientWithDialer(func() (net.Conn, error) {
		c, err := sn.dialer.Dial()
		if err != nil {
			dialError = err
			sn.dialErrors.Inc()
			return nil, err
		}
		return c, nil
	}, compression)
	if err != nil {
		if dialError != nil {
			return nil, dialError
		}
		sn.handshakeErrors.Inc()
		return nil, fmt.Errorf("handshake error: %w", err)
	}
	return bc, nil
}

// storageNode is a client sending data to vmstorage node.
type storageNode struct {

	// rpc defines RPC method to push data from br
	rpc vminsertapi.RPCCall

	// rpcIsNotSupportedDeadline defines a timeout for the next storage rpc call
	// if the given rpc version is not supported by storage server
	rpcIsNotSupportedDeadline atomic.Uint64

	// dropSamplesOnOverload defines whether to drop rows from br due to storage overload
	dropRowsOnOverload bool
	// isBroken is set to true if the given vmstorage node is temporarily unhealthy.
	// In this case the data is re-routed to the remaining healthy vmstorage nodes.
	isBroken atomic.Bool

	rpcCall vminsertapi.RPCCall

	// isReadOnly is set to true if the given vmstorage node is read only
	// In this case the data is re-routed to the remaining healthy vmstorage nodes.
	isReadOnly atomic.Bool

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

	stopCh chan struct{}

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

	// avgSaturation tracks the moving average of (send duration / (now - lastSendTime)).
	// Updated in run(). Used by allowRerouting to decide when to trigger slowness-based rerouting.
	avgSaturation *variableEWMA
	lastSendTime  time.Time
}

type storageNodesBucket struct {
	ms *metrics.Set

	// nodesHash is used for consistently selecting a storage node by key.
	nodesHash *consistenthash.ConsistentHash

	// sns is a list of storage nodes.
	sns []*storageNode

	stopCh chan struct{}
	wg     *sync.WaitGroup
}

// storageNodes and metadataStorageNodes contains a list of vmstorage node clients.
var (
	storageNodes         atomic.Pointer[storageNodesBucket]
	metadataStorageNodes atomic.Pointer[storageNodesBucket]
)

func getMetadataStorageNodesBucket() *storageNodesBucket {
	return metadataStorageNodes.Load()
}

func setMetadataStorageNodesBucket(snb *storageNodesBucket) {
	metadataStorageNodes.Store(snb)
}

func getStorageNodesBucket() *storageNodesBucket {
	return storageNodes.Load()
}

func setStorageNodesBucket(snb *storageNodesBucket) {
	storageNodes.Store(snb)
}

// Init initializes vmstorage nodes' connections to the given addrs.
//
// hashSeed is used for changing the distribution of input time series among addrs.
//
// Call MustStop when the initialized vmstorage connections are no longer needed.
func Init(addrs []string, hashSeed uint64) {
	snb := initStorageNodes(addrs, vminsertapi.MetricRowsRpcCall, hashSeed)
	setStorageNodesBucket(snb)
	metadataSnb := initStorageNodes(addrs, vminsertapi.MetricMetadataRpcCall, hashSeed)
	setMetadataStorageNodesBucket(metadataSnb)

}

// MustStop stops netstorage.
func MustStop() {
	snb := getStorageNodesBucket()
	mustStopStorageNodes(snb)
	metadataSnb := getMetadataStorageNodesBucket()
	mustStopStorageNodes(metadataSnb)
}

func initStorageNodes(unsortedAddrs []string, rpcCall vminsertapi.RPCCall, hashSeed uint64) *storageNodesBucket {
	if len(unsortedAddrs) == 0 {
		logger.Panicf("BUG: addrs must be non-empty")
	}

	addrs := make([]string, len(unsortedAddrs))
	copy(addrs, unsortedAddrs)
	sort.Strings(addrs)

	ms := metrics.NewSet()
	nodesHash := consistenthash.NewConsistentHash(addrs, hashSeed)
	sns := make([]*storageNode, 0, len(addrs))
	var dropRowsOnOverload bool

	if rpcCall.Name == vminsertapi.MetricRowsRpcCall.Name {
		dropRowsOnOverload = *dropSamplesOnOverload
	}
	stopCh := make(chan struct{})
	rpcName := rpcCall.Name
	for _, addr := range addrs {
		normalizedAddr, err := netutil.NormalizeAddr(addr, 8400)
		if err != nil {
			logger.Fatalf("cannot normalize -storageNode=%q: %s", addr, err)
		}
		addr = normalizedAddr

		sn := &storageNode{
			dialer:             netutil.NewTCPDialer(ms, "vminsert_"+rpcName, addr, *vmstorageDialTimeout, *vmstorageUserTimeout),
			rpc:                rpcCall,
			dropRowsOnOverload: dropRowsOnOverload,

			stopCh: stopCh,

			rpcCall: rpcCall,

			dialErrors:            ms.NewCounter(fmt.Sprintf(`vm_rpc_dial_errors_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			handshakeErrors:       ms.NewCounter(fmt.Sprintf(`vm_rpc_handshake_errors_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			connectionErrors:      ms.NewCounter(fmt.Sprintf(`vm_rpc_connection_errors_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			rowsPushed:            ms.NewCounter(fmt.Sprintf(`vm_rpc_rows_pushed_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			rowsSent:              ms.NewCounter(fmt.Sprintf(`vm_rpc_rows_sent_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			rowsDroppedOnOverload: ms.NewCounter(fmt.Sprintf(`vm_rpc_rows_dropped_on_overload_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			rowsReroutedFromHere:  ms.NewCounter(fmt.Sprintf(`vm_rpc_rows_rerouted_from_here_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			rowsReroutedToHere:    ms.NewCounter(fmt.Sprintf(`vm_rpc_rows_rerouted_to_here_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),
			sendDurationSeconds:   ms.NewFloatCounter(fmt.Sprintf(`vm_rpc_send_duration_seconds_total{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name)),

			avgSaturation: newMovingAverage(180),
			lastSendTime:  time.Now(),
		}
		sn.brCond = sync.NewCond(&sn.brLock)
		_ = ms.NewGauge(fmt.Sprintf(`vm_rpc_rows_pending{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name), func() float64 {
			sn.brLock.Lock()
			n := sn.br.rows
			sn.brLock.Unlock()
			return float64(n)
		})
		_ = ms.NewGauge(fmt.Sprintf(`vm_rpc_buf_pending_bytes{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name), func() float64 {
			sn.brLock.Lock()
			n := len(sn.br.buf)
			sn.brLock.Unlock()
			return float64(n)
		})
		// conditionally export health related metrics
		if rpcCall == vminsertapi.MetricRowsRpcCall {
			_ = ms.NewGauge(fmt.Sprintf(`vm_rpc_vmstorage_is_reachable{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name), func() float64 {
				if sn.isBroken.Load() {
					return 0
				}
				return 1
			})
			_ = ms.NewGauge(fmt.Sprintf(`vm_rpc_vmstorage_is_read_only{name="vminsert", addr=%q, rpc_call=%q}`, addr, rpcCall.Name), func() float64 {
				if sn.isReadOnly.Load() {
					return 1
				}
				return 0
			})

		}
		sns = append(sns, sn)
	}

	maxBufSizePerStorageNode = memory.Allowed() / 8 / len(sns)
	if maxBufSizePerStorageNode > consts.MaxInsertPacketSizeForVMInsert {
		maxBufSizePerStorageNode = consts.MaxInsertPacketSizeForVMInsert
	}

	metrics.RegisterSet(ms)

	var wg sync.WaitGroup
	snb := &storageNodesBucket{
		ms:        ms,
		nodesHash: nodesHash,
		sns:       sns,
		stopCh:    stopCh,
		wg:        &wg,
	}

	for idx, sn := range sns {
		wg.Go(func() {
			sn.run(snb, idx)
		})
	}

	return snb
}

func mustStopStorageNodes(snb *storageNodesBucket) {
	close(snb.stopCh)
	for _, sn := range snb.sns {
		sn.brCond.Broadcast()
	}
	snb.wg.Wait()
	metrics.UnregisterSet(snb.ms, true)
}

// rerouteRowsToReadyStorageNodes reroutes src from not ready snSource to ready storage nodes.
//
// The function blocks until src is fully re-routed.
func rerouteRowsToReadyStorageNodes(snb *storageNodesBucket, snSource *storageNode, src []byte, getRowHasher func() rowHasher) (int, error) {
	reroutesTotal.Inc()
	rowsProcessed := 0
	var idxsExclude, idxsExcludeNew []int
	nodesHash := snb.nodesHash
	sns := snb.sns
	idxsExclude = getNotReadyStorageNodeIdxsBlocking(snb, idxsExclude[:0])
	rowHasher := getRowHasher()
	for len(src) > 0 {
		h, tail, err := rowHasher(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal MetricRow: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail
		reroutedRowsProcessed.Inc()
		var sn *storageNode
		for {
			idx := nodesHash.GetNodeIdx(h, idxsExclude)
			sn = sns[idx]
			if sn.isReady() {
				break
			}
			select {
			case <-sn.stopCh:
				return rowsProcessed, fmt.Errorf("graceful shutdown started")
			default:
			}

			// re-generate idxsExclude list, since sn must be put there.
			idxsExclude = getNotReadyStorageNodeIdxsBlocking(snb, idxsExclude[:0])
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
		idxsExcludeNew = getNotReadyStorageNodeIdxs(snb, idxsExcludeNew[:0], sn)
		idx := nodesHash.GetNodeIdx(h, idxsExcludeNew)
		snNew := sns[idx]
		if !snNew.trySendBuf(rowBuf, 1) {
			// The row cannot be sent to both snSource, sn and snNew without blocking.
			// Sleep for a while and try sending the row to snSource again.
			time.Sleep(100 * time.Millisecond)
			goto again
		}
		rowsProcessed++
		if snNew != snSource {
			snSource.rowsReroutedFromHere.Inc()
			snNew.rowsReroutedToHere.Inc()
		}
	}
	return rowsProcessed, nil
}

var reroutingLogger = logger.WithThrottler("allowRerouting", 5*time.Second)

// allowRerouting determines whether data should be rerouted from snSource to other storage nodes (sns)
// based on performance metrics.
//
// It returns true only when snSource is the slowest node in the cluster
// and significantly slower than the cluster on average.
// See the comments below for detailed conditions.
func allowRerouting(snSource *storageNode, sns []*storageNode) bool {
	if *disableRerouting {
		return false
	}

	// Do not allow rerouting if saturation is not yet warmed up.
	snSourceSaturation := snSource.avgSaturation.Value()
	if snSourceSaturation == 0 {
		return false
	}

	saturations := make([]float64, 0, len(sns))
	for _, sn := range sns {
		// Skip not ready storage nodes.
		if !sn.isReady() {
			continue
		}
		// Do not allow rerouting if avgSaturation is not yet warmed up.
		if sn.avgSaturation.Value() == 0 {
			return false
		}

		// Do not allow rerouting if there is a slower storage node
		snSaturation := sn.avgSaturation.Value()
		if snSourceSaturation < snSaturation {
			return false
		}

		saturations = append(saturations, snSaturation)
	}
	// Do not allow rerouting if there are less than 3 ready storage nodes.
	if len(saturations) < 3 {
		return false
	}

	// Calculate median saturation
	sort.Float64s(saturations)
	var medianSaturation float64
	n := len(saturations)
	if n%2 == 0 {
		medianSaturation = (saturations[n/2-1] + saturations[n/2]) / 2
	} else {
		medianSaturation = saturations[n/2]
	}

	// Do not allow rerouting if the cluster is significantly overloaded.
	if medianSaturation > 0.80 {
		return false
	}

	reroutingLogger.Warnf("reroute metrics from the slowest storage %q with saturation %.2f, where cluster median saturation is %.2f", snSource.dialer.Addr(), snSourceSaturation, medianSaturation)
	return true
}

// rerouteRowsToFreeStorageNodes re-routes src from snSource to other storage nodes.
//
// It is expected that snSource has no enough buffer for sending src.
// It is expected than *disableRerouting isn't set when calling this function.
// It is expected that len(snb.sns) >= 2
func rerouteRowsToFreeStorageNodes(snb *storageNodesBucket, snSource *storageNode, src []byte, getRowHasher func() rowHasher) (int, error) {
	if *disableRerouting {
		logger.Panicf("BUG: disableRerouting must be disabled when calling rerouteRowsToFreeStorageNodes")
	}

	sns := snb.sns
	if len(sns) < 2 {
		logger.Panicf("BUG: the number of storage nodes is too small for calling rerouteRowsToFreeStorageNodes: %d", len(sns))
	}
	reroutesTotal.Inc()
	rowsProcessed := 0
	var idxsExclude []int
	nodesHash := snb.nodesHash
	idxsExclude = getNotReadyStorageNodeIdxs(snb, idxsExclude[:0], snSource)
	rowHasher := getRowHasher()
	for len(src) > 0 {
		h, tail, err := rowHasher(src)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal row: %s", err)
		}
		rowBuf := src[:len(src)-len(tail)]
		src = tail
		reroutedRowsProcessed.Inc()

	again:
		// Try sending the row to snSource in order to minimize re-routing.
		if snSource.trySendBuf(rowBuf, 1) {
			rowsProcessed++
			continue
		}
		// The row couldn't be sent to snSrouce. Try re-routing it to other node.
		idx := nodesHash.GetNodeIdx(h, idxsExclude)
		sn := sns[idx]
		for !sn.isReady() && len(idxsExclude) < len(sns) {
			// re-generate idxsExclude list, since sn and snSource must be put there.
			idxsExclude = getNotReadyStorageNodeIdxs(snb, idxsExclude[:0], snSource)
			idx := nodesHash.GetNodeIdx(h, idxsExclude)
			sn = sns[idx]
		}
		if !sn.trySendBuf(rowBuf, 1) {
			// The row cannot be sent to both snSource and sn without blocking.
			// Sleep for a while and try sending the row to snSource again.
			time.Sleep(100 * time.Millisecond)
			goto again
		}
		rowsProcessed++
		snSource.rowsReroutedFromHere.Inc()
		sn.rowsReroutedToHere.Inc()
	}
	return rowsProcessed, nil
}

func getNotReadyStorageNodeIdxsBlocking(snb *storageNodesBucket, dst []int) []int {
	dst = getNotReadyStorageNodeIdxs(snb, dst[:0], nil)
	sns := snb.sns
	if len(dst) < len(sns) {
		return dst
	}
	noStorageNodesLogger.Warnf("all the vmstorage nodes are unavailable; stopping data processing util at least a single node becomes available")
	for {
		tc := timerpool.Get(time.Second)
		select {
		case <-snb.stopCh:
			timerpool.Put(tc)
			return dst
		case <-tc.C:
			timerpool.Put(tc)
		}

		dst = getNotReadyStorageNodeIdxs(snb, dst[:0], nil)
		if availableNodes := len(sns) - len(dst); availableNodes > 0 {
			storageNodesBecameAvailableLogger.Warnf("%d vmstorage nodes became available, so continue data processing", availableNodes)
			return dst
		}
	}
}

var storageNodesBecameAvailableLogger = logger.WithThrottler("storageNodesBecameAvailable", 5*time.Second)

var noStorageNodesLogger = logger.WithThrottler("storageNodesUnavailable", 5*time.Second)

func getNotReadyStorageNodeIdxs(snb *storageNodesBucket, dst []int, snExtra *storageNode) []int {
	dst = dst[:0]
	for i, sn := range snb.sns {
		if sn == snExtra || !sn.isReady() {
			dst = append(dst, i)
		}
	}
	return dst
}

func (sn *storageNode) trySendBuf(buf []byte, rows int) bool {
	if !sn.isReady() {
		// Fast path without locking the sn.brLock.
		return false
	}

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
		case <-sn.stopCh:
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

func (sn *storageNode) readOnlyChecker() {
	d := timeutil.AddJitterToDuration(time.Second * 30)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-sn.stopCh:
			return
		case <-ticker.C:
			sn.checkReadOnlyMode()
		}
	}
}

func (sn *storageNode) checkReadOnlyMode() {
	if !sn.isReadOnly.Load() {
		// fast path - the sn isn't in readonly mode
		return
	}
	// Check whether the storage remains in readonly mode
	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()
	if sn.bc == nil {
		return
	}
	var err error
	if sn.bc.IsLegacy {
		err = vminsertapi.SendToConn(sn.bc, nil)
	} else {
		err = vminsertapi.SendRPCRequestToConn(sn.bc, sn.rpcCall.VersionedName, nil)
	}
	if err == nil {
		// The storage switched from readonly to non-readonly mode
		sn.isReadOnly.Store(false)
		return
	}
	if errors.Is(err, storage.ErrReadOnly) {
		// The storage remains in read-only mode
		return
	}

	// There was an error when sending nil buf to the storage.
	logger.Errorf("cannot check storage readonly mode for -storageNode=%q: %s", sn.dialer.Addr(), err)

	// Mark the connection to the storage as broken.
	if err = sn.bc.Close(); err != nil {
		cannotCloseStorageNodeConnLogger.Warnf("cannot close connection to storageNode %q: %s", sn.dialer.Addr(), err)
	}
	sn.bc = nil
	sn.isBroken.Store(true)
	sn.brCond.Broadcast()
	sn.connectionErrors.Inc()
}

var (
	maxBufSizePerStorageNode int

	reroutedRowsProcessed           = metrics.NewCounter(`vm_rpc_rerouted_rows_processed_total{name="vminsert"}`)
	reroutesTotal                   = metrics.NewCounter(`vm_rpc_reroutes_total{name="vminsert"}`)
	rowsIncompletelyReplicatedTotal = metrics.NewCounter(`vm_rpc_rows_incompletely_replicated_total{name="vminsert"}`)
)
