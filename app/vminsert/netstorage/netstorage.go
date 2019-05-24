package netstorage

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Disable compression of RPC traffic. This reduces CPU usage at the cost of higher network bandwidth usage")

// sendWithFallback sends buf to storage node sn.
//
// It falls back to sending data to another storage node if sn is currently
// unavailable.
func (sn *storageNode) sendWithFallback(buf []byte, sizeBuf []byte) error {
	deadline := time.Now().Add(30 * time.Second)
	err := sn.sendBuf(buf, deadline, sizeBuf)
	if err == nil {
		return nil
	}

	// Failed to send the data to sn. Try sending it to another storageNodes.
	if time.Until(deadline) <= 0 {
		sn.timeouts.Inc()
		return err
	}
	if len(storageNodes) == 1 {
		return err
	}
	idx := func() int {
		for i, snOther := range storageNodes {
			if sn == snOther {
				return i
			}
		}
		logger.Panicf("BUG: cannot find storageNode %p in storageNodes %p", sn, storageNodes)
		return -1
	}()
	for i := 0; i < len(storageNodes); i++ {
		idx++
		if idx >= len(storageNodes) {
			idx = 0
		}
		err = storageNodes[idx].sendBuf(buf, deadline, sizeBuf)
		if err == nil {
			storageNodes[idx].fallbacks.Inc()
			return nil
		}
		if time.Until(deadline) <= 0 {
			sn.timeouts.Inc()
			return err
		}
	}
	return err
}

func (sn *storageNode) sendBuf(buf []byte, deadline time.Time, sizeBuf []byte) error {
	// sizeBuf guarantees that the rows batch will be either fully
	// read or fully discarded on the vmstorage.
	// sizeBuf is used for read optimization in vmstorage.
	encoding.MarshalUint64(sizeBuf[:0], uint64(len(buf)))

	sn.bcLock.Lock()
	defer sn.bcLock.Unlock()

	if sn.bc == nil {
		if err := sn.dial(); err != nil {
			return fmt.Errorf("cannot dial %q: %s", sn.dialer.Addr(), err)
		}
	}

	if err := sn.sendBufNolock(buf, deadline, sizeBuf); err != nil {
		sn.closeConn()
		return err
	}
	return nil
}

func (sn *storageNode) sendBufNolock(buf []byte, deadline time.Time, sizeBuf []byte) error {
	if err := sn.bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write deadline to %s: %s", deadline, err)
	}
	if _, err := sn.bc.Write(sizeBuf); err != nil {
		return fmt.Errorf("cannot write data size %d: %s", len(buf), err)
	}
	if _, err := sn.bc.Write(buf); err != nil {
		return fmt.Errorf("cannot write data: %s", err)
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

func (sn *storageNode) closeConn() {
	_ = sn.bc.Close()
	sn.bc = nil
	sn.connectionErrors.Inc()
}

func (sn *storageNode) run() {
	mustStop := false
	for !mustStop {
		select {
		case <-stopCh:
			mustStop = true
		case <-time.After(time.Second):
		}

		sn.bcLock.Lock()
		if err := sn.flushNolock(); err != nil {
			sn.closeConn()
			logger.Errorf("cannot flush data to storageNode %q: %s", sn.dialer.Addr(), err)
		}
		sn.bcLock.Unlock()
	}
}

func (sn *storageNode) flushNolock() error {
	if sn.bc == nil {
		return nil
	}
	if err := sn.bc.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("cannot set write deadline: %s", err)
	}
	return sn.bc.Flush()
}

// storageNode is a client sending data to storage node.
type storageNode struct {
	dialer *netutil.TCPDialer

	bc     *handshake.BufferedConn
	bcLock sync.Mutex

	// The number of times the storage node was timed out (overflown).
	timeouts *metrics.Counter

	// The number of dial errors to storage node.
	dialErrors *metrics.Counter

	// The number of handshake errors to storage node.
	handshakeErrors *metrics.Counter

	// The number of connection errors to storage node.
	connectionErrors *metrics.Counter

	// The number of fallbacks to this node.
	fallbacks *metrics.Counter

	// The number of rows pushed to storage node.
	RowsPushed *metrics.Counter
}

// storageNodes contains a list of storage node clients.
var storageNodes []*storageNode

var storageNodesWG sync.WaitGroup

var stopCh = make(chan struct{})

// InitStorageNodes initializes storage nodes' connections to the given addrs.
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

			timeouts:         metrics.NewCounter(fmt.Sprintf(`vm_rpc_timeouts_total{name="vminsert", addr=%q}`, addr)),
			dialErrors:       metrics.NewCounter(fmt.Sprintf(`vm_rpc_dial_errors_total{name="vminsert", addr=%q}`, addr)),
			handshakeErrors:  metrics.NewCounter(fmt.Sprintf(`vm_rpc_handshake_errors_total{name="vminsert", addr=%q}`, addr)),
			connectionErrors: metrics.NewCounter(fmt.Sprintf(`vm_rpc_connection_errors_total{name="vminsert", addr=%q}`, addr)),
			fallbacks:        metrics.NewCounter(fmt.Sprintf(`vm_rpc_fallbacks_total{name="vminsert", addr=%q}`, addr)),
			RowsPushed:       metrics.NewCounter(fmt.Sprintf(`vm_rpc_rows_pushed_total{name="vminsert", addr=%q}`, addr)),
		}
		storageNodes = append(storageNodes, sn)
		storageNodesWG.Add(1)
		go func(addr string) {
			sn.run()
			storageNodesWG.Done()
		}(addr)
	}
}

// Stop gracefully stops netstorage.
func Stop() {
	close(stopCh)
	storageNodesWG.Wait()
}
