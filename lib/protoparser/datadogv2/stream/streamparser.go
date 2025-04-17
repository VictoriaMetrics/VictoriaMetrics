package stream

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv2"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses DataDog POST request for /api/v2/series from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func Parse(r io.Reader, encoding, contentType string, callback func(series []datadogv2.Series) error) error {
	readCalls.Inc()

	err := protoparserutil.ReadUncompressedData(r, encoding, datadogutil.MaxInsertRequestSize, func(data []byte) error {
		return parseData(data, contentType, callback)
	})
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot decode DataDog protocol data: %w", err)
	}
	return nil
}

func parseData(data []byte, contentType string, callback func(series []datadogv2.Series) error) error {
	req := getRequest()
	defer putRequest(req)

	var err error
	switch contentType {
	case "application/x-protobuf":
		err = datadogv2.UnmarshalProtobuf(req, data)
	default:
		err = datadogv2.UnmarshalJSON(req, data)
	}
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog %s request with size %d bytes: %w", contentType, len(data), err)
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
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadogv2"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadogv2"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadogv2"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadogv2"}`)
)

func getRequest() *datadogv2.Request {
	v := requestPool.Get()
	if v == nil {
		return &datadogv2.Request{}
	}
	return v.(*datadogv2.Request)
}

func putRequest(req *datadogv2.Request) {
	requestPool.Put(req)
}

var requestPool sync.Pool
