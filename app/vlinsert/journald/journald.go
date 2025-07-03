package journald

import (
	"bytes"
	"encoding/binary"
	"errors"
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
const maxFieldNameLen = 64

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
	case "/insert/journald/upload":
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
		errorsTotal.Inc()
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}

	if err := insertutil.CanWriteData(); err != nil {
		errorsTotal.Inc()
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	reader, err := protoparserutil.GetUncompressedReader(r.Body, encoding)
	if err != nil {
		errorsTotal.Inc()
		logger.Errorf("cannot decode journald request: %s", err)
		return
	}

	lmp := cp.NewLogMessageProcessor("journald", true)
	streamName := fmt.Sprintf("remoteAddr=%s, requestURI=%q", httpserver.GetQuotedRemoteAddr(r), r.RequestURI)
	err = processStreamInternal(streamName, reader, lmp, cp)
	protoparserutil.PutUncompressedReader(reader)
	lmp.MustClose()
	if err != nil {
		errorsTotal.Inc()
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
	requestDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/insert/journald/upload"}`)
)

func processStreamInternal(streamName string, r io.Reader, lmp insertutil.LogMessageProcessor, cp *insertutil.CommonParams) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lr := insertutil.NewLineReader("journald", wcr)

	for {
		err := readJournaldLogEntry(streamName, lr, lmp, cp)
		wcr.DecConcurrency()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("%s: %w", streamName, err)
		}
	}
}

type fieldsBuf struct {
	fields []logstorage.Field

	buf   []byte
	name  []byte
	value []byte
}

func (fb *fieldsBuf) reset() {
	fb.fields = fb.fields[:0]
	fb.buf = fb.buf[:0]
	fb.name = fb.name[:0]
	fb.value = fb.value[:0]
}

func (fb *fieldsBuf) addField(name, value string) {
	bufLen := len(fb.buf)
	fb.buf = append(fb.buf, name...)
	nameCopy := bytesutil.ToUnsafeString(fb.buf[bufLen:])

	bufLen = len(fb.buf)
	fb.buf = append(fb.buf, value...)
	valueCopy := bytesutil.ToUnsafeString(fb.buf[bufLen:])

	fb.fields = append(fb.fields, logstorage.Field{
		Name:  nameCopy,
		Value: valueCopy,
	})
}

func (fb *fieldsBuf) appendNextLineToValue(lr *insertutil.LineReader) error {
	if !lr.NextLine() {
		if err := lr.Err(); err != nil {
			return err
		}
		return fmt.Errorf("unexpected end of stream")
	}
	fb.value = append(fb.value, lr.Line...)
	fb.value = append(fb.value, '\n')
	return nil
}

func getFieldsBuf() *fieldsBuf {
	fb := fieldsBufPool.Get()
	if fb == nil {
		return &fieldsBuf{}
	}
	return fb.(*fieldsBuf)
}

func putFieldsBuf(fb *fieldsBuf) {
	fb.reset()
	fieldsBufPool.Put(fb)
}

var fieldsBufPool sync.Pool

// readJournaldLogEntry reads a single log entry in Journald format.
//
// See https://systemd.io/JOURNAL_EXPORT_FORMATS/#journal-export-format
func readJournaldLogEntry(streamName string, lr *insertutil.LineReader, lmp insertutil.LogMessageProcessor, cp *insertutil.CommonParams) error {
	var ts int64
	var name, value string

	fb := getFieldsBuf()
	defer putFieldsBuf(fb)

	if !lr.NextLine() {
		if err := lr.Err(); err != nil {
			return fmt.Errorf("cannot read the first field: %w", err)
		}
		return io.EOF
	}

	for {
		line := lr.Line
		if len(line) == 0 {
			// The end of a single log entry. Write it to the storage
			if len(fb.fields) > 0 {
				if ts == 0 {
					ts = time.Now().UnixNano()
				}
				lmp.AddRow(ts, fb.fields, nil)
			}
			return nil
		}

		// line could be either "key=value" or "key"
		// according to https://systemd.io/JOURNAL_EXPORT_FORMATS/#journal-export-format
		if n := bytes.IndexByte(line, '='); n >= 0 {
			// line = "key=value"
			fb.name = append(fb.name[:0], line[:n]...)
			name = bytesutil.ToUnsafeString(fb.name)

			fb.value = append(fb.value[:0], line[n+1:]...)
			value = bytesutil.ToUnsafeString(fb.value)
		} else {
			// line = "key"
			// Parse the binary-encoded value from the next line according to "key\n<little_endian_size_64>value\n" format
			fb.name = append(fb.name[:0], line...)
			name = bytesutil.ToUnsafeString(fb.name)

			fb.value = fb.value[:0]
			for len(fb.value) < 8 {
				if err := fb.appendNextLineToValue(lr); err != nil {
					return fmt.Errorf("cannot read value size: %w", err)
				}
			}
			size := binary.LittleEndian.Uint64(fb.value[:8])

			// Read the value until its length exceeds the given size - the last char in the read value will always be '\n'
			// because it is appended by appendNextLineToValue().
			for uint64(len(fb.value[8:])) <= size {
				if err := fb.appendNextLineToValue(lr); err != nil {
					return fmt.Errorf("cannot read %q value with size %d bytes; read only %d bytes: %w", fb.name, size, len(fb.value[8:]), err)
				}
			}
			value = bytesutil.ToUnsafeString(fb.value[8 : len(fb.value)-1])
			if uint64(len(value)) != size {
				return fmt.Errorf("unexpected %q value size; got %d bytes; want %d bytes; value: %q", fb.name, len(value), size, value)
			}
		}

		if !lr.NextLine() {
			if err := lr.Err(); err != nil {
				return fmt.Errorf("cannot read the next log field: %w", err)
			}

			// add the last log field below before the return
		}

		if len(name) > maxFieldNameLen {
			logger.Errorf("%s: field name size should not exceed %d bytes; got %d bytes: %q; skipping this field", streamName, maxFieldNameLen, len(name), name)
			continue
		}
		if !isValidFieldName(name) {
			logger.Errorf("%s: invalid field name %q; it must consist of `A-Z0-9_` chars and must start from non-digit char; skipping this field", streamName, name)
			continue
		}

		if slices.Contains(cp.TimeFields, name) {
			t, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				logger.Errorf("%s: cannot parse timestamp from the field %q: %w; using the current timestamp", streamName, name, err)
				ts = 0
			} else {
				// Convert journald microsecond timestamp to nanoseconds
				ts = t * 1e3
			}
			continue
		}

		if slices.Contains(cp.MsgFields, name) {
			name = "_msg"
		}

		if name == "PRIORITY" {
			priority := journaldPriorityToLevel(value)
			fb.addField("level", priority)
		}

		if !strings.HasPrefix(name, "__") || *journaldIncludeEntryMetadata {
			fb.addField(name, value)
		}
	}
}

func journaldPriorityToLevel(priority string) string {
	// See https://wiki.archlinux.org/title/Systemd/Journal#Priority_level
	// and https://grafana.com/docs/grafana/latest/explore/logs-integration/#log-level
	switch priority {
	case "0":
		return "emerg"
	case "1":
		return "alert"
	case "2":
		return "critical"
	case "3":
		return "error"
	case "4":
		return "warning"
	case "5":
		return "notice"
	case "6":
		return "info"
	case "7":
		return "debug"
	default:
		return priority
	}
}

func isValidFieldName(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[0]
	if !(c >= 'A' && c <= 'Z' || c == '_') {
		return false
	}

	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}
