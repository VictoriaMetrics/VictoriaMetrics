package journald

import (
	"flag"
	"fmt"
	"io"
	"net/http"
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

var bodyBufferPool bytesutil.ByteBufferPool

var (
	journaldStreamFields = flagutil.NewArrayString("journald.streamFields", "Journal fields to be used as stream fields. "+
		"See the list of allowed fields at https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html.")
	journaldIgnoreFields = flagutil.NewArrayString("journald.ignoreFields", "Journal fields to ignore. "+
		"See the list of allowed fields at https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html.")
	journaldTimeField = flag.String("journald.timeField", "__REALTIME_TIMESTAMP", "Journal field to be used as time field. "+
		"See the list of allowed fields at https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html.")
	journaldTenantID            = flag.String("journald.tenantID", "0:0", "TenantID for logs ingested via the Journald endpoint.")
	journaldIgnoreEntryMetadata = flag.Bool("journald.ignoreEntryMetadata", true, "Ignore journal entry fields, which with double underscores.")
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
	cp.MsgField = "MESSAGE"
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

	lmp := cp.NewLogMessageProcessor()
	n, err := parseJournaldRequest(data, lmp, cp)
	lmp.MustClose()
	if err != nil {
		errorsTotal.Inc()
		httpserver.Errorf(w, r, "cannot parse Journald protobuf request: %s", err)
		return
	}

	rowsIngestedJournaldTotal.Add(n)

	// update requestJournaldDuration only for successfully parsed requests
	// There is no need in updating requestJournaldDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestJournaldDuration.UpdateDuration(startTime)
}

var (
	rowsIngestedJournaldTotal = metrics.NewCounter(`vl_rows_ingested_total{type="journald", format="journald"}`)

	requestsJournaldTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/journald/upload",format="journald"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/journald/upload",format="journald"}`)

	requestJournaldDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/journald/upload",format="journald"}`)
)

func parseJournaldRequest(data []byte, lmp insertutils.LogMessageProcessor, cp *insertutils.CommonParams) (int, error) {
	dataStr := bytesutil.ToUnsafeString(data)
	var fields []logstorage.Field
	var ts int64
	var err error
	var rowsIngested, lastIdx int
	var name, value string
	for i, char := range dataStr {
		if char == '=' && len(name) == 0 {
			name = dataStr[lastIdx:i]
			if name == cp.MsgField {
				name = "_msg"
			}
			lastIdx = i + 1
		} else if char == '\n' || i == len(dataStr)-1 {
			if len(name) == 0 && len(fields) > 0 {
				lmp.AddRow(ts*1e3, fields)
				rowsIngested++
				fields = fields[:0]
			} else {
				if i == len(dataStr)-1 {
					value = dataStr[lastIdx:]
				} else {
					value = dataStr[lastIdx:i]
				}
				ignoreField := *journaldIgnoreEntryMetadata && strings.HasPrefix(name, "__")
				if name == cp.TimeField {
					// extract timetamp in microseconds
					ts, err = strconv.ParseInt(value, 10, 64)
					if err != nil {
						return 0, fmt.Errorf("failed to parse Journald timestamp, %w", err)
					}
				} else if !ignoreField {
					fields = append(fields, logstorage.Field{
						Name:  name,
						Value: value,
					})
				}
				name = ""
				value = ""
			}
			lastIdx = i + 1
		}
	}
	if len(fields) > 0 {
		lmp.AddRow(ts*1e3, fields)
		rowsIngested++
		fields = fields[:0]
	}
	return rowsIngested, nil
}
