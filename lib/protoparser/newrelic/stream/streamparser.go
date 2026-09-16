package stream

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
)

var (
	maxInsertRequestSize = flagutil.NewBytes("newrelic.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single NewRelic request "+
		"to /newrelic/infra/v2/metrics/events/bulk")
)

// Parse parses NewRelic POST request for /newrelic/infra/v2/metrics/events/bulk from r and calls callback for the parsed request.
//
// callback shouldn't hold rows after returning.
func Parse(r io.Reader, encoding string, callback func(rows []newrelic.Row) error) error {
	readCalls.Inc()
	err := protoparserutil.ReadUncompressedData(r, encoding, maxInsertRequestSize, func(data []byte) error {
		return parseData(data, callback)
	})
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot decode NewRelic data: %w", err)
	}
	return nil
}

func parseData(data []byte, callback func(rows []newrelic.Row) error) error {
	rows := getRows()
	defer putRows(rows)

	currentTimestamp := int64(fasttime.UnixTimestamp()) * 1e3
	var callbackErr error
	err := rows.UnmarshalWithCallback(data, func(rows []newrelic.Row) error {
		// Fill in missing timestamps.
		for i := range rows {
			r := &rows[i]
			if r.Timestamp == 0 {
				r.Timestamp = currentTimestamp
			}
		}

		if err := callback(rows); err != nil {
			callbackErr = fmt.Errorf("error when processing imported data: %w", err)
			return callbackErr
		}
		return nil
	})
	if callbackErr != nil {
		return callbackErr
	}
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal NewRelic request: %w", err)
	}
	return nil
}

func getRows() *newrelic.Rows {
	v := rowsPool.Get()
	if v == nil {
		return &newrelic.Rows{}
	}
	return v.(*newrelic.Rows)
}

func putRows(rows *newrelic.Rows) {
	rows.Reset()
	rowsPool.Put(rows)
}

var rowsPool sync.Pool

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="newrelic"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="newrelic"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="newrelic"}`)
)
