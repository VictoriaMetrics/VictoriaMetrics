package transport

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/clusternative"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxTagKeysPerSearch          = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search")
	maxTagValuesPerSearch        = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	maxMetricsPerSearch          = flag.Int("search.maxUniqueTimeseries", 0, "The maximum number of unique time series, which can be scanned during every query. This allows protecting against heavy queries, which select unexpectedly high number of series. Zero means 'no limit'. See also -search.max* command-line flags at vmselect")

	precisionBits         = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss")
	disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Whether to disable compression of the data sent from vmstorage to vmselect. This reduces CPU usage at the cost of higher network bandwidth usage")

	denyQueriesOutsideRetention = flag.Bool("denyQueriesOutsideRetention", false, "Whether to deny queries outside of the configured -retentionPeriod. "+
		"When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. "+
		"This may be useful when multiple data sources with distinct retentions are hidden behind query-tee")
)

// Server processes connections from vminsert and vmselect.
type Server struct {
	// Move stopFlag to the top of the struct in order to fix atomic access to it on 32-bit arches.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	stopFlag uint64

	storage *storage.Storage

	vminsertLN net.Listener
	vmselectLN net.Listener

	vminsertWG sync.WaitGroup
	vmselectWG sync.WaitGroup

	vminsertConnsMap ingestserver.ConnsMap
	vmselectConnsMap ingestserver.ConnsMap
}

// NewServer returns new Server.
func NewServer(vminsertAddr, vmselectAddr string, storage *storage.Storage) (*Server, error) {
	vminsertLN, err := netutil.NewTCPListener("vminsert", vminsertAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen vminsertAddr %s: %w", vminsertAddr, err)
	}
	vmselectLN, err := netutil.NewTCPListener("vmselect", vmselectAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen vmselectAddr %s: %w", vmselectAddr, err)
	}
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		return nil, fmt.Errorf("invalid -precisionBits: %w", err)
	}
	s := &Server{
		storage: storage,

		vminsertLN: vminsertLN,
		vmselectLN: vmselectLN,
	}
	s.vminsertConnsMap.Init()
	s.vmselectConnsMap.Init()
	return s, nil
}

// RunVMInsert runs a server accepting connections from vminsert.
func (s *Server) RunVMInsert() {
	logger.Infof("accepting vminsert conns at %s", s.vminsertLN.Addr())
	for {
		c, err := s.vminsertLN.Accept()
		if err != nil {
			if pe, ok := err.(net.Error); ok && pe.Temporary() {
				continue
			}
			if s.isStopping() {
				return
			}
			logger.Panicf("FATAL: cannot process vminsert conns at %s: %s", s.vminsertLN.Addr(), err)
		}
		logger.Infof("accepted vminsert conn from %s", c.RemoteAddr())

		if !s.vminsertConnsMap.Add(c) {
			// The server is closed.
			_ = c.Close()
			return
		}
		vminsertConns.Inc()
		s.vminsertWG.Add(1)
		go func() {
			defer func() {
				s.vminsertConnsMap.Delete(c)
				vminsertConns.Dec()
				s.vminsertWG.Done()
			}()

			// There is no need in response compression, since
			// vmstorage sends only small packets to vminsert.
			compressionLevel := 0
			bc, err := handshake.VMInsertServer(c, compressionLevel)
			if err != nil {
				if s.isStopping() {
					// c is stopped inside Server.MustClose
					return
				}
				logger.Errorf("cannot perform vminsert handshake with client %q: %s", c.RemoteAddr(), err)
				_ = c.Close()
				return
			}
			defer func() {
				if !s.isStopping() {
					logger.Infof("closing vminsert conn from %s", c.RemoteAddr())
				}
				_ = bc.Close()
			}()

			logger.Infof("processing vminsert conn from %s", c.RemoteAddr())
			if err := s.processVMInsertConn(bc); err != nil {
				if s.isStopping() {
					return
				}
				vminsertConnErrors.Inc()
				logger.Errorf("cannot process vminsert conn from %s: %s", c.RemoteAddr(), err)
			}
		}()
	}
}

var (
	vminsertConns      = metrics.NewCounter("vm_vminsert_conns")
	vminsertConnErrors = metrics.NewCounter("vm_vminsert_conn_errors_total")
)

