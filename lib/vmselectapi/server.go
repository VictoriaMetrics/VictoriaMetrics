package vmselectapi

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ingestserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

// Server processes vmselect requests.
type Server struct {
	// api contains the implementation of the server API for vmselect requests.
	api API

	// limits contains various limits for the Server.
	limits Limits

	// disableResponseCompression controls whether vmselect server must compress responses.
	disableResponseCompression bool

	// ln is the listener for incoming connections to the server.
	ln net.Listener

	// connsMap is a map of currently established connections to the server.
	// It is used for closing the connections when MustStop() is called.
	connsMap ingestserver.ConnsMap

	// wg is used for waiting for worker goroutines to stop when MustStop() is called.
	wg sync.WaitGroup

	// stopFlag is set to true when the server needs to stop.
	stopFlag uint32

	vmselectConns      *metrics.Counter
	vmselectConnErrors *metrics.Counter

	indexSearchDuration *metrics.Histogram

	registerMetricNamesRequests *metrics.Counter
	deleteSeriesRequests        *metrics.Counter
	labelNamesRequests          *metrics.Counter
	labelValuesRequests         *metrics.Counter
	tagValueSuffixesRequests    *metrics.Counter
	seriesCountRequests         *metrics.Counter
	tsdbStatusRequests          *metrics.Counter
	searchMetricNamesRequests   *metrics.Counter
	searchRequests              *metrics.Counter

	metricBlocksRead *metrics.Counter
	metricRowsRead   *metrics.Counter
}

// Limits contains various limits for Server.
type Limits struct {
	// MaxLabelNames is the maximum label names, which may be returned from labelNames request.
	MaxLabelNames int

	// MaxLabelValues is the maximum label values, which may be returned from labelValues request.
	MaxLabelValues int

	// MaxTagValueSuffixes is the maximum number of entries, which can be returned from tagValueSuffixes request.
	MaxTagValueSuffixes int
}

// NewServer starts new Server at the given addr, which serves the given api with the given limits.
//
// If disableResponseCompression is set to true, then the returned server doesn't compress responses.
func NewServer(addr string, api API, limits Limits, disableResponseCompression bool) (*Server, error) {
	ln, err := netutil.NewTCPListener("vmselect", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to listen vmselectAddr %s: %w", addr, err)
	}
	s := &Server{
		api:    api,
		limits: limits,
		ln:     ln,

		vmselectConns:      metrics.NewCounter(fmt.Sprintf(`vm_vmselect_conns{addr=%q}`, addr)),
		vmselectConnErrors: metrics.NewCounter(fmt.Sprintf(`vm_vmselect_conn_errors_total{addr=%q}`, addr)),

		indexSearchDuration: metrics.NewHistogram(fmt.Sprintf(`vm_index_search_duration_seconds{addr=%q}`, addr)),

		registerMetricNamesRequests: metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="registerMetricNames",addr=%q}`, addr)),
		deleteSeriesRequests:        metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="deleteSeries",addr=%q}`, addr)),
		labelNamesRequests:          metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="labelNames",addr=%q}`, addr)),
		labelValuesRequests:         metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="labelValues",addr=%q}`, addr)),
		tagValueSuffixesRequests:    metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="tagValueSuffixes",addr=%q}`, addr)),
		seriesCountRequests:         metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="seriesSount",addr=%q}`, addr)),
		tsdbStatusRequests:          metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="tsdbStatus",addr=%q}`, addr)),
		searchMetricNamesRequests:   metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="searchMetricNames",addr=%q}`, addr)),
		searchRequests:              metrics.NewCounter(fmt.Sprintf(`vm_vmselect_rpc_requests_total{action="search",addr=%q}`, addr)),

		metricBlocksRead: metrics.NewCounter(fmt.Sprintf(`vm_vmselect_metric_blocks_read_total{addr=%q}`, addr)),
		metricRowsRead:   metrics.NewCounter(fmt.Sprintf(`vm_vmselect_metric_rows_read_total{addr=%q}`, addr)),
	}
	s.connsMap.Init()
	s.wg.Add(1)
	go func() {
		s.run()
		s.wg.Done()
	}()
	return s, nil
}

