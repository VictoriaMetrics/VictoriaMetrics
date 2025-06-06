package journald

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// See https://github.com/systemd/systemd/blob/main/src/libsystemd/sd-journal/journal-file.c#L1703
const journaldEntryMaxNameLen = 64

var allowedJournaldEntryNameChars = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*`)

var (
	journaldStreamFields = flagutil.NewArrayString("journald.streamFields", "Comma-separated list of fields to use as log stream fields for logs ingested over journald protocol. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#stream-fields")
	journaldIgnoreFields = flagutil.NewArrayString("journald.ignoreFields", "Comma-separated list of fields to ignore for logs ingested over journald protocol. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#dropping-fields")
	journaldTimeField = flag.String("journald.timeField", "__REALTIME_TIMESTAMP", "Field to use as a log timestamp for logs ingested via journald protocol. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#time-field")
	journaldTenantID = flag.String("journald.tenantID", "0:0", "TenantID for logs ingested via the Journald endpoint. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#multitenancy")
	journaldIncludeEntryMetadata = flag.Bool("journald.includeEntryMetadata", false, "Include journal entry fields, which with double underscores.")
)

func getCommonParams(r *http.Request) (*insertutil.CommonParams, error) {
	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		return nil, err
	}
	if cp.TenantID.AccountID == 0 && cp.TenantID.ProjectID == 0 {
		tenantID, err := logstorage.ParseTenantID(*journaldTenantID)
		if err != nil {
			return nil, fmt.Errorf("cannot parse -journald.tenantID=%q for journald: %w", *journaldTenantID, err)
		}
		cp.TenantID = tenantID
	}
	if len(cp.TimeFields) == 0 {
		cp.TimeFields = []string{*journaldTimeField}
	}
	if len(cp.StreamFields) == 0 {
		cp.StreamFields = *journaldStreamFields
	}
	if len(cp.IgnoreFields) == 0 {
		cp.IgnoreFields = *journaldIgnoreFields
	}
	cp.MsgFields = []string{"MESSAGE"}
	return cp, nil
}

// RequestHandler processes Journald Export insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/upload":
		if r.Header.Get("Content-Type") != "application/vnd.fdo.journal" {
			httpserver.Errorf(w, r, "only application/vnd.fdo.journal encoding is supported for Journald")
			return true
		}
		handleJournald(r, w)
		return true
	default:
		return false
	}
}

// handleJournald parses Journal binary entries
func handleJournald(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsTotal.Inc()

	cp, err := getCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}

	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	reader, err := protoparserutil.GetUncompressedReader(r.Body, encoding)
	if err != nil {
		logger.Errorf("cannot decode journald request: %s", err)
		return
	}
	lmp := cp.NewLogMessageProcessor("journald", true)
	streamName := fmt.Sprintf("remoteAddr=%s, requestURI=%q", httpserver.GetQuotedRemoteAddr(r), r.RequestURI)
	err = processStreamInternal(streamName, reader, lmp, cp)
	lmp.MustClose()
	if err != nil {
		httpserver.Errorf(w, r, "cannot read journald protocol data: %s", err)
		return
	}

	// systemd starting release v258 will support compression, which starts working after negotiation: it expects supported compression
	// algorithms list in Accept-Encoding response header in a format "<algorithm_1>[:<priority_1>][;<algorithm_2>:<priority_2>]"
	// See https://github.com/systemd/systemd/pull/34822
	w.Header().Set("Accept-Encoding", "zstd")

	// update requestDuration only for successfully parsed requests
	// There is no need in updating requestDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestDuration.UpdateDuration(startTime)
}

var (
	requestsTotal   = metrics.NewCounter(`vl_http_requests_total{path="/insert/journald/upload"}`)
	errorsTotal     = metrics.NewCounter(`vl_http_errors_total{path="/insert/journald/upload"}`)
	requestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/journald/upload"}`)
)

func processStreamInternal(streamName string, r io.Reader, lmp insertutil.LogMessageProcessor, cp *insertutil.CommonParams) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lr := insertutil.NewLineReader(streamName, wcr)

	n := 0
	errors := 0
	var lastError error
	for {
		ok, err := readMessage(lr, lmp, cp)
		wcr.DecConcurrency()
		if err != nil {
			lastError = err
			errors++
			logger.Warnf("journald: cannot read line #%d in /journald request: %s", n, err)
		}
		if !ok {
			break
		}
		n++
	}
	errorsTotal.Add(errors)

	if errors > 0 && n == errors {
		// Return an error if no logs were processed and there were errors
		return lastError
	}

	return nil
}

// See https://systemd.io/JOURNAL_EXPORT_FORMATS/#journal-export-format
func readMessage(lr *insertutil.LineReader, lmp insertutil.LogMessageProcessor, cp *insertutil.CommonParams) (bool, error) {
	var fields []logstorage.Field
	var ts int64
	var name, value string
	var line []byte
	var hasLine bool

	currentTimestamp := time.Now().UnixNano()

	for {
		if hasLine = lr.NextLine(); !hasLine || len(lr.Line) == 0 {
			break
		}
		line = lr.Line
		idx := bytes.IndexByte(line, '=')
		// could b either e key=value\n pair
		// or just  key\n
		// with binary data at the buffer
		if idx > 0 {
			name = bytesutil.ToUnsafeString(line[:idx])
			value = bytesutil.ToUnsafeString(line[idx+1:])
		} else {
			name = bytesutil.ToUnsafeString(line)
			if !lr.NextLineWithSize() {
				return true, fmt.Errorf("failed to extract binary field %q value size: %w", name, lr.Err())
			}
			value = bytesutil.ToUnsafeString(lr.Line)
		}
		if len(name) > journaldEntryMaxNameLen {
			return true, fmt.Errorf("journald entry name should not exceed %d symbols, got: %q", journaldEntryMaxNameLen, name)
		}
		if !allowedJournaldEntryNameChars.MatchString(name) {
			return true, fmt.Errorf("journald entry name should consist of `A-Z0-9_` characters and must start from non-digit symbol")
		}
		if slices.Contains(cp.TimeFields, name) {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return true, fmt.Errorf("failed to parse Journald timestamp, %w", err)
			}
			ts = n * 1e3
			continue
		}

		if slices.Contains(cp.MsgFields, name) {
			name = "_msg"
		}

		if *journaldIncludeEntryMetadata || !strings.HasPrefix(name, "__") {
			fields = append(fields, logstorage.Field{
				Name:  name,
				Value: value,
			})
		}
	}
	if len(fields) > 0 {
		if ts == 0 {
			ts = currentTimestamp
		}
		lmp.AddRow(ts, fields, nil)
	}
	return hasLine, nil
}