// RunVMSelect runs a server accepting connections from vmselect.
func (s *Server) RunVMSelect() {
	logger.Infof("accepting vmselect conns at %s", s.vmselectLN.Addr())
	for {
		c, err := s.vmselectLN.Accept()
		if err != nil {
			if pe, ok := err.(net.Error); ok && pe.Temporary() {
				continue
			}
			if s.isStopping() {
				return
			}
			logger.Panicf("FATAL: cannot process vmselect conns at %s: %s", s.vmselectLN.Addr(), err)
		}
		logger.Infof("accepted vmselect conn from %s", c.RemoteAddr())

		if !s.vmselectConnsMap.Add(c) {
			// The server is closed.
			_ = c.Close()
			return
		}
		vmselectConns.Inc()
		s.vmselectWG.Add(1)
		go func() {
			defer func() {
				s.vmselectConnsMap.Delete(c)
				vmselectConns.Dec()
				s.vmselectWG.Done()
			}()

			// Compress responses to vmselect even if they already contain compressed blocks.
			// Responses contain uncompressed metric names, which should compress well
			// when the response contains high number of time series.
			// Additionally, recently added metric blocks are usually uncompressed, so the compression
			// should save network bandwidth.
			compressionLevel := 1
			if *disableRPCCompression {
				compressionLevel = 0
			}
			bc, err := handshake.VMSelectServer(c, compressionLevel)
			if err != nil {
				if s.isStopping() {
					// c is closed inside Server.MustClose
					return
				}
				logger.Errorf("cannot perform vmselect handshake with client %q: %s", c.RemoteAddr(), err)
				_ = c.Close()
				return
			}

			defer func() {
				if !s.isStopping() {
					logger.Infof("closing vmselect conn from %s", c.RemoteAddr())
				}
				_ = bc.Close()
			}()

			logger.Infof("processing vmselect conn from %s", c.RemoteAddr())
			if err := s.processVMSelectConn(bc); err != nil {
				if s.isStopping() {
					return
				}
				vmselectConnErrors.Inc()
				logger.Errorf("cannot process vmselect conn %s: %s", c.RemoteAddr(), err)
			}
		}()
	}
}

var (
	vmselectConns      = metrics.NewCounter("vm_vmselect_conns")
	vmselectConnErrors = metrics.NewCounter("vm_vmselect_conn_errors_total")
)

// MustClose gracefully closes the server,
// so it no longer touches s.storage after returning.
func (s *Server) MustClose() {
	// Mark the server as stoping.
	s.setIsStopping()

	// Stop accepting new connections from vminsert and vmselect.
	if err := s.vminsertLN.Close(); err != nil {
		logger.Panicf("FATAL: cannot close vminsert listener: %s", err)
	}
	if err := s.vmselectLN.Close(); err != nil {
		logger.Panicf("FATAL: cannot close vmselect listener: %s", err)
	}

	// Close existing connections from vminsert, so the goroutines
	// processing these connections are finished.
	s.vminsertConnsMap.CloseAll()

	// Close existing connections from vmselect, so the goroutines
	// processing these connections are finished.
	s.vmselectConnsMap.CloseAll()

	// Wait until all the goroutines processing vminsert and vmselect conns
	// are finished.
	s.vminsertWG.Wait()
	s.vmselectWG.Wait()
}

func (s *Server) setIsStopping() {
	atomic.StoreUint64(&s.stopFlag, 1)
}

func (s *Server) isStopping() bool {
	return atomic.LoadUint64(&s.stopFlag) != 0
}

func (s *Server) processVMInsertConn(bc *handshake.BufferedConn) error {
	return clusternative.ParseStream(bc, func(rows []storage.MetricRow) error {
		vminsertMetricsRead.Add(len(rows))
		return s.storage.AddRows(rows, uint8(*precisionBits))
	}, s.storage.IsReadOnly)
}

var vminsertMetricsRead = metrics.NewCounter("vm_vminsert_metrics_read_total")

func (s *Server) processVMSelectConn(bc *handshake.BufferedConn) error {
	ctx := &vmselectRequestCtx{
		bc:      bc,
		sizeBuf: make([]byte, 8),
	}
	for {
		if err := s.processVMSelectRequest(ctx); err != nil {
			if err == io.EOF {
				// Remote client gracefully closed the connection.
				return nil
			}
			if errors.Is(err, storage.ErrDeadlineExceeded) {
				return fmt.Errorf("cannot process vmselect request in %d seconds: %w", ctx.timeout, err)
			}
			return fmt.Errorf("cannot process vmselect request: %w", err)
		}
		if err := bc.Flush(); err != nil {
			return fmt.Errorf("cannot flush compressed buffers: %w", err)
		}
	}
}

type vmselectRequestCtx struct {
	bc      *handshake.BufferedConn
	sizeBuf []byte
	dataBuf []byte

	qt   *querytracer.Tracer
	sq   storage.SearchQuery
	tfss []*storage.TagFilters
	sr   storage.Search
	mb   storage.MetricBlock

	// timeout in seconds for the current request
	timeout uint64

	// deadline in unix timestamp seconds for the current request.
	deadline uint64
}

