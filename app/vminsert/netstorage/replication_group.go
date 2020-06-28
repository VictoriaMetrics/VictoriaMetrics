package netstorage

import (
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

type replicationGroup struct {
	name                     string
	storageNodes             []*storageNode
	maxBufSizePerStorageNode int

	reroutedBR         bufRows
	reroutedBRLock     sync.Mutex
	reroutedBRCond     *sync.Cond
	reroutedBufMaxSize int

	reroutedRowsProcessed           *metrics.Counter
	reroutedBufWaits                *metrics.Counter
	reroutesTotal                   *metrics.Counter
	rerouteErrors                   *metrics.Counter
	rowsLostTotal                   *metrics.Counter
	rowsIncompletelyReplicatedTotal *metrics.Counter

	storageNodesWG      sync.WaitGroup
	rerouteWorkerWG     sync.WaitGroup
	storageNodesStopCh  chan struct{}
	rerouteWorkerStopCh chan struct{}
}

var replicationGroups map[string]*replicationGroup

func initReplicationGroups() {
	for _, rg := range replicationGroups {
		rg.initReplicationGroup()
	}
}

func (rg *replicationGroup) initReplicationGroup() {
	rg.reroutedRowsProcessed = metrics.NewCounter(fmt.Sprintf(`vm_rpc_rerouted_rows_processed_total{name="vminsert",group="%s"}`, rg.name))
	rg.reroutedBufWaits = metrics.NewCounter(fmt.Sprintf(`vm_rpc_rerouted_buf_waits_total{name="vminsert",group="%s"}`, rg.name))
	rg.reroutesTotal = metrics.NewCounter(fmt.Sprintf(`vm_rpc_reroutes_total{name="vminsert",group="%s"}`, rg.name))
	_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_rerouted_rows_pending{name="vminsert",group="%s"}`, rg.name), func() float64 {
		rg.reroutedBRLock.Lock()
		n := rg.reroutedBR.rows
		rg.reroutedBRLock.Unlock()
		return float64(n)
	})
	_ = metrics.NewGauge(fmt.Sprintf(`vm_rpc_rerouted_buf_pending_bytes{name="vminsert",group="%s"}`, rg.name), func() float64 {
		rg.reroutedBRLock.Lock()
		n := len(rg.reroutedBR.buf)
		rg.reroutedBRLock.Unlock()
		return float64(n)
	})
	rg.rerouteErrors = metrics.NewCounter(fmt.Sprintf(`vm_rpc_reroute_errors_total{name="vminsert",group="%s"}`, rg.name))
	rg.rowsLostTotal = metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_lost_total{name="vminsert",group="%s"}`, rg.name))
	rg.rowsIncompletelyReplicatedTotal = metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_incompletely_replicated_total{name="vminsert",group="%s"}`, rg.name))

	rg.reroutedBRCond = sync.NewCond(&rg.reroutedBRLock)
	rg.rerouteWorkerStopCh = make(chan struct{})

	rg.maxBufSizePerStorageNode = memory.Allowed() / 8 / len(rg.storageNodes)
	if rg.maxBufSizePerStorageNode > consts.MaxInsertPacketSize {
		rg.maxBufSizePerStorageNode = consts.MaxInsertPacketSize
	}
	rg.reroutedBufMaxSize = memory.Allowed() / 16
	if rg.reroutedBufMaxSize < rg.maxBufSizePerStorageNode {
		rg.reroutedBufMaxSize = rg.maxBufSizePerStorageNode
	}
	if rg.reroutedBufMaxSize > rg.maxBufSizePerStorageNode*len(rg.storageNodes) {
		rg.reroutedBufMaxSize = rg.maxBufSizePerStorageNode * len(rg.storageNodes)
	}

	for idx, sn := range rg.storageNodes {
		rg.storageNodesWG.Add(1)
		go func(sn *storageNode, idx int) {
			sn.run(rg.storageNodesStopCh, idx)
			rg.storageNodesWG.Done()
		}(sn, idx)
	}

	rg.rerouteWorkerWG.Add(1)
	go func() {
		rg.rerouteWorker(rg.rerouteWorkerStopCh)
		rg.rerouteWorkerWG.Done()
	}()
}

// Stop gracefully stops netstorage.
func Stop() {
	for _, rg := range replicationGroups {
		close(rg.rerouteWorkerStopCh)
		rg.rerouteWorkerWG.Wait()

		close(rg.storageNodesStopCh)
		rg.storageNodesWG.Wait()
	}
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
func (rg *replicationGroup) addToReroutedBufMayBlock(buf []byte, rows int) error {
	if len(buf) > rg.reroutedBufMaxSize {
		logger.Panicf("BUG: len(buf)=%d cannot exceed reroutedBufMaxSize=%d", len(buf), rg.reroutedBufMaxSize)
	}

	rg.reroutedBRLock.Lock()
	defer rg.reroutedBRLock.Unlock()

	for len(rg.reroutedBR.buf)+len(buf) > rg.reroutedBufMaxSize {
		if rg.getHealthyStorageNodesCount() == 0 {
			rg.rowsLostTotal.Add(rows)
			return fmt.Errorf("all the vmstorage nodes are unavailable and reroutedBR has no enough space for storing %d bytes; only %d bytes left in reroutedBR",
				len(buf), rg.reroutedBufMaxSize-len(rg.reroutedBR.buf))
		}
		select {
		case <-rg.rerouteWorkerStopCh:
			rg.rowsLostTotal.Add(rows)
			return fmt.Errorf("rerouteWorker cannot send the data since it is stopped")
		default:
		}

		// The reroutedBR.buf has no enough space for len(buf). Wait while the reroutedBR.buf is be sent by rerouteWorker.
		rg.reroutedBufWaits.Inc()
		rg.reroutedBRCond.Wait()
	}
	rg.reroutedBR.buf = append(rg.reroutedBR.buf, buf...)
	rg.reroutedBR.rows += rows
	rg.reroutesTotal.Inc()
	return nil
}

func (rg *replicationGroup) getHealthyStorageNodesCount() int {
	n := 0
	for _, sn := range rg.storageNodes {
		if !sn.isBroken() {
			n++
		}
	}
	return n
}

func (rg *replicationGroup) getHealthyStorageNodes() []*storageNode {
	sns := make([]*storageNode, 0, len(rg.storageNodes)-1)
	for _, sn := range rg.storageNodes {
		if !sn.isBroken() {
			sns = append(sns, sn)
		}
	}
	return sns
}

func (rg *replicationGroup) rerouteWorker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var br bufRows
	brLastResetTime := fasttime.UnixTimestamp()
	var waitCh <-chan struct{}
	mustStop := false
	for !mustStop {
		rg.reroutedBRLock.Lock()
		bufLen := len(rg.reroutedBR.buf)
		rg.reroutedBRLock.Unlock()
		waitCh = nil
		if len(br.buf) == 0 && bufLen > rg.reroutedBufMaxSize/4 {
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
			rg.reroutedBRLock.Lock()
			rg.reroutedBR, br = br, rg.reroutedBR
			rg.reroutedBRLock.Unlock()
		}
		rg.reroutedBRCond.Broadcast()
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
		sns := rg.getHealthyStorageNodes()
		if len(sns) == 0 {
			// No more vmstorage nodes to write data to.
			rg.rerouteErrors.Inc()
			logger.Errorf("cannot send rerouted rows because all the storage nodes are unhealthy")
			// Do not reset br in the hope it could be sent next time.
			continue
		}
		rg.spreadReroutedBufToStorageNodes(sns, &br)
		// There is no need in br.reset() here, since it is already done in spreadReroutedBufToStorageNodes.
	}
	// Notify all the blocked addToReroutedBufMayBlock callers, so they may finish the work.
	rg.reroutedBRCond.Broadcast()
}

func (rg *replicationGroup) spreadReroutedBufToStorageNodes(sns []*storageNode, br *bufRows) {
	var mr storage.MetricRow
	rowsProcessed := 0
	defer rg.reroutedRowsProcessed.Add(rowsProcessed)

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
