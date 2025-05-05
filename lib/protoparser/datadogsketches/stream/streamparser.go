package stream

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses DataDog POST request for /api/beta/sketches from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func Parse(r io.Reader, encoding string, callback func(series []*datadogsketches.Sketch) error) error {
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

func parseData(data []byte, callback func(series []*datadogsketches.Sketch) error) error {
	req := getRequest()
	defer putRequest(req)

	if err := req.UnmarshalProtobuf(data); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog Sketches request with size %d bytes: %w", len(data), err)
	}

	rows := 0
	sketches := req.Sketches
	for _, sketch := range sketches {
		rows += sketch.RowsCount()
		if *datadogutil.SanitizeMetricName {
			sketch.Metric = datadogutil.SanitizeName(sketch.Metric)
		}
	}
	rowsRead.Add(rows)

	if err := callback(sketches); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadogsketches"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadogsketches"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadogsketches"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadogsketches"}`)
)

func getRequest() *datadogsketches.SketchPayload {
	v := requestPool.Get()
	if v == nil {
		return &datadogsketches.SketchPayload{}
	}
	return v.(*datadogsketches.SketchPayload)
}

func putRequest(req *datadogsketches.SketchPayload) {
	requestPool.Put(req)
}

var requestPool sync.Pool