func (ctx *vmselectRequestCtx) readTimeRange() (storage.TimeRange, error) {
	var tr storage.TimeRange
	minTimestamp, err := ctx.readUint64()
	if err != nil {
		return tr, fmt.Errorf("cannot read minTimestamp: %w", err)
	}
	maxTimestamp, err := ctx.readUint64()
	if err != nil {
		return tr, fmt.Errorf("cannot read maxTimestamp: %w", err)
	}
	tr.MinTimestamp = int64(minTimestamp)
	tr.MaxTimestamp = int64(maxTimestamp)
	return tr, nil
}

func (ctx *vmselectRequestCtx) readUint32() (uint32, error) {
	ctx.sizeBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.sizeBuf, 4)
	if _, err := io.ReadFull(ctx.bc, ctx.sizeBuf); err != nil {
		if err == io.EOF {
			return 0, err
		}
		return 0, fmt.Errorf("cannot read uint32: %w", err)
	}
	n := encoding.UnmarshalUint32(ctx.sizeBuf)
	return n, nil
}

func (ctx *vmselectRequestCtx) readUint64() (uint64, error) {
	ctx.sizeBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.sizeBuf, 8)
	if _, err := io.ReadFull(ctx.bc, ctx.sizeBuf); err != nil {
		if err == io.EOF {
			return 0, err
		}
		return 0, fmt.Errorf("cannot read uint64: %w", err)
	}
	n := encoding.UnmarshalUint64(ctx.sizeBuf)
	return n, nil
}

func (ctx *vmselectRequestCtx) readAccountIDProjectID() (uint32, uint32, error) {
	accountID, err := ctx.readUint32()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read accountID: %w", err)
	}
	projectID, err := ctx.readUint32()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read projectID: %w", err)
	}
	return accountID, projectID, nil
}

func (ctx *vmselectRequestCtx) readSearchQuery() error {
	if err := ctx.readDataBufBytes(maxSearchQuerySize); err != nil {
		return fmt.Errorf("cannot read searchQuery: %w", err)
	}
	tail, err := ctx.sq.Unmarshal(ctx.dataBuf)
	if err != nil {
		return fmt.Errorf("cannot unmarshal SearchQuery: %w", err)
	}
	if len(tail) > 0 {
		return fmt.Errorf("unexpected non-zero tail left after unmarshaling SearchQuery: (len=%d) %q", len(tail), tail)
	}
	return nil
}

func (ctx *vmselectRequestCtx) readDataBufBytes(maxDataSize int) error {
	ctx.sizeBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.sizeBuf, 8)
	if _, err := io.ReadFull(ctx.bc, ctx.sizeBuf); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("cannot read data size: %w", err)
	}
	dataSize := encoding.UnmarshalUint64(ctx.sizeBuf)
	if dataSize > uint64(maxDataSize) {
		return fmt.Errorf("too big data size: %d; it mustn't exceed %d bytes", dataSize, maxDataSize)
	}
	ctx.dataBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.dataBuf, int(dataSize))
	if dataSize == 0 {
		return nil
	}
	if n, err := io.ReadFull(ctx.bc, ctx.dataBuf); err != nil {
		return fmt.Errorf("cannot read data with size %d: %w; read only %d bytes", dataSize, err, n)
	}
	return nil
}

func (ctx *vmselectRequestCtx) readBool() (bool, error) {
	ctx.dataBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.dataBuf, 1)
	if _, err := io.ReadFull(ctx.bc, ctx.dataBuf); err != nil {
		if err == io.EOF {
			return false, err
		}
		return false, fmt.Errorf("cannot read bool: %w", err)
	}
	v := ctx.dataBuf[0] != 0
	return v, nil
}

func (ctx *vmselectRequestCtx) readByte() (byte, error) {
	ctx.dataBuf = bytesutil.ResizeNoCopyMayOverallocate(ctx.dataBuf, 1)
	if _, err := io.ReadFull(ctx.bc, ctx.dataBuf); err != nil {
		if err == io.EOF {
			return 0, err
		}
		return 0, fmt.Errorf("cannot read byte: %w", err)
	}
	b := ctx.dataBuf[0]
	return b, nil
}

func (ctx *vmselectRequestCtx) writeDataBufBytes() error {
	if err := ctx.writeUint64(uint64(len(ctx.dataBuf))); err != nil {
		return fmt.Errorf("cannot write data size: %w", err)
	}
	if len(ctx.dataBuf) == 0 {
		return nil
	}
	if _, err := ctx.bc.Write(ctx.dataBuf); err != nil {
		return fmt.Errorf("cannot write data with size %d: %w", len(ctx.dataBuf), err)
	}
	return nil
}

// maxErrorMessageSize is the maximum size of error message to send to clients.
const maxErrorMessageSize = 64 * 1024

