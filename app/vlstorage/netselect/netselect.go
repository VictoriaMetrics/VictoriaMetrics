package netselect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

const (
	// FieldNamesProtocolVersion is the version of the protocol used for /internal/select/field_names HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	FieldNamesProtocolVersion = "v1"

	// FieldValuesProtocolVersion is the version of the protocol used for /internal/select/field_values HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	FieldValuesProtocolVersion = "v1"

	// StreamFieldNamesProtocolVersion is the version of the protocol used for /internal/select/stream_field_names HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	StreamFieldNamesProtocolVersion = "v1"

	// StreamFieldValuesProtocolVersion is the version of the protocol used for /internal/select/stream_field_values HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	StreamFieldValuesProtocolVersion = "v1"

	// StreamsProtocolVersion is the version of the protocol used for /internal/select/streams HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	StreamsProtocolVersion = "v1"

	// StreamIDsProtocolVersion is the version of the protocol used for /internal/select/stream_ids HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	StreamIDsProtocolVersion = "v1"

	// QueryProtocolVersion is the version of the protocol used for /internal/select/query HTTP endpoint.
	//
	// It must be updated every time the protocol changes.
	QueryProtocolVersion = "v1"
)

// Storage is a network storage for querying remote storage nodes in the cluster.
type Storage struct {
	sns []*storageNode

	disableCompression bool
}

type storageNode struct {
	// scheme is http or https scheme to communicate with addr
	scheme string

	// addr is TCP address of the storage node to query
	addr string

	// s is a storage, which holds the given storageNode
	s *Storage

	// c is an http client used for querying storage node at addr.
	c *http.Client

	// ac is auth config used for setting request headers such as Authorization and Host.
	ac *promauth.Config
}

func newStorageNode(s *Storage, addr string, ac *promauth.Config, isTLS bool) *storageNode {
	tr := httputil.NewTransport(false, "vlselect_backend")
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
	}
	return sn
}

func (sn *storageNode) runQuery(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, processBlock func(db *logstorage.DataBlock)) error {
	args := sn.getCommonArgs(QueryProtocolVersion, tenantIDs, q)

	reqURL := sn.getRequestURL("/internal/select/query", args)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		logger.Panicf("BUG: unexpected error when creating a request: %s", err)
	}
	if err := sn.ac.SetHeaders(req, true); err != nil {
		return fmt.Errorf("cannot set auth headers for %q: %w", reqURL, err)
	}

	// send the request to the storage node
	resp, err := sn.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			responseBody = []byte(err.Error())
		}
		return fmt.Errorf("unexpected status code for the request to %q: %d; want %d; response: %q", reqURL, resp.StatusCode, http.StatusOK, responseBody)
	}

	// read the response
	var dataLenBuf [8]byte
	var buf []byte
	var db logstorage.DataBlock
	var valuesBuf []string
	for {
		if _, err := io.ReadFull(resp.Body, dataLenBuf[:]); err != nil {
			if errors.Is(err, io.EOF) {
				// The end of response stream
				return nil
			}
			return fmt.Errorf("cannot read block size from %q: %w", reqURL, err)
		}
		blockLen := encoding.UnmarshalUint64(dataLenBuf[:])
		if blockLen > math.MaxInt {
			return fmt.Errorf("too big data block: %d bytes; mustn't exceed %v bytes", blockLen, math.MaxInt)
		}

		buf = slicesutil.SetLength(buf, int(blockLen))
		if _, err := io.ReadFull(resp.Body, buf); err != nil {
			return fmt.Errorf("cannot read block with size of %d bytes from %q: %w", blockLen, reqURL, err)
		}

		src := buf
		if !sn.s.disableCompression {
			bufLen := len(buf)
			var err error
			buf, err = zstd.Decompress(buf, buf)
			if err != nil {
				return fmt.Errorf("cannot decompress data block: %w", err)
			}
			src = buf[bufLen:]
		}

		for len(src) > 0 {
			tail, vb, err := db.UnmarshalInplace(src, valuesBuf[:0])
			if err != nil {
				return fmt.Errorf("cannot unmarshal data block received from %q: %w", reqURL, err)
			}
			valuesBuf = vb
			src = tail

			processBlock(&db)

			clear(valuesBuf)
		}
	}
}

func (sn *storageNode) getFieldNames(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query) ([]logstorage.ValueWithHits, error) {
	args := sn.getCommonArgs(FieldNamesProtocolVersion, tenantIDs, q)

	return sn.getValuesWithHits(ctx, "/internal/select/field_names", args)
}

func (sn *storageNode) getFieldValues(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, fieldName string, limit uint64) ([]logstorage.ValueWithHits, error) {
	args := sn.getCommonArgs(FieldValuesProtocolVersion, tenantIDs, q)
	args.Set("field", fieldName)
	args.Set("limit", fmt.Sprintf("%d", limit))

	return sn.getValuesWithHits(ctx, "/internal/select/field_values", args)
}

