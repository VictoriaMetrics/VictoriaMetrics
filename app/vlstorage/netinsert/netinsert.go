package netinsert

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fastrand"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
)

// the maximum size of a single data block sent to storage node.
const maxInsertBlockSize = 2 * 1024 * 1024

// ProtocolVersion is the version of the data ingestion protocol.
//
// It must be changed every time the data encoding at /internal/insert HTTP endpoint is changed.
const ProtocolVersion = "v1"

// Storage is a network storage for sending data to remote storage nodes in the cluster.
type Storage struct {
	sns []*storageNode

	disableCompression bool

	srt *streamRowsTracker

	pendingDataBuffers chan *bytesutil.ByteBuffer

	stopCh chan struct{}
	wg     sync.WaitGroup
}

type storageNode struct {
	// scheme is http or https scheme to communicate with addr
	scheme string

	// addr is TCP address of storage node to send the ingested data to
	addr string

	// s is a storage, which holds the given storageNode
	s *Storage

	// c is an http client used for sending data blocks to addr.
	c *http.Client

	// ac is auth config used for setting request headers such as Authorization and Host.
	ac *promauth.Config

	// pendingData contains pending data, which must be sent to the storage node at the addr.
	pendingDataMu        sync.Mutex
	pendingData          *bytesutil.ByteBuffer
	pendingDataLastFlush time.Time

	// the unix timestamp until the storageNode is disabled for data writing.
	disabledUntil atomic.Uint64
}

func newStorageNode(s *Storage, addr string, ac *promauth.Config, isTLS bool) *storageNode {
	tr := httputil.NewTransport(false, "vlinsert_backend")
	tr.TLSHandshakeTimeout = 20 * time.Second
	tr.DisableCompression = true

	scheme := "http"
	if isTLS {
		scheme = "https"
	}

	sn := &storageNode{
		scheme: scheme,
		addr:   addr,
		s:      s,
		c: &http.Client{
			Transport: ac.NewRoundTripper(tr),
		},
		ac: ac,

		pendingData: &bytesutil.ByteBuffer{},
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		sn.backgroundFlusher()
	}()

	return sn
}

func (sn *storageNode) backgroundFlusher() {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-sn.s.stopCh:
			return
		case <-t.C:
			sn.flushPendingData()
		}
	}
}

func (sn *storageNode) flushPendingData() {
	sn.pendingDataMu.Lock()
	if time.Since(sn.pendingDataLastFlush) < time.Second {
		// nothing to flush
		sn.pendingDataMu.Unlock()
		return
	}

	pendingData := sn.grabPendingDataForFlushLocked()
	sn.pendingDataMu.Unlock()

	sn.mustSendInsertRequest(pendingData)
}

func (sn *storageNode) addRow(r *logstorage.InsertRow) {
	bb := bbPool.Get()
	b := bb.B

	b = r.Marshal(b)

	if len(b) > maxInsertBlockSize {
		logger.Warnf("skipping too long log entry, since its length exceeds %d bytes; the actual log entry length is %d bytes; log entry contents: %s", maxInsertBlockSize, len(b), b)
		return
	}

	var pendingData *bytesutil.ByteBuffer
	sn.pendingDataMu.Lock()
	if sn.pendingData.Len()+len(b) > maxInsertBlockSize {
		pendingData = sn.grabPendingDataForFlushLocked()
	}
	sn.pendingData.MustWrite(b)
	sn.pendingDataMu.Unlock()

	bb.B = b
	bbPool.Put(bb)

	if pendingData != nil {
		sn.mustSendInsertRequest(pendingData)
	}
}

var bbPool bytesutil.ByteBufferPool

func (sn *storageNode) grabPendingDataForFlushLocked() *bytesutil.ByteBuffer {
	sn.pendingDataLastFlush = time.Now()
	pendingData := sn.pendingData
	sn.pendingData = <-sn.s.pendingDataBuffers

	return pendingData
}

func (sn *storageNode) mustSendInsertRequest(pendingData *bytesutil.ByteBuffer) {
	defer func() {
		pendingData.Reset()
		sn.s.pendingDataBuffers <- pendingData
	}()

	err := sn.sendInsertRequest(pendingData)
	if err == nil {
		return
	}

	if !errors.Is(err, errTemporarilyDisabled) {
		logger.Warnf("%s; re-routing the data block to the remaining nodes", err)
	}
	for !sn.s.sendInsertRequestToAnyNode(pendingData) {
		logger.Errorf("cannot send pending data to all storage nodes, since all of them are unavailable; re-trying to send the data in a second")

		t := timerpool.Get(time.Second)
		select {
		case <-sn.s.stopCh:
			timerpool.Put(t)
			logger.Errorf("dropping %d bytes of data, since there are no available storage nodes", pendingData.Len())
			return
		case <-t.C:
			timerpool.Put(t)
		}
	}
}