func (ctx *vmselectRequestCtx) writeErrorMessage(err error) error {
	if errors.Is(err, storage.ErrDeadlineExceeded) {
		err = fmt.Errorf("cannot execute request in %d seconds: %w", ctx.timeout, err)
	}
	errMsg := err.Error()
	if len(errMsg) > maxErrorMessageSize {
		// Trim too long error message.
		errMsg = errMsg[:maxErrorMessageSize]
	}
	if err := ctx.writeString(errMsg); err != nil {
		return fmt.Errorf("cannot send error message %q to client: %w", errMsg, err)
	}
	return nil
}

func (ctx *vmselectRequestCtx) writeString(s string) error {
	ctx.dataBuf = append(ctx.dataBuf[:0], s...)
	return ctx.writeDataBufBytes()
}

func (ctx *vmselectRequestCtx) writeUint64(n uint64) error {
	ctx.sizeBuf = encoding.MarshalUint64(ctx.sizeBuf[:0], n)
	if _, err := ctx.bc.Write(ctx.sizeBuf); err != nil {
		return fmt.Errorf("cannot write uint64 %d: %w", n, err)
	}
	return nil
}

const maxRPCNameSize = 128

func (s *Server) processVMSelectRequest(ctx *vmselectRequestCtx) error {
	// Read rpcName
	// Do not set deadline on reading rpcName, since it may take a
	// lot of time for idle connection.
	if err := ctx.readDataBufBytes(maxRPCNameSize); err != nil {
		if err == io.EOF {
			// Remote client gracefully closed the connection.
			return err
		}
		return fmt.Errorf("cannot read rpcName: %w", err)
	}
	rpcName := string(ctx.dataBuf)

	// Initialize query tracing.
	traceEnabled, err := ctx.readBool()
	if err != nil {
		return fmt.Errorf("cannot read traceEnabled: %w", err)
	}
	ctx.qt = querytracer.New(traceEnabled)

	// Limit the time required for reading request args.
	if err := ctx.bc.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("cannot set read deadline for reading request args: %w", err)
	}
	defer func() {
		_ = ctx.bc.SetReadDeadline(time.Time{})
	}()

	// Read the timeout for request execution.
	timeout, err := ctx.readUint32()
	if err != nil {
		return fmt.Errorf("cannot read timeout for the request %q: %w", rpcName, err)
	}
	ctx.timeout = uint64(timeout)
	ctx.deadline = fasttime.UnixTimestamp() + uint64(timeout)

	// Process the rpcName call.
	if err := s.processVMSelectRPC(ctx, rpcName); err != nil {
		return err
	}

	// Finish query trace.
	ctx.qt.Donef("%s() at vmstorage", rpcName)
	traceJSON := ctx.qt.ToJSON()
	if err := ctx.writeString(traceJSON); err != nil {
		return fmt.Errorf("cannot send trace with length %d bytes to vmselect: %w", len(traceJSON), err)
	}
	return nil
}

func (s *Server) processVMSelectRPC(ctx *vmselectRequestCtx, rpcName string) error {
	switch rpcName {
	case "search_v6":
		return s.processVMSelectSearch(ctx)
	case "searchMetricNames_v3":
		return s.processVMSelectSearchMetricNames(ctx)
	case "labelValuesOnTimeRange_v3":
		return s.processVMSelectLabelValuesOnTimeRange(ctx)
	case "labelValues_v4":
		return s.processVMSelectLabelValues(ctx)
	case "tagValueSuffixes_v3":
		return s.processVMSelectTagValueSuffixes(ctx)
	case "labelEntries_v4":
		return s.processVMSelectLabelEntries(ctx)
	case "labelsOnTimeRange_v3":
		return s.processVMSelectLabelsOnTimeRange(ctx)
	case "labels_v4":
		return s.processVMSelectLabels(ctx)
	case "seriesCount_v4":
		return s.processVMSelectSeriesCount(ctx)
	case "tsdbStatus_v4":
		return s.processVMSelectTSDBStatus(ctx)
	case "tsdbStatusWithFilters_v3":
		return s.processVMSelectTSDBStatusWithFilters(ctx)
	case "deleteMetrics_v5":
		return s.processVMSelectDeleteMetrics(ctx)
	case "registerMetricNames_v3":
		return s.processVMSelectRegisterMetricNames(ctx)
	default:
		return fmt.Errorf("unsupported rpcName: %q", ctx.dataBuf)
	}
}

const maxMetricNameRawSize = 1024 * 1024
const maxMetricNamesPerRequest = 1024 * 1024