func (sn *storageNode) getStreamFieldNames(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query) ([]logstorage.ValueWithHits, error) {
	args := sn.getCommonArgs(StreamFieldNamesProtocolVersion, tenantIDs, q)

	return sn.getValuesWithHits(ctx, "/internal/select/stream_field_names", args)
}

func (sn *storageNode) getStreamFieldValues(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, fieldName string, limit uint64) ([]logstorage.ValueWithHits, error) {
	args := sn.getCommonArgs(StreamFieldValuesProtocolVersion, tenantIDs, q)
	args.Set("field", fieldName)
	args.Set("limit", fmt.Sprintf("%d", limit))

	return sn.getValuesWithHits(ctx, "/internal/select/stream_field_values", args)
}

func (sn *storageNode) getStreams(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit uint64) ([]logstorage.ValueWithHits, error) {
	args := sn.getCommonArgs(StreamsProtocolVersion, tenantIDs, q)
	args.Set("limit", fmt.Sprintf("%d", limit))

	return sn.getValuesWithHits(ctx, "/internal/select/streams", args)
}

func (sn *storageNode) getStreamIDs(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit uint64) ([]logstorage.ValueWithHits, error) {
	args := sn.getCommonArgs(StreamIDsProtocolVersion, tenantIDs, q)
	args.Set("limit", fmt.Sprintf("%d", limit))

	return sn.getValuesWithHits(ctx, "/internal/select/stream_ids", args)
}

func (sn *storageNode) getCommonArgs(version string, tenantIDs []logstorage.TenantID, q *logstorage.Query) url.Values {
	args := url.Values{}
	args.Set("version", version)
	args.Set("tenant_ids", string(logstorage.MarshalTenantIDs(nil, tenantIDs)))
	args.Set("query", q.String())
	args.Set("timestamp", fmt.Sprintf("%d", q.GetTimestamp()))
	args.Set("disable_compression", fmt.Sprintf("%v", sn.s.disableCompression))
	return args
}

func (sn *storageNode) getValuesWithHits(ctx context.Context, path string, args url.Values) ([]logstorage.ValueWithHits, error) {
	data, err := sn.executeRequestAt(ctx, path, args)
	if err != nil {
		return nil, err
	}
	return unmarshalValuesWithHits(data)
}

func (sn *storageNode) executeRequestAt(ctx context.Context, path string, args url.Values) ([]byte, error) {
	reqURL := sn.getRequestURL(path, args)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		logger.Panicf("BUG: unexpected error when creating a request: %s", err)
	}
	if err := sn.ac.SetHeaders(req, true); err != nil {
		return nil, fmt.Errorf("cannot set auth headers for %q: %w", reqURL, err)
	}

	// send the request to the storage node
	resp, err := sn.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			responseBody = []byte(err.Error())
		}
		return nil, fmt.Errorf("unexpected status code for the request to %q: %d; want %d; response: %q", reqURL, resp.StatusCode, http.StatusOK, responseBody)
	}

	// read the response
	var bb bytesutil.ByteBuffer
	if _, err := bb.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", reqURL, err)
	}

	if sn.s.disableCompression {
		return bb.B, nil
	}

	bbLen := len(bb.B)
	bb.B, err = zstd.Decompress(bb.B, bb.B)
	if err != nil {
		return nil, err
	}
	return bb.B[bbLen:], nil
}

func (sn *storageNode) getRequestURL(path string, args url.Values) string {
	return fmt.Sprintf("%s://%s%s?%s", sn.scheme, sn.addr, path, args.Encode())
}

// NewStorage returns new Storage for the given addrs and the given authCfgs.
//
// If disableCompression is set, then uncompressed responses are received from storage nodes.
//
// Call MustStop on the returned storage when it is no longer needed.
func NewStorage(addrs []string, authCfgs []*promauth.Config, isTLSs []bool, disableCompression bool) *Storage {
	s := &Storage{
		disableCompression: disableCompression,
	}

	sns := make([]*storageNode, len(addrs))
	for i, addr := range addrs {
		sns[i] = newStorageNode(s, addr, authCfgs[i], isTLSs[i])
	}
	s.sns = sns

	return s
}

// MustStop stops the s.
func (s *Storage) MustStop() {
	s.sns = nil
}

// RunQuery runs the given q and calls writeBlock for the returned data blocks
func (s *Storage) RunQuery(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, writeBlock logstorage.WriteDataBlockFunc) error {
	nqr, err := logstorage.NewNetQueryRunner(ctx, tenantIDs, q, s.RunQuery, writeBlock)
	if err != nil {
		return err
	}

	search := func(stopCh <-chan struct{}, q *logstorage.Query, writeBlock logstorage.WriteDataBlockFunc) error {
		return s.runQuery(stopCh, tenantIDs, q, writeBlock)
	}

	concurrency := q.GetConcurrency()
	return nqr.Run(ctx, concurrency, search)
}

