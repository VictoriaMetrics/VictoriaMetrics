package loki

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	rowsIngestedTotalProtobuf = metrics.NewCounter(`vl_rows_ingested_total{type="loki", format="protobuf"}`)
	bytesBufPool              bytesutil.ByteBufferPool
	pushReqsPool              sync.Pool
)

func handleProtobuf(r *http.Request, w http.ResponseWriter) bool {
	wcr := writeconcurrencylimiter.GetReader(r.Body)
	defer writeconcurrencylimiter.PutReader(wcr)

	cp, err := getCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse request: %s", err)
		return true
	}
	lr := logstorage.GetLogRows(cp.StreamFields, cp.IgnoreFields)

	processLogMessage := cp.GetProcessLogMessageFunc(lr)
	n, err := processProtobufRequest(wcr, processLogMessage)
	if err != nil {
		httpserver.Errorf(w, r, "cannot decode loki request: %s", err)
		return true
	}

	rowsIngestedTotalProtobuf.Add(n)

	return true
}

func processProtobufRequest(r io.Reader, processLogMessage func(timestamp int64, fields []logstorage.Field)) (int, error) {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	bytes, err := io.ReadAll(wcr)
	if err != nil {
		return 0, fmt.Errorf("cannot read request body: %s", err)
	}

	bb := bytesBufPool.Get()
	defer bytesBufPool.Put(bb)
	bb.B, err = snappy.Decode(bb.B[:cap(bb.B)], bytes)
	if err != nil {
		return 0, fmt.Errorf("cannot decode snappy from request body: %s", err)
	}

	req := getPushReq()
	defer putPushReq(req)
	err = req.Unmarshal(bb.B)
	if err != nil {
		return 0, fmt.Errorf("cannot parse request body: %s", err)
	}

	rowsIngested := 0
	for stIdx, st := range req.Streams {
		// st.Labels contains labels for the stream.
		// Labels are same for all entries in the stream.
		commonFields, err := parseLogFields(st.Labels)
		if err != nil {
			return rowsIngested, fmt.Errorf("failed to unmarshal labels in stream %d: %q; %s", stIdx, st.Labels, err)
		}
		msgFieldIDx := len(commonFields) - 1
		commonFields[msgFieldIDx].Name = msgField

		for _, v := range st.Entries {
			commonFields[msgFieldIDx].Value = v.Line
			processLogMessage(v.Timestamp.UnixNano(), commonFields)
			rowsIngested++
		}
	}
	return rowsIngested, nil
}

// Parses labels selector from s and returns the corresponding log fields.
// Cannot use searchutils.ParseMetricSelector here because its dependencies
// bring flags which clashes with logstorage flags.
func parseLogFields(s string) ([]logstorage.Field, error) {
	expr, err := metricsql.Parse(s)
	if err != nil {
		return nil, err
	}

	me, ok := expr.(*metricsql.MetricExpr)
	if !ok {
		return nil, fmt.Errorf("failed to parse stream labels; got %q", expr.AppendString(nil))
	}

	// Allocate space for labels + msg field.
	// Msg field is added by caller.
	lf := make([]logstorage.Field, len(me.LabelFilters)+1)
	for i, l := range me.LabelFilters {
		lf[i].Name = l.Label
		lf[i].Value = l.Value
	}

	return lf, nil
}

func getPushReq() *PushRequest {
	v := pushReqsPool.Get()
	if v == nil {
		return &PushRequest{}
	}
	return v.(*PushRequest)
}

func putPushReq(reqs *PushRequest) {
	reqs.Reset()
	pushReqsPool.Put(reqs)
}