func (s *Server) processVMSelectRegisterMetricNames(ctx *vmselectRequestCtx) error {
	vmselectRegisterMetricNamesRequests.Inc()

	// Read request
	metricsCount, err := ctx.readUint64()
	if err != nil {
		return fmt.Errorf("cannot read metricsCount: %w", err)
	}
	if metricsCount > maxMetricNamesPerRequest {
		return fmt.Errorf("too many metric names in a single request; got %d; mustn't exceed %d", metricsCount, maxMetricNamesPerRequest)
	}
	mrs := make([]storage.MetricRow, metricsCount)
	for i := 0; i < int(metricsCount); i++ {
		if err := ctx.readDataBufBytes(maxMetricNameRawSize); err != nil {
			return fmt.Errorf("cannot read metricNameRaw: %w", err)
		}
		mr := &mrs[i]
		mr.MetricNameRaw = append(mr.MetricNameRaw[:0], ctx.dataBuf...)
		n, err := ctx.readUint64()
		if err != nil {
			return fmt.Errorf("cannot read timestamp: %w", err)
		}
		mr.Timestamp = int64(n)
	}

	// Register metric names from mrs.
	if err := s.storage.RegisterMetricNames(mrs); err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}
	return nil
}

const maxTagFiltersSize = 64 * 1024

func (s *Server) processVMSelectDeleteMetrics(ctx *vmselectRequestCtx) error {
	vmselectDeleteMetricsRequests.Inc()

	// Read request
	if err := ctx.readDataBufBytes(maxTagFiltersSize); err != nil {
		return fmt.Errorf("cannot read labelName: %w", err)
	}
	tail, err := ctx.sq.Unmarshal(ctx.dataBuf)
	if err != nil {
		return fmt.Errorf("cannot unmarshal SearchQuery: %w", err)
	}
	if len(tail) > 0 {
		return fmt.Errorf("unexpected non-zero tail left after unmarshaling SearchQuery: (len=%d) %q", len(tail), tail)
	}

	// Setup ctx.tfss
	tr := storage.TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: time.Now().UnixNano() / 1e6,
	}
	if err := ctx.setupTfss(s.storage, tr); err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Delete the given metrics.
	deletedCount, err := s.storage.DeleteMetrics(ctx.tfss)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}
	// Send deletedCount to vmselect.
	if err := ctx.writeUint64(uint64(deletedCount)); err != nil {
		return fmt.Errorf("cannot send deletedCount=%d: %w", deletedCount, err)
	}
	return nil
}