func (s *Storage) runQuery(stopCh <-chan struct{}, tenantIDs []logstorage.TenantID, q *logstorage.Query, writeBlock logstorage.WriteDataBlockFunc) error {
	ctxWithCancel, cancel := contextutil.NewStopChanContext(stopCh)
	defer cancel()

	errs := make([]error, len(s.sns))

	var wg sync.WaitGroup
	for i := range s.sns {
		wg.Add(1)
		go func(nodeIdx int) {
			defer wg.Done()
			sn := s.sns[nodeIdx]
			err := sn.runQuery(ctxWithCancel, tenantIDs, q, func(db *logstorage.DataBlock) {
				writeBlock(uint(nodeIdx), db)
			})
			if err != nil {
				// Cancel the remaining parallel queries
				cancel()
			}

			errs[nodeIdx] = err
		}(i)
	}
	wg.Wait()

	return getFirstNonCancelError(errs)
}

// GetFieldNames executes q and returns field names seen in results.
func (s *Storage) GetFieldNames(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query) ([]logstorage.ValueWithHits, error) {
	return s.getValuesWithHits(ctx, 0, false, func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error) {
		return sn.getFieldNames(ctx, tenantIDs, q)
	})
}

// GetFieldValues executes q and returns unique values for the fieldName seen in results.
//
// If limit > 0, then up to limit unique values are returned.
func (s *Storage) GetFieldValues(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, fieldName string, limit uint64) ([]logstorage.ValueWithHits, error) {
	return s.getValuesWithHits(ctx, limit, true, func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error) {
		return sn.getFieldValues(ctx, tenantIDs, q, fieldName, limit)
	})
}

// GetStreamFieldNames executes q and returns stream field names seen in results.
func (s *Storage) GetStreamFieldNames(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query) ([]logstorage.ValueWithHits, error) {
	return s.getValuesWithHits(ctx, 0, false, func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error) {
		return sn.getStreamFieldNames(ctx, tenantIDs, q)
	})
}

// GetStreamFieldValues executes q and returns stream field values for the given fieldName seen in results.
//
// If limit > 0, then up to limit unique stream field values are returned.
func (s *Storage) GetStreamFieldValues(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, fieldName string, limit uint64) ([]logstorage.ValueWithHits, error) {
	return s.getValuesWithHits(ctx, limit, true, func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error) {
		return sn.getStreamFieldValues(ctx, tenantIDs, q, fieldName, limit)
	})
}

// GetStreams executes q and returns streams seen in query results.
//
// If limit > 0, then up to limit unique streams are returned.
func (s *Storage) GetStreams(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit uint64) ([]logstorage.ValueWithHits, error) {
	return s.getValuesWithHits(ctx, limit, true, func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error) {
		return sn.getStreams(ctx, tenantIDs, q, limit)
	})
}

// GetStreamIDs executes q and returns streamIDs seen in query results.
//
// If limit > 0, then up to limit unique streamIDs are returned.
func (s *Storage) GetStreamIDs(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit uint64) ([]logstorage.ValueWithHits, error) {
	return s.getValuesWithHits(ctx, limit, true, func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error) {
		return sn.getStreamIDs(ctx, tenantIDs, q, limit)
	})
}

func (s *Storage) getValuesWithHits(ctx context.Context, limit uint64, resetHitsOnLimitExceeded bool,
	callback func(ctx context.Context, sn *storageNode) ([]logstorage.ValueWithHits, error)) ([]logstorage.ValueWithHits, error) {

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]logstorage.ValueWithHits, len(s.sns))
	errs := make([]error, len(s.sns))

	var wg sync.WaitGroup
	for i := range s.sns {
		wg.Add(1)
		go func(nodeIdx int) {
			defer wg.Done()

			sn := s.sns[nodeIdx]
			vhs, err := callback(ctxWithCancel, sn)
			results[nodeIdx] = vhs
			errs[nodeIdx] = err

			if err != nil {
				// Cancel the remaining parallel requests
				cancel()
			}
		}(i)
	}
	wg.Wait()

	if err := getFirstNonCancelError(errs); err != nil {
		return nil, err
	}

	vhs := logstorage.MergeValuesWithHits(results, limit, resetHitsOnLimitExceeded)

	return vhs, nil
}

func getFirstNonCancelError(errs []error) error {
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func unmarshalValuesWithHits(src []byte) ([]logstorage.ValueWithHits, error) {
	var vhs []logstorage.ValueWithHits
	for len(src) > 0 {
		var vh logstorage.ValueWithHits
		tail, err := vh.UnmarshalInplace(src)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal ValueWithHits #%d: %w", len(vhs), err)
		}
		src = tail

		// Clone vh.Value, since it points to src.
		vh.Value = strings.Clone(vh.Value)

		vhs = append(vhs, vh)
	}

	return vhs, nil
}
