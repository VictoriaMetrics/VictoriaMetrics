package journald

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
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

func isNameValid(s string) bool {
	if len(s) == 0 || ((s[0] < 'A' || s[0] > 'Z') && s[0] != '_') {
		return false
	}
	for _, r := range s[1:] {
		if (r < 'A' || r > 'Z') && (r < '0' && r > '9') && r != '_' {
			return false
		}
	}
	return true
}

var (
	journaldStreamFields = flagutil.NewArrayString("journald.streamFields", "Comma-separated list of fields to use as log stream fields for logs ingested over journald protocol. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#stream-fields")
	journaldIgnoreFields = flagutil.NewArrayString("journald.ignoreFields", "Comma-separated list of fields to ignore for logs ingested over journald protocol. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#dropping-fields")
	journaldTimeField = flag.String("journald.timeField", "__REALTIME_TIMESTAMP", "Field to use as a log timestamp for logs ingested via journald protocol. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#time-field")
	journaldTenantID = flag.String("journald.tenantID", "0:0", "TenantID for logs ingested via the Journald endpoint. "+
		"See https://docs.victoriametrics.com/victorialogs/data-ingestion/journald/#multitenancy")
	journaldIncludeEntryMetadata = flag.Bool("journald.includeEntryMetadata", false, "Include Journald fields with double underscore prefixes")
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

	if !cp.IsTimeFieldSet {
		cp.TimeFields = []string{*journaldTimeField}
	}
	if len(cp.StreamFields) == 0 {
		cp.StreamFields = getStreamFields()
	}
	if len(cp.IgnoreFields) == 0 {
		cp.IgnoreFields = *journaldIgnoreFields
	}
	cp.MsgFields = []string{"MESSAGE"}
	return cp, nil
}

func getStreamFields() []string {
	if len(*journaldStreamFields) > 0 {
		return *journaldStreamFields
	}
	return defaultStreamFields
}

var defaultStreamFields = []string{
	"_MACHINE_ID",
	"_HOSTNAME",
	"_SYSTEMD_UNIT",
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

func lineFn(buf []byte) (int, int) {
	if len(buf) < 8 {
		return 0, -1
	}
	var size uint64
	offset, err := binary.Decode(buf, binary.LittleEndian, &size)
	if err != nil {
		return 0, -1
	}
	return offset, int(size)
}

var fieldsPool sync.Pool

type fieldsBuf struct {
	fields []logstorage.Field
	buf    []byte
}

func getFieldsBuf() *fieldsBuf {
	b := fieldsPool.Get()
	if b == nil {
		return &fieldsBuf{}
	}
	return b.(*fieldsBuf)
}

func putFieldsBuf(b *fieldsBuf) {
	b.fields = b.fields[:0]
	b.buf = b.buf[:0]
	fieldsPool.Put(b)
}

// See https://systemd.io/JOURNAL_EXPORT_FORMATS/#journal-export-format
func readMessage(lr *insertutil.LineReader, lmp insertutil.LogMessageProcessor, cp *insertutil.CommonParams) (bool, error) {
	var ts int64
	var name, value string
	var hasLine bool
	fb := getFieldsBuf()
	defer putFieldsBuf(fb)

	for {
		if hasLine = lr.NextLine(); !hasLine || len(lr.Line) == 0 {
			break
		}
		line := lr.Line
		idx := bytes.IndexByte(line, '=')
		// could b either e key=value\n pair
		// or just key\n
		// with binary data at the buffer
		if idx > 0 {
			name = bytesutil.ToUnsafeString(line[:idx])
			value = bytesutil.ToUnsafeString(line[idx+1:])
		} else {
			fb.buf = append(fb.buf[:0], line...)
			name = bytesutil.ToUnsafeString(fb.buf)
			if !lr.NextLineWithLineFn(lineFn) {
				err := fmt.Errorf("failed to extract binary field %q value size", name)
				if lr.Err() != nil {
					err = fmt.Errorf("%w: %w", err, lr.Err())
				}
				return true, err
			}
			value = bytesutil.ToUnsafeString(lr.Line)
		}

		if len(name) > journaldEntryMaxNameLen {
			return true, fmt.Errorf("journald entry name should not exceed %d symbols, got: %q", journaldEntryMaxNameLen, name)
		}
		if !isNameValid(name) {
			return true, fmt.Errorf("journald entry name=%q should consist of `A-Z0-9_` characters and must start from non-digit symbol", name)
		}

		if slices.Contains(cp.TimeFields, name) {
			t, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return true, fmt.Errorf("failed to parse Journald timestamp, %w", err)
			}
			ts = t * 1e3
			continue
		}

		if slices.Contains(cp.MsgFields, name) {
			name = "_msg"
		}

		if !strings.HasPrefix(name, "__") || *journaldIncludeEntryMetadata {
			if name == "PRIORITY" {
				fb.fields = append(fb.fields, logstorage.Field{
					Name:  "level",
					Value: journaldPriorityToLevel(value),
				})
			}
			fb.fields = append(fb.fields, logstorage.Field{
				Name:  name,
				Value: value,
			})
		}
	}
	if len(fb.fields) > 0 {
		if ts == 0 {
			ts = time.Now().UnixNano()
		}
		lmp.AddRow(ts, fb.fields, nil)
	}
	return hasLine, nil
}

func journaldPriorityToLevel(priority string) string {
	// See https://wiki.archlinux.org/title/Systemd/Journal#Priority_level
	// and https://grafana.com/docs/grafana/latest/explore/logs-integration/#log-level
	switch priority {
	case "0", "1", "2":
		return "critical"
	case "3":
		return "error"
	case "4":
		return "warning"
	case "5", "6":
		return "info"
	case "7":
		return "debug"
	default:
		return priority
	}
}
