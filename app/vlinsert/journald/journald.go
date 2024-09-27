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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

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

func getCommonParams(tenantID logstorage.TenantID) *insertutils.CommonParams {
	return &insertutils.CommonParams{
		TenantID:     tenantID,
		TimeField:    *journaldTimeField,
		MsgField:     "MESSAGE",
		StreamFields: *journaldStreamFields,
		IgnoreFields: *journaldIgnoreFields,
	}
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
	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			httpserver.Errorf(w, r, "cannot initialize gzip reader: %s", err)
			return
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	wcr := writeconcurrencylimiter.GetReader(reader)
	data, err := io.ReadAll(wcr)
	writeconcurrencylimiter.PutReader(wcr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot read request body: %s", err)
		return
	}

	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	tenantID, err := logstorage.ParseTenantID(*journaldTenantID)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse -journald.tenantID=%q for journald: %s", *journaldTenantID, err)
	}
	cp := getCommonParams(tenantID)
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
	var rowsIngested int
	var lastIdx int
	var name string
	for i, char := range dataStr {
		if char == '=' && len(name) == 0 {
			name = dataStr[lastIdx:i]
			if name == cp.MsgField {
				name = "_msg"
			}
			lastIdx = i + 1
		} else if char == '\n' {
			value := dataStr[lastIdx:i]
			if len(name) == 0 && len(fields) > 0 {
				lmp.AddRow(ts*1e3, fields)
				rowsIngested++
				fields = fields[:0]
			} else {
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
			}
			lastIdx = i + 1
		}
	}
	return rowsIngested, nil
}