func (s *Server) processVMSelectLabelsOnTimeRange(ctx *vmselectRequestCtx) error {
	vmselectLabelsOnTimeRangeRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}
	tr, err := ctx.readTimeRange()
	if err != nil {
		return err
	}

	// Search for tag keys
	labels, err := s.storage.SearchTagKeysOnTimeRange(accountID, projectID, tr, *maxTagKeysPerSearch, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send labels to vmselect
	for _, label := range labels {
		if len(label) == 0 {
			// Do this substitution in order to prevent clashing with 'end of response' marker.
			label = "__name__"
		}
		if err := ctx.writeString(label); err != nil {
			return fmt.Errorf("cannot write label %q: %w", label, err)
		}
	}

	// Send 'end of response' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}

func (s *Server) processVMSelectLabels(ctx *vmselectRequestCtx) error {
	vmselectLabelsRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}

	// Search for tag keys
	labels, err := s.storage.SearchTagKeys(accountID, projectID, *maxTagKeysPerSearch, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send labels to vmselect
	for _, label := range labels {
		if len(label) == 0 {
			// Do this substitution in order to prevent clashing with 'end of response' marker.
			label = "__name__"
		}
		if err := ctx.writeString(label); err != nil {
			return fmt.Errorf("cannot write label %q: %w", label, err)
		}
	}

	// Send 'end of response' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}

const maxLabelValueSize = 16 * 1024

func (s *Server) processVMSelectLabelValuesOnTimeRange(ctx *vmselectRequestCtx) error {
	vmselectLabelValuesOnTimeRangeRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read labelName: %w", err)
	}
	labelName := string(ctx.dataBuf)
	tr, err := ctx.readTimeRange()
	if err != nil {
		return err
	}

	// Search for tag values
	labelValues, err := s.storage.SearchTagValuesOnTimeRange(accountID, projectID, []byte(labelName), tr, *maxTagValuesPerSearch, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	return writeLabelValues(ctx, labelValues)
}

func (s *Server) processVMSelectLabelValues(ctx *vmselectRequestCtx) error {
	vmselectLabelValuesRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read labelName: %w", err)
	}
	labelName := ctx.dataBuf

	// Search for tag values
	labelValues, err := s.storage.SearchTagValues(accountID, projectID, labelName, *maxTagValuesPerSearch, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	return writeLabelValues(ctx, labelValues)
}

func (s *Server) processVMSelectTagValueSuffixes(ctx *vmselectRequestCtx) error {
	vmselectTagValueSuffixesRequests.Inc()

	// read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}
	tr, err := ctx.readTimeRange()
	if err != nil {
		return err
	}
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read tagKey: %w", err)
	}
	tagKey := append([]byte{}, ctx.dataBuf...)
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read tagValuePrefix: %w", err)
	}
	tagValuePrefix := append([]byte{}, ctx.dataBuf...)
	delimiter, err := ctx.readByte()
	if err != nil {
		return fmt.Errorf("cannot read delimiter: %w", err)
	}

	// Search for tag value suffixes
	suffixes, err := s.storage.SearchTagValueSuffixes(accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, *maxTagValueSuffixesPerSearch, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}
	if len(suffixes) >= *maxTagValueSuffixesPerSearch {
		err := fmt.Errorf("more than -search.maxTagValueSuffixesPerSearch=%d tag value suffixes found "+
			"for tagKey=%q, tagValuePrefix=%q, delimiter=%c on time range %s; "+
			"either narrow down the query or increase -search.maxTagValueSuffixesPerSearch command-line flag value",
			*maxTagValueSuffixesPerSearch, tagKey, tagValuePrefix, delimiter, tr.String())
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send suffixes to vmselect.
	// Suffixes may contain empty string, so prepend suffixes with suffixCount.
	if err := ctx.writeUint64(uint64(len(suffixes))); err != nil {
		return fmt.Errorf("cannot write suffixesCount: %w", err)
	}
	for i, suffix := range suffixes {
		if err := ctx.writeString(suffix); err != nil {
			return fmt.Errorf("cannot write suffix #%d: %w", i+1, err)
		}
	}
	return nil
}

func writeLabelValues(ctx *vmselectRequestCtx, labelValues []string) error {
	for _, labelValue := range labelValues {
		if len(labelValue) == 0 {
			// Skip empty label values, since they have no sense for prometheus.
			continue
		}
		if err := ctx.writeString(labelValue); err != nil {
			return fmt.Errorf("cannot write labelValue %q: %w", labelValue, err)
		}
	}
	// Send 'end of label values' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}

func (s *Server) processVMSelectLabelEntries(ctx *vmselectRequestCtx) error {
	vmselectLabelEntriesRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}

	// Perform the request
	labelEntries, err := s.storage.SearchTagEntries(accountID, projectID, *maxTagKeysPerSearch, *maxTagValuesPerSearch, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send labelEntries to vmselect
	for i := range labelEntries {
		e := &labelEntries[i]
		label := e.Key
		if label == "" {
			// Do this substitution in order to prevent clashing with 'end of response' marker.
			label = "__name__"
		}
		if err := ctx.writeString(label); err != nil {
			return fmt.Errorf("cannot write label %q: %w", label, err)
		}
		if err := writeLabelValues(ctx, e.Values); err != nil {
			return fmt.Errorf("cannot write label values for %q: %w", label, err)
		}
	}

	// Send 'end of response' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}

func (s *Server) processVMSelectSeriesCount(ctx *vmselectRequestCtx) error {
	vmselectSeriesCountRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}

	// Execute the request
	n, err := s.storage.GetSeriesCount(accountID, projectID, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send series count to vmselect.
	if err := ctx.writeUint64(n); err != nil {
		return fmt.Errorf("cannot write series count to vmselect: %w", err)
	}
	return nil
}

func (s *Server) processVMSelectTSDBStatus(ctx *vmselectRequestCtx) error {
	vmselectTSDBStatusRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}
	date, err := ctx.readUint32()
	if err != nil {
		return fmt.Errorf("cannot read date: %w", err)
	}
	topN, err := ctx.readUint32()
	if err != nil {
		return fmt.Errorf("cannot read topN: %w", err)
	}
	maxMetricsUint32, err := ctx.readUint32()
	if err != nil {
		return fmt.Errorf("cannot read MaxMetrics: %w", err)
	}
	maxMetrics := int(maxMetricsUint32)
	if maxMetrics < 0 {
		return fmt.Errorf("too big value for MaxMetrics=%d; must be smaller than 2e9", maxMetricsUint32)
	}

	// Execute the request
	status, err := s.storage.GetTSDBStatusWithFiltersForDate(accountID, projectID, nil, uint64(date), int(topN), maxMetrics, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send status to vmselect.
	if err := writeTopHeapEntries(ctx, status.SeriesCountByMetricName); err != nil {
		return fmt.Errorf("cannot write seriesCountByMetricName to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.LabelValueCountByLabelName); err != nil {
		return fmt.Errorf("cannot write labelValueCountByLabelName to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.SeriesCountByLabelValuePair); err != nil {
		return fmt.Errorf("cannot write seriesCountByLabelValuePair to vmselect: %w", err)
	}
	if err := ctx.writeUint64(status.TotalSeries); err != nil {
		return fmt.Errorf("cannot write totalSeries to vmselect: %w", err)
	}
	if err := ctx.writeUint64(status.TotalLabelValuePairs); err != nil {
		return fmt.Errorf("cannot write totalLabelValuePairs to vmselect: %w", err)
	}
	return nil
}

func (s *Server) processVMSelectTSDBStatusWithFilters(ctx *vmselectRequestCtx) error {
	vmselectTSDBStatusWithFiltersRequests.Inc()

	// Read request
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}
	topN, err := ctx.readUint32()
	if err != nil {
		return fmt.Errorf("cannot read topN: %w", err)
	}

	// Execute the request
	tr := storage.TimeRange{
		MinTimestamp: ctx.sq.MinTimestamp,
		MaxTimestamp: ctx.sq.MaxTimestamp,
	}
	if err := ctx.setupTfss(s.storage, tr); err != nil {
		return ctx.writeErrorMessage(err)
	}
	maxMetrics := ctx.getMaxMetrics()
	date := uint64(ctx.sq.MinTimestamp) / (24 * 3600 * 1000)
	status, err := s.storage.GetTSDBStatusWithFiltersForDate(ctx.sq.AccountID, ctx.sq.ProjectID, ctx.tfss, date, int(topN), maxMetrics, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send status to vmselect.
	if err := writeTopHeapEntries(ctx, status.SeriesCountByMetricName); err != nil {
		return fmt.Errorf("cannot write seriesCountByMetricName to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.LabelValueCountByLabelName); err != nil {
		return fmt.Errorf("cannot write labelValueCountByLabelName to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.SeriesCountByLabelValuePair); err != nil {
		return fmt.Errorf("cannot write seriesCountByLabelValuePair to vmselect: %w", err)
	}
	return nil
}

func writeTopHeapEntries(ctx *vmselectRequestCtx, a []storage.TopHeapEntry) error {
	if err := ctx.writeUint64(uint64(len(a))); err != nil {
		return fmt.Errorf("cannot write topHeapEntries size: %w", err)
	}
	for _, e := range a {
		if err := ctx.writeString(e.Name); err != nil {
			return fmt.Errorf("cannot write topHeapEntry name: %w", err)
		}
		if err := ctx.writeUint64(e.Count); err != nil {
			return fmt.Errorf("cannot write topHeapEntry count: %w", err)
		}
	}
	return nil
}

// maxSearchQuerySize is the maximum size of SearchQuery packet in bytes.
const maxSearchQuerySize = 1024 * 1024

func (s *Server) processVMSelectSearchMetricNames(ctx *vmselectRequestCtx) error {
	vmselectSearchMetricNamesRequests.Inc()

	// Read request.
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}

	// Search metric names.
	tr := storage.TimeRange{
		MinTimestamp: ctx.sq.MinTimestamp,
		MaxTimestamp: ctx.sq.MaxTimestamp,
	}
	if err := ctx.setupTfss(s.storage, tr); err != nil {
		return ctx.writeErrorMessage(err)
	}
	maxMetrics := ctx.getMaxMetrics()
	mns, err := s.storage.SearchMetricNames(ctx.qt, ctx.tfss, tr, maxMetrics, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send response.
	metricNamesCount := len(mns)
	if err := ctx.writeUint64(uint64(metricNamesCount)); err != nil {
		return fmt.Errorf("cannot send metricNamesCount: %w", err)
	}
	for i, mn := range mns {
		ctx.dataBuf = mn.Marshal(ctx.dataBuf[:0])
		if err := ctx.writeDataBufBytes(); err != nil {
			return fmt.Errorf("cannot send metricName #%d: %w", i+1, err)
		}
	}
	ctx.qt.Printf("sent %d series to vmselect", len(mns))
	return nil
}

func (s *Server) processVMSelectSearch(ctx *vmselectRequestCtx) error {
	vmselectSearchRequests.Inc()

	// Read request.
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}
	fetchData, err := ctx.readBool()
	if err != nil {
		return fmt.Errorf("cannot read `fetchData` bool: %w", err)
	}

	// Setup search.
	tr := storage.TimeRange{
		MinTimestamp: ctx.sq.MinTimestamp,
		MaxTimestamp: ctx.sq.MaxTimestamp,
	}
	if err := ctx.setupTfss(s.storage, tr); err != nil {
		return ctx.writeErrorMessage(err)
	}
	if err := checkTimeRange(s.storage, tr); err != nil {
		return ctx.writeErrorMessage(err)
	}
	startTime := time.Now()
	maxMetrics := ctx.getMaxMetrics()
	ctx.sr.Init(ctx.qt, s.storage, ctx.tfss, tr, maxMetrics, ctx.deadline)
	indexSearchDuration.UpdateDuration(startTime)
	defer ctx.sr.MustClose()
	if err := ctx.sr.Error(); err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send found blocks to vmselect.
	blocksRead := 0
	for ctx.sr.NextMetricBlock() {
		blocksRead++
		ctx.mb.MetricName = ctx.sr.MetricBlockRef.MetricName
		ctx.sr.MetricBlockRef.BlockRef.MustReadBlock(&ctx.mb.Block, fetchData)

		vmselectMetricBlocksRead.Inc()
		vmselectMetricRowsRead.Add(ctx.mb.Block.RowsCount())

		ctx.dataBuf = ctx.mb.Marshal(ctx.dataBuf[:0])
		if err := ctx.writeDataBufBytes(); err != nil {
			return fmt.Errorf("cannot send MetricBlock: %w", err)
		}
	}
	if err := ctx.sr.Error(); err != nil {
		return fmt.Errorf("search error: %w", err)
	}
	ctx.qt.Printf("sent %d blocks to vmselect", blocksRead)

	// Send 'end of response' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}

var indexSearchDuration = metrics.NewHistogram(`vm_index_search_duration_seconds`)

// checkTimeRange returns true if the given tr is denied for querying.
func checkTimeRange(s *storage.Storage, tr storage.TimeRange) error {
	if !*denyQueriesOutsideRetention {
		return nil
	}
	retentionMsecs := s.RetentionMsecs()
	minAllowedTimestamp := int64(fasttime.UnixTimestamp()*1000) - retentionMsecs
	if tr.MinTimestamp > minAllowedTimestamp {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err: fmt.Errorf("the given time range %s is outside the allowed retention %.3f days according to -denyQueriesOutsideRetention",
			&tr, float64(retentionMsecs)/(24*3600*1000)),
		StatusCode: http.StatusServiceUnavailable,
	}
}

var (
	vmselectRegisterMetricNamesRequests    = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="register_metric_names"}`)
	vmselectDeleteMetricsRequests          = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="delete_metrics"}`)
	vmselectLabelsOnTimeRangeRequests      = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="labels_on_time_range"}`)
	vmselectLabelsRequests                 = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="labels"}`)
	vmselectLabelValuesOnTimeRangeRequests = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="label_values_on_time_range"}`)
	vmselectLabelValuesRequests            = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="label_values"}`)
	vmselectTagValueSuffixesRequests       = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="tag_value_suffixes"}`)
	vmselectLabelEntriesRequests           = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="label_entries"}`)
	vmselectSeriesCountRequests            = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="series_count"}`)
	vmselectTSDBStatusRequests             = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="tsdb_status"}`)
	vmselectTSDBStatusWithFiltersRequests  = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="tsdb_status_with_filters"}`)
	vmselectSearchMetricNamesRequests      = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="search_metric_names"}`)
	vmselectSearchRequests                 = metrics.NewCounter(`vm_vmselect_rpc_requests_total{name="search"}`)

	vmselectMetricBlocksRead = metrics.NewCounter(`vm_vmselect_metric_blocks_read_total`)
	vmselectMetricRowsRead   = metrics.NewCounter(`vm_vmselect_metric_rows_read_total`)
)

