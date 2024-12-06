package journald

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

const (
	journaldEntryMaxNameLen = 64
)

var (
	bodyBufferPool                bytesutil.ByteBufferPool
	allowedJournaldEntryNameChars = regexp.MustCompile(`^[A-Z_][A-Z0-9_]+`)
)

var (
	journaldStreamFields = flagutil.NewArrayString("journald.streamFields", "Journal fields to be used as stream fields. "+
		"See the list of allowed fields at https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html.")
	journaldIgnoreFields = flagutil.NewArrayString("journald.ignoreFields", "Journal fields to ignore. "+
		"See the list of allowed fields at https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html.")
	journaldTimeField = flag.String("journald.timeField", "__REALTIME_TIMESTAMP", "Journal field to be used as time field. "+
		"See the list of allowed fields at https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html.")
	journaldTenantID             = flag.String("journald.tenantID", "0:0", "TenantID for logs ingested via the Journald endpoint.")
	journaldIncludeEntryMetadata = flag.Bool("journald.includeEntryMetadata", false, "Include journal entry fields, which with double underscores.")
)

func getCommonParams(r *http.Request) (*insertutils.CommonParams, error) {
	cp, err := insertutils.GetCommonParams(r)
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
	if cp.TimeField != "" {
		cp.TimeField = *journaldTimeField
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
	requestsJournaldTotal.Inc()

	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	reader := r.Body
	var err error

	wcr := writeconcurrencylimiter.GetReader(reader)
	data, err := io.ReadAll(wcr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot read request body: %s", err)
		return
	}
	writeconcurrencylimiter.PutReader(wcr)
	bb := bodyBufferPool.Get()
	defer bodyBufferPool.Put(bb)
	if r.Header.Get("Content-Encoding") == "zstd" {
		bb.B, err = zstd.Decompress(bb.B[:0], data)
		if err != nil {
			httpserver.Errorf(w, r, "cannot decompress zstd-encoded request with length %d: %s", len(data), err)
			return
		}
		data = bb.B
	}
	cp, err := getCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}

	lmp := cp.NewLogMessageProcessor("journald")
	err = parseJournaldRequest(data, lmp, cp)
	lmp.MustClose()
	if err != nil {
		errorsTotal.Inc()
		httpserver.Errorf(w, r, "cannot parse Journald protobuf request: %s", err)
		return
	}

	// update requestJournaldDuration only for successfully parsed requests
	// There is no need in updating requestJournaldDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestJournaldDuration.UpdateDuration(startTime)
}

var (
	requestsJournaldTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/journald/upload"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/journald/upload"}`)

	requestJournaldDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/journald/upload"}`)
)

// See https://systemd.io/JOURNAL_EXPORT_FORMATS/#journal-export-format
func parseJournaldRequest(data []byte, lmp insertutils.LogMessageProcessor, cp *insertutils.CommonParams) error {
	var fields []logstorage.Field
	var ts int64
	var size uint64
	var name, value string
	var line []byte

	currentTimestamp := time.Now().UnixNano()

	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		switch {
		case idx > 0:
			// process fields
			line = data[:idx]
			data = data[idx+1:]
		case idx == 0:
			// next message or end of file
			// double new line is a separator for the next message
			if len(fields) > 0 {
				if ts == 0 {
					ts = currentTimestamp
				}
				lmp.AddRow(ts, fields, nil)
				fields = fields[:0]
			}
			// skip newline separator
			data = data[1:]
			continue
		case idx < 0:
			return fmt.Errorf("missing new line separator, unread data left=%d", len(data))
		}

		idx = bytes.IndexByte(line, '=')
		// could b either e key=value\n pair
		// or just  key\n
		// with binary data at the buffer
		if idx > 0 {
			name = bytesutil.ToUnsafeString(line[:idx])
			value = bytesutil.ToUnsafeString(line[idx+1:])
		} else {
			name = bytesutil.ToUnsafeString(line)
			if len(data) == 0 {
				return fmt.Errorf("unexpected zero data for binary field value of key=%s", name)
			}
			// size of binary data encoded as le i64 at the begging
			idx, err := binary.Decode(data, binary.LittleEndian, &size)
			if err != nil {
				return fmt.Errorf("failed to extract binary field %q value size: %w", name, err)
			}
			// skip binary data sise
			data = data[idx:]
			if size == 0 {
				return fmt.Errorf("unexpected zero binary data size decoded %d", size)
			}
			if int(size) > len(data) {
				return fmt.Errorf("binary data size=%d cannot exceed size of the data at buffer=%d", size, len(data))
			}
			value = bytesutil.ToUnsafeString(data[:size])
			data = data[int(size):]
			// binary data must has new line separator for the new line or next field
			if len(data) == 0 {
				return fmt.Errorf("unexpected empty buffer after binary field=%s read", name)
			}
			lastB := data[0]
			if lastB != '\n' {
				return fmt.Errorf("expected new line separator after binary field=%s, got=%s", name, string(lastB))
			}
			data = data[1:]
		}
		// https://github.com/systemd/systemd/blob/main/src/libsystemd/sd-journal/journal-file.c#L1703
		if len(name) > journaldEntryMaxNameLen {
			return fmt.Errorf("journald entry name should not exceed %d symbols, got: %q", journaldEntryMaxNameLen, name)
		}
		if !allowedJournaldEntryNameChars.MatchString(name) {
			return fmt.Errorf("journald entry name should consist of `A-Z0-9_` characters and must start from non-digit symbol")
		}
		if name == cp.TimeField {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse Journald timestamp, %w", err)
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
	return nil
}
