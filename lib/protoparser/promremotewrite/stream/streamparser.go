package stream

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/snappy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

var maxInsertRequestSize = flagutil.NewBytes("maxInsertRequestSize", 32*1024*1024, "The maximum size in bytes of a single Prometheus remote_write API request")

// Parse parses Prometheus remote_write message from reader and calls callback for the parsed timeseries.
//
// callback shouldn't hold tss after returning.
func Parse(r io.Reader, isVMRemoteWrite bool, callback func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error) error {
	startTime := fasttime.UnixTimestamp()

	readCalls.Inc()
	err := protoparserutil.ReadUncompressedData(r, "", maxInsertRequestSize, func(data []byte) error {
		return parseRequestBody(data, isVMRemoteWrite, callback)
	})
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read prometheus remote_write data from client in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	return nil
}

func parseRequestBody(data []byte, isVMRemoteWrite bool, callback func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error) error {
	// Synchronously process the request in order to properly return errors to Parse caller,
	// so it could properly return HTTP 503 status code in response.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/896
	bb := bodyBufferPool.Get()
	defer bodyBufferPool.Put(bb)
	if isVMRemoteWrite {
		var err error
		bb.B, err = encoding.DecompressZSTDLimited(bb.B[:0], data, maxInsertRequestSize.IntN())
		if err != nil {
			// Fall back to Snappy decompression, since vmagent may send snappy-encoded messages
			// with 'Content-Encoding: zstd' header if they were put into persistent queue before vmagent restart.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301
			//
			// Newer vmagent sends proper 'Content-Encoding' header.
			// The logic is preserved for backwards compatibility.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8650
			zstdErr := err
			bb.B, err = snappy.Decode(bb.B, data, maxInsertRequestSize.IntN())
			if err != nil {
				return fmt.Errorf("cannot decompress zstd-encoded request with length %d: %w", len(data), zstdErr)
			}
		}
	} else {
		var err error
		bb.B, err = snappy.Decode(bb.B, data, maxInsertRequestSize.IntN())
		if err != nil {
			// Fall back to zstd decompression, since vmagent may send zstd-encoded messages
			// without 'Content-Encoding: zstd' header if they were put into persistent queue before vmagent restart.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301#issuecomment-1815871992
			//
			// Newer vmagent sends proper 'Content-Encoding' header.
			// The logic is preserved for backwards compatibility.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8650
			snappyErr := err
			bb.B, err = encoding.DecompressZSTDLimited(bb.B[:0], data, maxInsertRequestSize.IntN())
			if err != nil {
				return fmt.Errorf("cannot decompress snappy-encoded request with length %d: %w", len(data), snappyErr)
			}
		}
	}
	if int64(len(bb.B)) > maxInsertRequestSize.N {
		return fmt.Errorf("too big unpacked request; mustn't exceed `-maxInsertRequestSize=%d` bytes; got %d bytes", maxInsertRequestSize.N, len(bb.B))
	}
	wru := prompb.GetWriteRequestUnmarshaler()
	defer prompb.PutWriteRequestUnmarshaler(wru)
	wr, err := wru.UnmarshalProtobuf(bb.B)
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal prompb.WriteRequest with size %d bytes: %w", len(bb.B), err)
	}

	rows := 0
	tss := wr.Timeseries
	for i := range tss {
		rows += len(tss[i].Samples)
	}
	rowsRead.Add(rows)
	mms := wr.Metadata
	metadataRead.Add(len(mms))

	if err := callback(tss, mms); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

var bodyBufferPool bytesutil.ByteBufferPool

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="promremotewrite"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="promremotewrite"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="promremotewrite"}`)
	metadataRead    = metrics.NewCounter(`vm_protoparser_metadata_read_total{type="promremotewrite"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="promremotewrite"}`)
)