func (ctx *vmselectRequestCtx) getMaxMetrics() int {
	maxMetrics := ctx.sq.MaxMetrics
	maxMetricsLimit := *maxMetricsPerSearch
	if maxMetricsLimit <= 0 {
		maxMetricsLimit = 2e9
	}
	if maxMetrics <= 0 || maxMetrics > maxMetricsLimit {
		maxMetrics = maxMetricsLimit
	}
	return maxMetrics
}

func (ctx *vmselectRequestCtx) setupTfss(s *storage.Storage, tr storage.TimeRange) error {
	tfss := ctx.tfss[:0]
	accountID := ctx.sq.AccountID
	projectID := ctx.sq.ProjectID
	for _, tagFilters := range ctx.sq.TagFilterss {
		tfs := storage.NewTagFilters(accountID, projectID)
		for i := range tagFilters {
			tf := &tagFilters[i]
			if string(tf.Key) == "__graphite__" {
				query := tf.Value
				maxMetrics := ctx.getMaxMetrics()
				paths, err := s.SearchGraphitePaths(accountID, projectID, tr, query, maxMetrics, ctx.deadline)
				if err != nil {
					return fmt.Errorf("error when searching for Graphite paths for query %q: %w", query, err)
				}
				if len(paths) >= maxMetrics {
					return fmt.Errorf("more than %d time series match Graphite query %q; "+
						"either narrow down the query or increase the corresponding -search.max* command-line flag value at vmselect nodes", maxMetrics, query)
				}
				tfs.AddGraphiteQuery(query, paths, tf.IsNegative)
				continue
			}
			if err := tfs.Add(tf.Key, tf.Value, tf.IsNegative, tf.IsRegexp); err != nil {
				return fmt.Errorf("cannot parse tag filter %s: %w", tf, err)
			}
		}
		tfss = append(tfss, tfs)
	}
	ctx.tfss = tfss
	return nil
}
