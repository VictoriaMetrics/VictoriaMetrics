package internalselect

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage/netselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

// RequestHandler processes requests to /internal/select/*
func RequestHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	path := r.URL.Path
	rh := requestHandlers[path]
	if rh == nil {
		httpserver.Errorf(w, r, "unsupported endpoint requested: %s", path)
		return
	}

	metrics.GetOrCreateCounter(fmt.Sprintf(`vl_http_requests_total{path=%q}`, path)).Inc()
	if err := rh(ctx, w, r); err != nil && !netutil.IsTrivialNetworkError(err) {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vl_http_request_errors_total{path=%q}`, path)).Inc()
		httpserver.Errorf(w, r, "%s", err)
		// The return is skipped intentionally in order to track the duration of failed queries.
	}
	metrics.GetOrCreateSummary(fmt.Sprintf(`vl_http_request_duration_seconds{path=%q}`, path)).UpdateDuration(startTime)
}

var requestHandlers = map[string]func(ctx context.Context, w http.ResponseWriter, r *http.Request) error{
	"/internal/select/query":               processQueryRequest,
	"/internal/select/field_names":         processFieldNamesRequest,
	"/internal/select/field_values":        processFieldValuesRequest,
	"/internal/select/stream_field_names":  processStreamFieldNamesRequest,
	"/internal/select/stream_field_values": processStreamFieldValuesRequest,
	"/internal/select/streams":             processStreamsRequest,
	"/internal/select/stream_ids":          processStreamIDsRequest,
}

func processQueryRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.QueryProtocolVersion)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	var wLock sync.Mutex
	var dataLenBuf []byte

	sendBuf := func(bb *bytesutil.ByteBuffer) error {
		if len(bb.B) == 0 {
			return nil
		}

		data := bb.B
		if !cp.DisableCompression {
			bufLen := len(bb.B)
			bb.B = zstd.CompressLevel(bb.B, bb.B, 1)
			data = bb.B[bufLen:]
		}

		wLock.Lock()
		dataLenBuf = encoding.MarshalUint64(dataLenBuf[:0], uint64(len(data)))
		_, err := w.Write(dataLenBuf)
		if err == nil {
			_, err = w.Write(data)
		}
		wLock.Unlock()

		// Reset the sent buf
		bb.Reset()

		return err
	}

	var bufs atomicutil.Slice[bytesutil.ByteBuffer]

	var errGlobalLock sync.Mutex
	var errGlobal error

	writeBlock := func(workerID uint, db *logstorage.DataBlock) {
		if errGlobal != nil {
			return
		}

		bb := bufs.Get(workerID)

		bb.B = db.Marshal(bb.B)

		if len(bb.B) < 1024*1024 {
			// Fast path - the bb is too small to be sent to the client yet.
			return
		}

		// Slow path - the bb must be sent to the client.
		if err := sendBuf(bb); err != nil {
			errGlobalLock.Lock()
			if errGlobal != nil {
				errGlobal = err
			}
			errGlobalLock.Unlock()
		}
	}

	if err := vlstorage.RunQuery(ctx, cp.TenantIDs, cp.Query, writeBlock); err != nil {
		return err
	}
	if errGlobal != nil {
		return errGlobal
	}

	// Send the remaining data
	for _, bb := range bufs.All() {
		if err := sendBuf(bb); err != nil {
			return err
		}
	}

	return nil
}

func processFieldNamesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.FieldNamesProtocolVersion)
	if err != nil {
		return err
	}

	fieldNames, err := vlstorage.GetFieldNames(ctx, cp.TenantIDs, cp.Query)
	if err != nil {
		return fmt.Errorf("cannot obtain field names: %w", err)
	}

	return writeValuesWithHits(w, fieldNames, cp.DisableCompression)
}

func processFieldValuesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.FieldValuesProtocolVersion)
	if err != nil {
		return err
	}

	fieldName := r.FormValue("field")

	limit, err := getInt64FromRequest(r, "limit")
	if err != nil {
		return err
	}

	fieldValues, err := vlstorage.GetFieldValues(ctx, cp.TenantIDs, cp.Query, fieldName, uint64(limit))
	if err != nil {
		return fmt.Errorf("cannot obtain field values: %w", err)
	}

	return writeValuesWithHits(w, fieldValues, cp.DisableCompression)
}

func processStreamFieldNamesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.StreamFieldNamesProtocolVersion)
	if err != nil {
		return err
	}

	fieldNames, err := vlstorage.GetStreamFieldNames(ctx, cp.TenantIDs, cp.Query)
	if err != nil {
		return fmt.Errorf("cannot obtain stream field names: %w", err)
	}

	return writeValuesWithHits(w, fieldNames, cp.DisableCompression)
}

func processStreamFieldValuesRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.StreamFieldValuesProtocolVersion)
	if err != nil {
		return err
	}

	fieldName := r.FormValue("field")

	limit, err := getInt64FromRequest(r, "limit")
	if err != nil {
		return err
	}

	fieldValues, err := vlstorage.GetStreamFieldValues(ctx, cp.TenantIDs, cp.Query, fieldName, uint64(limit))
	if err != nil {
		return fmt.Errorf("cannot obtain stream field values: %w", err)
	}

	return writeValuesWithHits(w, fieldValues, cp.DisableCompression)
}

func processStreamsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.StreamsProtocolVersion)
	if err != nil {
		return err
	}

	limit, err := getInt64FromRequest(r, "limit")
	if err != nil {
		return err
	}

	streams, err := vlstorage.GetStreams(ctx, cp.TenantIDs, cp.Query, uint64(limit))
	if err != nil {
		return fmt.Errorf("cannot obtain streams: %w", err)
	}

	return writeValuesWithHits(w, streams, cp.DisableCompression)
}

func processStreamIDsRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cp, err := getCommonParams(r, netselect.StreamIDsProtocolVersion)
	if err != nil {
		return err
	}

	limit, err := getInt64FromRequest(r, "limit")
	if err != nil {
		return err
	}

	streamIDs, err := vlstorage.GetStreamIDs(ctx, cp.TenantIDs, cp.Query, uint64(limit))
	if err != nil {
		return fmt.Errorf("cannot obtain streams: %w", err)
	}

	return writeValuesWithHits(w, streamIDs, cp.DisableCompression)
}

type commonParams struct {
	TenantIDs []logstorage.TenantID
	Query     *logstorage.Query

	DisableCompression bool
}

func getCommonParams(r *http.Request, expectedProtocolVersion string) (*commonParams, error) {
	version := r.FormValue("version")
	if version != expectedProtocolVersion {
		return nil, fmt.Errorf("unexpected version=%q; want %q", version, expectedProtocolVersion)
	}

	tenantIDsStr := r.FormValue("tenant_ids")
	tenantIDs, err := logstorage.UnmarshalTenantIDs([]byte(tenantIDsStr))
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal tenant_ids=%q: %w", tenantIDsStr, err)
	}

	timestamp, err := getInt64FromRequest(r, "timestamp")
	if err != nil {
		return nil, err
	}

	qStr := r.FormValue("query")
	q, err := logstorage.ParseQueryAtTimestamp(qStr, timestamp)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal query=%q: %w", qStr, err)
	}

	s := r.FormValue("disable_compression")
	disableCompression, err := strconv.ParseBool(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse disable_compression=%q: %w", s, err)
	}

	cp := &commonParams{
		TenantIDs: tenantIDs,
		Query:     q,

		DisableCompression: disableCompression,
	}
	return cp, nil
}

func writeValuesWithHits(w http.ResponseWriter, vhs []logstorage.ValueWithHits, disableCompression bool) error {
	var b []byte
	for i := range vhs {
		b = vhs[i].Marshal(b)
	}

	if !disableCompression {
		b = zstd.CompressLevel(nil, b, 1)
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("cannot send response to the client: %w", err)
	}

	return nil
}

func getInt64FromRequest(r *http.Request, argName string) (int64, error) {
	s := r.FormValue(argName)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %s=%q: %w", argName, s, err)
	}
	return n, nil
}
