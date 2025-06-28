package internalinsert

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage/netinsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
)

var (
	maxRequestSize = flagutil.NewBytes("internalinsert.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single request, which can be accepted at /internal/insert HTTP endpoint")
)

// RequestHandler processes /internal/insert requests.
func RequestHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	version := r.FormValue("version")
	if version != netinsert.ProtocolVersion {
		httpserver.Errorf(w, r, "unsupported protocol version=%q; want %q", version, netinsert.ProtocolVersion)
		return
	}

	requestsTotal.Inc()

	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if err := insertutil.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	err = protoparserutil.ReadUncompressedData(r.Body, encoding, maxRequestSize, func(data []byte) error {
		lmp := cp.NewLogMessageProcessor("internalinsert", false)
		irp := lmp.(insertutil.InsertRowProcessor)
		err := parseData(irp, data)
		lmp.MustClose()
		return err
	})
	if err != nil {
		errorsTotal.Inc()
		httpserver.Errorf(w, r, "cannot parse internal insert request: %s", err)
		return
	}

	requestDuration.UpdateDuration(startTime)
}

func parseData(irp insertutil.InsertRowProcessor, data []byte) error {
	r := logstorage.GetInsertRow()
	src := data
	i := 0
	for len(src) > 0 {
		tail, err := r.UnmarshalInplace(src)
		if err != nil {
			return fmt.Errorf("cannot parse row #%d: %s", i, err)
		}
		src = tail
		i++

		irp.AddInsertRow(r)
	}
	logstorage.PutInsertRow(r)

	return nil
}

var (
	requestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/internal/insert"}`)
	errorsTotal   = metrics.NewCounter(`vl_http_errors_total{path="/internal/insert"}`)

	requestDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/internal/insert"}`)
)