func (sn *storageNode) sendInsertRequest(pendingData *bytesutil.ByteBuffer) error {
	dataLen := pendingData.Len()
	if dataLen == 0 {
		// Nothing to send.
		return nil
	}

	if sn.disabledUntil.Load() > fasttime.UnixTimestamp() {
		return errTemporarilyDisabled
	}

	ctx, cancel := contextutil.NewStopChanContext(sn.s.stopCh)
	defer cancel()

	var body io.Reader
	if !sn.s.disableCompression {
		bb := zstdBufPool.Get()
		defer zstdBufPool.Put(bb)

		bb.B = zstd.CompressLevel(bb.B[:0], pendingData.B, 1)
		body = bb.NewReader()
	} else {
		body = pendingData.NewReader()
	}

	reqURL := sn.getRequestURL("/internal/insert")
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, body)
	if err != nil {
		logger.Panicf("BUG: unexpected error when creating an http request: %s", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if !sn.s.disableCompression {
		req.Header.Set("Content-Encoding", "zstd")
	}
	if err := sn.ac.SetHeaders(req, true); err != nil {
		return fmt.Errorf("cannot set auth headers for %q: %w", reqURL, err)
	}

	resp, err := sn.c.Do(req)
	if err != nil {
		// Disable sn for data writing for 10 seconds.
		sn.disabledUntil.Store(fasttime.UnixTimestamp() + 10)

		return fmt.Errorf("cannot send data block with the length %d to %q: %s", pendingData.Len(), reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 == 2 {
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respBody = []byte(fmt.Sprintf("%s", err))
	}

	// Disable sn for data writing for 10 seconds.
	sn.disabledUntil.Store(fasttime.UnixTimestamp() + 10)

	return fmt.Errorf("unexpected status code returned when sending data block to %q: %d; want 2xx; response body: %q", reqURL, resp.StatusCode, respBody)
}

func (sn *storageNode) getRequestURL(path string) string {
	return fmt.Sprintf("%s://%s%s?version=%s", sn.scheme, sn.addr, path, url.QueryEscape(ProtocolVersion))
}

var zstdBufPool bytesutil.ByteBufferPool

// NewStorage returns new Storage for the given addrs with the given authCfgs.
//
// The concurrency is the average number of concurrent connections per every addr.
//
// If disableCompression is set, then the data is sent uncompressed to the remote storage.
//
// Call MustStop on the returned storage when it is no longer needed.
func NewStorage(addrs []string, authCfgs []*promauth.Config, isTLSs []bool, concurrency int, disableCompression bool) *Storage {
	pendingDataBuffers := make(chan *bytesutil.ByteBuffer, concurrency*len(addrs))
	for i := 0; i < cap(pendingDataBuffers); i++ {
		pendingDataBuffers <- &bytesutil.ByteBuffer{}
	}

	s := &Storage{
		disableCompression: disableCompression,
		pendingDataBuffers: pendingDataBuffers,
		stopCh:             make(chan struct{}),
	}

	sns := make([]*storageNode, len(addrs))
	for i, addr := range addrs {
		sns[i] = newStorageNode(s, addr, authCfgs[i], isTLSs[i])
	}
	s.sns = sns

	s.srt = newStreamRowsTracker(len(sns))

	return s
}

// MustStop stops the s.
func (s *Storage) MustStop() {
	close(s.stopCh)
	s.wg.Wait()
	s.sns = nil
}

// AddRow adds the given log row into s.
func (s *Storage) AddRow(streamHash uint64, r *logstorage.InsertRow) {
	idx := s.srt.getNodeIdx(streamHash)
	sn := s.sns[idx]
	sn.addRow(r)
}

func (s *Storage) sendInsertRequestToAnyNode(pendingData *bytesutil.ByteBuffer) bool {
	startIdx := int(fastrand.Uint32n(uint32(len(s.sns))))
	for i := range s.sns {
		idx := (startIdx + i) % len(s.sns)
		sn := s.sns[idx]
		err := sn.sendInsertRequest(pendingData)
		if err == nil {
			return true
		}
		if !errors.Is(err, errTemporarilyDisabled) {
			logger.Warnf("cannot send pending data to the storage node %q: %s; trying to send it to another storage node", sn.addr, err)
		}
	}
	return false
}

var errTemporarilyDisabled = fmt.Errorf("writing to the node is temporarily disabled")

type streamRowsTracker struct {
	mu sync.Mutex

	nodesCount    int64
	rowsPerStream map[uint64]uint64
}

func newStreamRowsTracker(nodesCount int) *streamRowsTracker {
	return &streamRowsTracker{
		nodesCount:    int64(nodesCount),
		rowsPerStream: make(map[uint64]uint64),
	}
}

func (srt *streamRowsTracker) getNodeIdx(streamHash uint64) uint64 {
	if srt.nodesCount == 1 {
		// Fast path for a single node.
		return 0
	}

	srt.mu.Lock()
	defer srt.mu.Unlock()

	streamRows := srt.rowsPerStream[streamHash] + 1
	srt.rowsPerStream[streamHash] = streamRows

	if streamRows <= 1000 {
		// Write the initial rows for the stream to a single storage node for better locality.
		// This should work great for log streams containing small number of logs, since will be distributed
		// evenly among available storage nodes because they have different streamHash.
		return streamHash % uint64(srt.nodesCount)
	}

	// The log stream contains more than 1000 rows. Distribute them among storage nodes at random
	// in order to improve query performance over this stream (the data for the log stream
	// can be processed in parallel on all the storage nodes).
	//
	// The random distribution is preferred over round-robin distribution in order to avoid possible
	// dependency between the order of the ingested logs and the number of storage nodes,
	// which may lead to non-uniform distribution of logs among storage nodes.
	return uint64(fastrand.Uint32n(uint32(srt.nodesCount)))
}
