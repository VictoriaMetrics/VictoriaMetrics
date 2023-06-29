package loki

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"

	"github.com/valyala/fastjson"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
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
			return true
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	cp, err := getCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse request: %s", err)
		return true
	}
	lr := logstorage.GetLogRows(cp.StreamFields, cp.IgnoreFields)
	defer logstorage.PutLogRows(lr)

	processLogMessage := cp.GetProcessLogMessageFunc(lr)
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

	var commonFields []logstorage.Field
	rowsIngested := 0
	for stIdx, st := range v.GetArray("streams") {
		// `stream` contains labels for the stream.
		// Labels are same for all entries in the stream.
		logFields := st.GetObject("stream")
		if logFields == nil {
			logger.Warnf("missing streams field from %q", st)
			logFields = &fastjson.Object{}
		}
		commonFields = slicesutil.ResizeNoCopyMayOverallocate(commonFields, logFields.Len()+1)
		i := 0
		logFields.Visit(func(k []byte, v *fastjson.Value) {
			sfName := bytesutil.ToUnsafeString(k)
			sfValue := bytesutil.ToUnsafeString(v.GetStringBytes())
			commonFields[i].Name = sfName
			commonFields[i].Value = sfValue
			i++
		})
		msgFieldIdx := logFields.Len()
		commonFields[msgFieldIdx].Name = msgField

		for idx, v := range st.GetArray("values") {
			vs := v.GetArray()
			if len(vs) != 2 {
				return rowsIngested, fmt.Errorf("unexpected number of values in stream %d line %d: %q; got %d; want %d", stIdx, idx, v, len(vs), 2)
			}

			tsString := bytesutil.ToUnsafeString(vs[0].GetStringBytes())
			ts, err := parseLokiTimestamp(tsString)
			if err != nil {
				return rowsIngested, fmt.Errorf("cannot parse timestamp in stream %d line %d: %q: %s", stIdx, idx, vs, err)
			}

			commonFields[msgFieldIdx].Value = bytesutil.ToUnsafeString(vs[1].GetStringBytes())
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
		return 0, fmt.Errorf("too big timestamp in nanoseconds: %d; mustn't exceed %d", n, math.MaxInt64)
	}
	if n < 0 {
		return 0, fmt.Errorf("too small timestamp in nanoseconds: %d; must be bigger than %d", n, 0)
	}
	return n, nil
}
