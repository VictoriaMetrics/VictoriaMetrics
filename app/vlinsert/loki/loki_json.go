package loki

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"

	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsIngestedTotalJSON = metrics.NewCounter(`vl_rows_ingested_total{type="loki", format="json"}`)
	parserPool            fastjson.ParserPool
)

func handleJSON(r *http.Request, w http.ResponseWriter) bool {
	contentType := r.Header.Get("Content-Type")
	reader := r.Body
	if contentType == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			httpserver.Errorf(w, r, "cannot read gzipped request: %s", err)
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	lr := getLogRows(r)

	tenantID, err := getTenantIDFromRequest(r)
	if err != nil {
		httpserver.Errorf(w, r, "failed to get tenantID: %s", err)
		return true
	}

	processLogMessage := getLogMessageHandler(r, tenantID, lr)
	n, err := processJSONRequest(reader, processLogMessage)
	if err != nil {
		httpserver.Errorf(w, r, "cannot decode loki request: %s", err)
		return true
	}
	rowsIngestedTotalJSON.Add(n)
	return true
}

func processJSONRequest(r io.Reader, processLogMessage func(timestamp int64, fields []logstorage.Field)) (int, error) {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	bytes, err := io.ReadAll(wcr)
	if err != nil {
		return 0, fmt.Errorf("cannot read request body: %w", err)
	}

	p := parserPool.Get()
	defer parserPool.Put(p)
	v, err := p.ParseBytes(bytes)
	if err != nil {
		return 0, fmt.Errorf("cannot parse request body: %w", err)
	}

	rowsIngested := 0
	for _, st := range v.GetArray("streams") {
		// `stream` contains labels for the stream.
		// Labels are same for all entries in the stream.
		logFields := st.GetObject("stream")
		if logFields == nil {
			logger.Warnf("missing streams field from %q", st)
			logFields = &fastjson.Object{}
		}
		commonFields := make([]logstorage.Field, logFields.Len()+1)
		i := 0
		logFields.Visit(func(k []byte, v *fastjson.Value) {
			sfName := bytesutil.ToUnsafeString(k)
			sfValue := bytesutil.ToUnsafeString(v.GetStringBytes())
			commonFields[i] = logstorage.Field{
				Name:  sfName,
				Value: sfValue,
			}
			i++
		})
		msgFieldIdx := logFields.Len()
		commonFields[msgFieldIdx].Name = msgField

		for _, v := range st.GetArray("values") {
			vs := v.GetArray()
			if len(vs) != 2 {
				logger.Warnf("unexpected number of values in %q; got %d; want %d", v, len(vs), 2)
				continue
			}
			tsString := bytesutil.ToUnsafeString(vs[0].GetStringBytes())
			tsString = strings.Trim(tsString, `"`)
			ts, err := parseLokiTimestamp(tsString)
			if err != nil {
				logger.Warnf("cannot parse timestamp %q: %s", vs, err)
				continue
			}

			commonFields[msgFieldIdx].Value = strings.Trim(vs[1].String(), `"`)
			processLogMessage(ts, commonFields)

			rowsIngested++
		}
	}

	return rowsIngested, nil
}

func parseLokiTimestamp(s string) (int64, error) {
	// Parsing timestamp in nanoseconds
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse timestamp in nanoseconds from %q: %w", s, err)
	}
	if n > int64(math.MaxInt64) {
		return 0, fmt.Errorf("too big timestamp in nanoseconds: %d; mustn't exceed %d", n, int64(math.MaxInt64)/1e9)
	}
	if n < 0 {
		return 0, fmt.Errorf("too small timestamp in nanoseconds: %d; must be bigger than %d", n, 0)
	}
	return n, nil
}
