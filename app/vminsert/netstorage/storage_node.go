package netstorage

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage")
	replicationFactor     = flag.Int("replicationFactor", 1, "Replication factor for the ingested data, i.e. how many copies to make among distinct -storageNode instances. "+
		"Note that vmselect must run with -dedup.minScrapeInterval=1ms for data de-duplication when replicationFactor is greater than 1. "+
		"Higher values for -dedup.minScrapeInterval at vmselect is OK")
)

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

	// Replication group this node is a member of
	replicationGroupOwner *replicationGroup
}

// InitStorageNodes initializes vmstorage nodes' connections to the given addrs.
func InitStorageNodes(addrGroups []string) {
	if len(addrGroups) == 0 {
		logger.Panicf("BUG: addrs must be non-empty")
	}
	if len(addrGroups) > 255 {
		logger.Panicf("BUG: too much addresses: %d; max supported %d addresses", len(addrGroups), 255)
	}

	replicationGroups = make(map[string]*replicationGroup)

	for _, addrGroup := range addrGroups {
		group, addr := splitReplicationGroup(addrGroup)
		sn := createStorageNode(addr)
		appendNodeToReplicationGroup(group, sn)
	}

	initReplicationGroups()
}

func splitReplicationGroup(addr string) (string, string) {
	hostGroup := strings.Split(addr, "/")
	switch len(hostGroup) {
	case 1:
		return "default", hostGroup[0]
	case 2:
		return hostGroup[0], hostGroup[1]
	default:
		return "", ""
	}
}

func createStorageNode(addr string) *storageNode {
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

	return sn
}

func appendNodeToReplicationGroup(group string, sn *storageNode) {
	if _, ok := replicationGroups[group]; ok {
		replicationGroups[group].storageNodes = append(replicationGroups[group].storageNodes, sn)
	} else {
		rg := &replicationGroup{name: group, storageNodes: []*storageNode{sn}}
		replicationGroups[group] = rg
	}

	sn.replicationGroupOwner = replicationGroups[group]
}

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
	if len(buf) > sn.replicationGroupOwner.reroutedBufMaxSize {
		logger.Panicf("BUG: len(buf)=%d cannot exceed %d", len(buf), sn.replicationGroupOwner.maxBufSizePerStorageNode)
	}
	sn.rowsPushed.Add(rows)

	if sn.isBroken() {
		// The vmstorage node is temporarily broken. Re-route buf to healthy vmstorage nodes.
		if err := sn.replicationGroupOwner.addToReroutedBufMayBlock(buf, rows); err != nil {
			return fmt.Errorf("%d rows dropped because the current vsmtorage is unavailable and %s", rows, err)
		}
		sn.rowsReroutedFromHere.Add(rows)
		return nil
	}

	sn.brLock.Lock()
	if len(sn.br.buf)+len(buf) <= sn.replicationGroupOwner.maxBufSizePerStorageNode {
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
	if err := sn.replicationGroupOwner.addToReroutedBufMayBlock(buf, rows); err != nil {
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

func (sn *storageNode) run(stopCh <-chan struct{}, snIdx int) {
	replicas := *replicationFactor
	if replicas <= 0 {
		replicas = 1
	}
	if replicas > len(sn.replicationGroupOwner.storageNodes) {
		replicas = len(sn.replicationGroupOwner.storageNodes)
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
		if len(br.buf) == 0 && bufLen > sn.replicationGroupOwner.maxBufSizePerStorageNode/4 {
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
		if !sn.sendBufToReplicas(&br, snIdx, replicas) {
			// do not reset br in the hope it will be sent next time.
			continue
		}
		br.reset()
	}
}

func (sn *storageNode) sendBufToReplicas(br *bufRows, snIdx, replicas int) bool {
	usedStorageNodes := make(map[*storageNode]bool, replicas)
	for i := 0; i < replicas; i++ {
		idx := snIdx + i
		attempts := 0
		for {
			attempts++
			if attempts > len(sn.replicationGroupOwner.storageNodes) {
				if i == 0 {
					// The data wasn't replicated at all.
					logger.Warnf("cannot push %d bytes with %d rows to storage nodes, since all the nodes are temporarily unavailable; "+
						"re-trying to send the data soon", len(br.buf), br.rows)
					return false
				}
				// The data is partially replicated, so just emit a warning and return true.
				// We could retry sending the data again, but this may result in uncontrolled duplicate data.
				// So it is better returning true.
				sn.replicationGroupOwner.rowsIncompletelyReplicatedTotal.Add(br.rows)
				logger.Warnf("cannot make a copy #%d out of %d copies according to -replicationFactor=%d for %d bytes with %d rows, "+
					"since a part of storage nodes is temporarily unavailable", i+1, replicas, *replicationFactor, len(br.buf), br.rows)
				return true
			}
			if idx >= len(sn.replicationGroupOwner.storageNodes) {
				idx %= len(sn.replicationGroupOwner.storageNodes)
			}
			sn := sn.replicationGroupOwner.storageNodes[idx]
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

func (sn *storageNode) sendReroutedRow(buf []byte) bool {
	sn.brLock.Lock()
	ok := len(sn.br.buf)+len(buf) <= sn.replicationGroupOwner.maxBufSizePerStorageNode
	if ok {
		sn.br.buf = append(sn.br.buf, buf...)
		sn.br.rows++
		sn.rowsReroutedToHere.Inc()
	}
	sn.brLock.Unlock()
	return ok
}
