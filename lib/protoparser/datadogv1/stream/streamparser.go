package stream

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv1"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses DataDog POST request for /api/v1/series from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func Parse(r io.Reader, encoding string, callback func(series []datadogv1.Series) error) error {
	readCalls.Inc()

	err := protoparserutil.ReadUncompressedData(r, encoding, datadogutil.MaxInsertRequestSize, func(data []byte) error {
		return parseData(data, callback)
	})
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot decode DataDog protocol data: %w", err)
	}
	return nil
}

func parseData(data []byte, callback func(series []datadogv1.Series) error) error {
	req := getRequest()
	defer putRequest(req)

	if err := req.Unmarshal(data); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog POST request with size %d bytes: %w", len(data), err)
	}

	rows := 0
	series := req.Series
	for i := range series {
		rows += len(series[i].Points)
		if *datadogutil.SanitizeMetricName {
			series[i].Metric = datadogutil.SanitizeName(series[i].Metric)
		}
	}
	rowsRead.Add(rows)

	if err := callback(series); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadogv1"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadogv1"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadogv1"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadogv1"}`)
)

func getRequest() *datadogv1.Request {
	v := requestPool.Get()
	if v == nil {
		return &datadogv1.Request{}
	}
	return v.(*datadogv1.Request)
}

func putRequest(req *datadogv1.Request) {
	requestPool.Put(req)
}

var requestPool sync.Pool
