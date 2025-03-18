package stream

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxInsertRequestSize = flagutil.NewBytes("opentsdbhttp.maxInsertRequestSize", 32*1024*1024, "The maximum size of OpenTSDB HTTP put request")
	trimTimestamp        = flag.Duration("opentsdbhttpTrimTimestamp", time.Millisecond, "Trim timestamps for OpenTSDB HTTP data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// Parse parses OpenTSDB http lines from req and calls callback for the parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from req.
//
// callback shouldn't hold rows after returning.
func Parse(req *http.Request, callback func(rows []opentsdbhttp.Row) error) error {
	readCalls.Inc()
	encoding := req.Header.Get("Content-Encoding")
	err := protoparserutil.ReadUncompressedData(req.Body, encoding, maxInsertRequestSize, func(data []byte) error {
		return parseData(data, callback)
	})
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot decode opentsdb data: %w", err)
	}
	return nil
}

func parseData(data []byte, callback func(rows []opentsdbhttp.Row) error) error {
	p := opentsdbhttp.GetJSONParser()
	defer opentsdbhttp.PutJSONParser(p)
	v, err := p.ParseBytes(data)
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot parse HTTP OpenTSDB json: %w", err)
	}
	rs := getRows()
	defer putRows(rs)
	rs.Unmarshal(v)
	rows := rs.Rows
	rowsRead.Add(len(rows))

	// Fill in missing timestamps
	currentTimestamp := int64(fasttime.UnixTimestamp())
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = currentTimestamp
		}
	}

	// Convert timestamps in seconds to milliseconds if needed.
	// See http://opentsdb.net/docs/javadoc/net/opentsdb/core/Const.html#SECOND_MASK
	for i := range rows {
		r := &rows[i]
		if r.Timestamp&secondMask == 0 {
			r.Timestamp *= 1e3
		}
	}

	// Trim timestamps if required.
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}

	if err := callback(rows); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

const secondMask int64 = 0x7FFFFFFF00000000

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="opentsdbhttp"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="opentsdbhttp"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentsdbhttp"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="opentsdbhttp"}`)
)

func getRows() *opentsdbhttp.Rows {
	v := rowsPool.Get()
	if v == nil {
		return &opentsdbhttp.Rows{}
	}
	return v.(*opentsdbhttp.Rows)
}

func putRows(rs *opentsdbhttp.Rows) {
	rs.Reset()
	rowsPool.Put(rs)
}

var rowsPool sync.Pool