func (s *Server) run() {
	logger.Infof("accepting vmselect conns at %s", s.ln.Addr())
	for {
		c, err := s.ln.Accept()
		if err != nil {
			if pe, ok := err.(net.Error); ok && pe.Temporary() {
				continue
			}
			if s.isStopping() {
				return
			}
			logger.Panicf("FATAL: cannot process vmselect conns at %s: %s", s.ln.Addr(), err)
		}
		logger.Infof("accepted vmselect conn from %s", c.RemoteAddr())

		if !s.connsMap.Add(c) {
			// The server is closed.
			_ = c.Close()
			return
		}
		s.vmselectConns.Inc()
		s.wg.Add(1)
		go func() {
			defer func() {
				s.connsMap.Delete(c)
				s.vmselectConns.Dec()
				s.wg.Done()
			}()

			// Compress responses to vmselect even if they already contain compressed blocks.
			// Responses contain uncompressed metric names, which should compress well
			// when the response contains high number of time series.
			// Additionally, recently added metric blocks are usually uncompressed, so the compression
			// should save network bandwidth.
			compressionLevel := 1
			if s.disableResponseCompression {
				compressionLevel = 0
			}
			bc, err := handshake.VMSelectServer(c, compressionLevel)
			if err != nil {
				if s.isStopping() {
					// c is closed inside Server.MustStop
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
			if err := s.processConn(bc); err != nil {
				if s.isStopping() {
					return
				}
				s.vmselectConnErrors.Inc()
				logger.Errorf("cannot process vmselect conn %s: %s", c.RemoteAddr(), err)
			}
		}()
	}
}

// MustStop gracefully stops s, so it no longer touches s.api after returning.
func (s *Server) MustStop() {
	// Mark the server as stoping.
	s.setIsStopping()

	// Stop accepting new connections from vmselect.
	if err := s.ln.Close(); err != nil {
		logger.Panicf("FATAL: cannot close vmselect listener: %s", err)
	}

	// Close existing connections from vmselect, so the goroutines
	// processing these connections are finished.
	s.connsMap.CloseAll()

	// Wait until all the goroutines processing vmselect conns are finished.
	s.wg.Wait()
}

func (s *Server) setIsStopping() {
	atomic.StoreUint32(&s.stopFlag, 1)
}

func (s *Server) isStopping() bool {
	return atomic.LoadUint32(&s.stopFlag) != 0
}

func (s *Server) processConn(bc *handshake.BufferedConn) error {
	ctx := &vmselectRequestCtx{
		bc:      bc,
		sizeBuf: make([]byte, 8),
	}
	for {
		if err := s.processRequest(ctx); err != nil {
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

	qt *querytracer.Tracer
	sq storage.SearchQuery
	mb storage.MetricBlock

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

func (ctx *vmselectRequestCtx) readLimit() (int, error) {
	n, err := ctx.readUint32()
	if err != nil {
		return 0, fmt.Errorf("cannot read limit: %w", err)
	}
	if n > 1<<31-1 {
		n = 1<<31 - 1
	}
	return int(n), nil
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

// maxSearchQuerySize is the maximum size of SearchQuery packet in bytes.
const maxSearchQuerySize = 1024 * 1024

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

func (s *Server) processRequest(ctx *vmselectRequestCtx) error {
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
	ctx.qt = querytracer.New(traceEnabled, "%s() at vmstorage", rpcName)

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
	if err := s.processRPC(ctx, rpcName); err != nil {
		return fmt.Errorf("cannot execute %q: %s", rpcName, err)
	}

	// Finish query trace.
	ctx.qt.Done()
	traceJSON := ctx.qt.ToJSON()
	if err := ctx.writeString(traceJSON); err != nil {
		return fmt.Errorf("cannot send trace with length %d bytes to vmselect: %w", len(traceJSON), err)
	}
	return nil
}

func (s *Server) processRPC(ctx *vmselectRequestCtx, rpcName string) error {
	switch rpcName {
	case "search_v7":
		return s.processSearch(ctx)
	case "searchMetricNames_v3":
		return s.processSearchMetricNames(ctx)
	case "labelValues_v5":
		return s.processLabelValues(ctx)
	case "tagValueSuffixes_v4":
		return s.processTagValueSuffixes(ctx)
	case "labelNames_v5":
		return s.processLabelNames(ctx)
	case "seriesCount_v4":
		return s.processSeriesCount(ctx)
	case "tsdbStatus_v5":
		return s.processTSDBStatus(ctx)
	case "deleteSeries_v5":
		return s.processDeleteSeries(ctx)
	case "registerMetricNames_v3":
		return s.processRegisterMetricNames(ctx)
	default:
		return fmt.Errorf("unsupported rpcName: %q", rpcName)
	}
}

const maxMetricNameRawSize = 1024 * 1024
const maxMetricNamesPerRequest = 1024 * 1024

func (s *Server) processRegisterMetricNames(ctx *vmselectRequestCtx) error {
	s.registerMetricNamesRequests.Inc()

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
	if err := s.api.RegisterMetricNames(ctx.qt, mrs, ctx.deadline); err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}
	return nil
}

func (s *Server) processDeleteSeries(ctx *vmselectRequestCtx) error {
	s.deleteSeriesRequests.Inc()

	// Read request
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}

	// Execute the request.
	deletedCount, err := s.api.DeleteSeries(ctx.qt, &ctx.sq, ctx.deadline)
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

func (s *Server) processLabelNames(ctx *vmselectRequestCtx) error {
	s.labelNamesRequests.Inc()

	// Read request
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}
	maxLabelNames, err := ctx.readLimit()
	if err != nil {
		return fmt.Errorf("cannot read maxLabelNames: %w", err)
	}
	if maxLabelNames <= 0 || maxLabelNames > s.limits.MaxLabelNames {
		maxLabelNames = s.limits.MaxLabelNames
	}

	// Execute the request
	labelNames, err := s.api.LabelNames(ctx.qt, &ctx.sq, maxLabelNames, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send labelNames to vmselect
	for _, labelName := range labelNames {
		if err := ctx.writeString(labelName); err != nil {
			return fmt.Errorf("cannot write label name %q: %w", labelName, err)
		}
	}
	// Send 'end of response' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}

const maxLabelValueSize = 16 * 1024

func (s *Server) processLabelValues(ctx *vmselectRequestCtx) error {
	s.labelValuesRequests.Inc()

	// Read request
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read labelName: %w", err)
	}
	labelName := string(ctx.dataBuf)
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}
	maxLabelValues, err := ctx.readLimit()
	if err != nil {
		return fmt.Errorf("cannot read maxLabelValues: %w", err)
	}
	if maxLabelValues <= 0 || maxLabelValues > s.limits.MaxLabelValues {
		maxLabelValues = s.limits.MaxLabelValues
	}

	// Execute the request
	labelValues, err := s.api.LabelValues(ctx.qt, &ctx.sq, labelName, maxLabelValues, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send labelValues to vmselect
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

func (s *Server) processTagValueSuffixes(ctx *vmselectRequestCtx) error {
	s.tagValueSuffixesRequests.Inc()

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
	tagKey := string(ctx.dataBuf)
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read tagValuePrefix: %w", err)
	}
	tagValuePrefix := string(ctx.dataBuf)
	delimiter, err := ctx.readByte()
	if err != nil {
		return fmt.Errorf("cannot read delimiter: %w", err)
	}
	maxSuffixes, err := ctx.readLimit()
	if err != nil {
		return fmt.Errorf("cannot read maxTagValueSuffixes: %d", err)
	}
	if maxSuffixes <= 0 || maxSuffixes > s.limits.MaxTagValueSuffixes {
		maxSuffixes = s.limits.MaxTagValueSuffixes
	}

	// Execute the request
	suffixes, err := s.api.TagValueSuffixes(ctx.qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	if len(suffixes) >= s.limits.MaxTagValueSuffixes {
		err := fmt.Errorf("more than %d tag value suffixes found "+
			"for tagKey=%q, tagValuePrefix=%q, delimiter=%c on time range %s; "+
			"either narrow down the query or increase -search.max* command-line flag value; see https://docs.victoriametrics.com/#resource-usage-limits",
			s.limits.MaxTagValueSuffixes, tagKey, tagValuePrefix, delimiter, tr.String())
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

func (s *Server) processSeriesCount(ctx *vmselectRequestCtx) error {
	s.seriesCountRequests.Inc()

	// Read request
	accountID, projectID, err := ctx.readAccountIDProjectID()
	if err != nil {
		return err
	}

	// Execute the request
	n, err := s.api.SeriesCount(ctx.qt, accountID, projectID, ctx.deadline)
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

func (s *Server) processTSDBStatus(ctx *vmselectRequestCtx) error {
	s.tsdbStatusRequests.Inc()

	// Read request
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}
	if err := ctx.readDataBufBytes(maxLabelValueSize); err != nil {
		return fmt.Errorf("cannot read focusLabel: %w", err)
	}
	focusLabel := string(ctx.dataBuf)
	topN, err := ctx.readUint32()
	if err != nil {
		return fmt.Errorf("cannot read topN: %w", err)
	}

	// Execute the request
	status, err := s.api.TSDBStatus(ctx.qt, &ctx.sq, focusLabel, int(topN), ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send an empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send status to vmselect.
	return writeTSDBStatus(ctx, status)
}

func writeTSDBStatus(ctx *vmselectRequestCtx, status *storage.TSDBStatus) error {
	if err := ctx.writeUint64(status.TotalSeries); err != nil {
		return fmt.Errorf("cannot write totalSeries to vmselect: %w", err)
	}
	if err := ctx.writeUint64(status.TotalLabelValuePairs); err != nil {
		return fmt.Errorf("cannot write totalLabelValuePairs to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.SeriesCountByMetricName); err != nil {
		return fmt.Errorf("cannot write seriesCountByMetricName to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.SeriesCountByLabelName); err != nil {
		return fmt.Errorf("cannot write seriesCountByLabelName to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.SeriesCountByFocusLabelValue); err != nil {
		return fmt.Errorf("cannot write seriesCountByFocusLabelValue to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.SeriesCountByLabelValuePair); err != nil {
		return fmt.Errorf("cannot write seriesCountByLabelValuePair to vmselect: %w", err)
	}
	if err := writeTopHeapEntries(ctx, status.LabelValueCountByLabelName); err != nil {
		return fmt.Errorf("cannot write labelValueCountByLabelName to vmselect: %w", err)
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

func (s *Server) processSearchMetricNames(ctx *vmselectRequestCtx) error {
	s.searchMetricNamesRequests.Inc()

	// Read request.
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}

	// Execute request.
	metricNames, err := s.api.SearchMetricNames(ctx.qt, &ctx.sq, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}

	// Send empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send response.
	metricNamesCount := len(metricNames)
	if err := ctx.writeUint64(uint64(metricNamesCount)); err != nil {
		return fmt.Errorf("cannot send metricNamesCount: %w", err)
	}
	for i, metricName := range metricNames {
		if err := ctx.writeString(metricName); err != nil {
			return fmt.Errorf("cannot send metricName #%d: %w", i+1, err)
		}
	}
	ctx.qt.Printf("sent %d series to vmselect", len(metricNames))
	return nil
}

func (s *Server) processSearch(ctx *vmselectRequestCtx) error {
	s.searchRequests.Inc()

	// Read request.
	if err := ctx.readSearchQuery(); err != nil {
		return err
	}

	// Initiaialize the search.
	startTime := time.Now()
	bi, err := s.api.InitSearch(ctx.qt, &ctx.sq, ctx.deadline)
	if err != nil {
		return ctx.writeErrorMessage(err)
	}
	s.indexSearchDuration.UpdateDuration(startTime)
	defer bi.MustClose()

	// Send empty error message to vmselect.
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send empty error message: %w", err)
	}

	// Send found blocks to vmselect.
	blocksRead := 0
	for bi.NextBlock(&ctx.mb) {
		blocksRead++
		s.metricBlocksRead.Inc()
		s.metricRowsRead.Add(ctx.mb.Block.RowsCount())

		ctx.dataBuf = ctx.mb.Marshal(ctx.dataBuf[:0])
		if err := ctx.writeDataBufBytes(); err != nil {
			return fmt.Errorf("cannot send MetricBlock: %w", err)
		}
	}
	if err := bi.Error(); err != nil {
		return fmt.Errorf("search error: %w", err)
	}
	ctx.qt.Printf("sent %d blocks to vmselect", blocksRead)

	// Send 'end of response' marker
	if err := ctx.writeString(""); err != nil {
		return fmt.Errorf("cannot send 'end of response' marker")
	}
	return nil
}
