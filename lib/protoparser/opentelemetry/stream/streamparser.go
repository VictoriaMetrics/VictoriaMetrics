package stream

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
)

var maxRequestSize = flagutil.NewBytes("opentelemetry.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single OpenTelemetry request")

// ParseStream parses OpenTelemetry protobuf or json data from r and calls callback for the parsed rows.
//
// callback shouldn't hold tss items after returning.
//
// optional processBody can be used for pre-processing the read request body from r before parsing it in OpenTelemetry format.
func ParseStream(r io.Reader, encoding string, processBody func(data []byte) ([]byte, error), callback func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error) error {
	err := protoparserutil.ReadUncompressedData(r, encoding, maxRequestSize, func(data []byte) error {
		if processBody != nil {
			dataNew, err := processBody(data)
			if err != nil {
				return fmt.Errorf("cannot process request body: %w", err)
			}
			data = dataNew
		}
		return parseData(data, callback)
	})
	if err != nil {
		return fmt.Errorf("cannot decode OpenTelemetry protocol data: %w", err)
	}
	return nil
}

func parseData(data []byte, callback func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error) error {
	wctx := getWriteRequestContext()
	defer putWriteRequestContext(wctx)

	// the flushFunc will be called multiple time if the request is big, to avoid over allocating memory for such request.
	wctx.flushFunc = callback

	if err := pb.DecodeMetricsData(data, wctx); err != nil {
		return fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	// flush 1 last time before finishing the request. there might be data left.
	if len(wctx.tss) > 0 {
		if err := wctx.flushFunc(wctx.tss, wctx.mms); err != nil {
			if wctx.firstErr == nil {
				wctx.firstErr = err
			}
		} else {
			rowsRead.Add(len(wctx.tss))
		}
	}

	if wctx.firstErr != nil {
		// the request might be partially flushed. the client should be aware of the error and retry.
		return fmt.Errorf("errors happened during parsing, the first error: %w", wctx.firstErr)
	}

	// all succeed
	return nil
}

type writeRequestContext struct {
	samplesBuf []prompb.Sample
	labelsBuf  []prompb.Label

	sctx sanitizerContext

	seenMetricMetadata map[string]struct{}

	tss []prompb.TimeSeries
	mms []prompb.MetricMetadata

	buf []byte

	flushFunc func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error
	firstErr  error
}

func (wctx *writeRequestContext) reset() {
	wctx.resetBuffer()

	wctx.firstErr = nil
	wctx.flushFunc = nil
}

// resetBuffer only resets the buffer and labels while leaving error and flushFunc still in place.
func (wctx *writeRequestContext) resetBuffer() {
	clear(wctx.samplesBuf)
	wctx.samplesBuf = wctx.samplesBuf[:0]

	clear(wctx.labelsBuf)
	wctx.labelsBuf = wctx.labelsBuf[:0]

	wctx.sctx.reset()

	clear(wctx.seenMetricMetadata)

	clear(wctx.tss)
	wctx.tss = wctx.tss[:0]

	clear(wctx.mms)
	wctx.mms = wctx.mms[:0]

	wctx.buf = wctx.buf[:0]
}

func (wctx *writeRequestContext) PushSample(mm *pb.MetricMetadata, suffix string, ls *promutil.Labels, timestampNsecs uint64, value float64, flags uint32) {
	metricName := wctx.sctx.sanitizeMetricName(mm)
	metricName = wctx.concat(metricName, suffix)

	if flags&1 != 0 {
		// See https://github.com/open-telemetry/opentelemetry-proto/blob/049d4332834935792fd4dbd392ecd31904f99ba2/opentelemetry/proto/metrics/v1/metrics.proto#L375
		value = decimal.StaleNaN
	}

	timestamp := int64(timestampNsecs / 1e6)

	wctx.samplesBuf = append(wctx.samplesBuf, prompb.Sample{
		Value:     value,
		Timestamp: timestamp,
	})

	labelsBufLen := len(wctx.labelsBuf)
	wctx.labelsBuf = append(wctx.labelsBuf, prompb.Label{
		Name:  "__name__",
		Value: metricName,
	})
	for _, label := range ls.Labels {
		name := wctx.sctx.sanitizeLabelName(label.Name)
		name = wctx.cloneString(name)
		value := wctx.cloneString(label.Value)

		wctx.labelsBuf = append(wctx.labelsBuf, prompb.Label{
			Name:  name,
			Value: value,
		})
	}

	wctx.tss = append(wctx.tss, prompb.TimeSeries{
		Labels:  wctx.labelsBuf[labelsBufLen:],
		Samples: wctx.samplesBuf[len(wctx.samplesBuf)-1:],
	})

	// check if we should flush it right now, if the buf is already huge (4MiB).
	if len(wctx.buf) > 4*1024*1024 {
		if err := wctx.flushFunc(wctx.tss, wctx.mms); err != nil {
			if wctx.firstErr == nil {
				wctx.firstErr = err
			}
		} else {
			rowsRead.Add(len(wctx.tss))
		}
		wctx.resetBuffer()
	}
}

func (wctx *writeRequestContext) PushMetricMetadata(mm *pb.MetricMetadata) {
	metricName := wctx.sctx.sanitizeMetricName(mm)
	metricName = wctx.cloneString(metricName)

	if _, ok := wctx.seenMetricMetadata[metricName]; ok {
		// The metadata for this metric has been already registered
		return
	}
	if wctx.seenMetricMetadata == nil {
		wctx.seenMetricMetadata = make(map[string]struct{})
	}
	wctx.seenMetricMetadata[metricName] = struct{}{}

	wctx.mms = append(wctx.mms, prompb.MetricMetadata{
		MetricFamilyName: metricName,
		Help:             wctx.cloneString(mm.Description),
		Unit:             wctx.cloneString(mm.Unit),
		Type:             mm.Type,
	})
}

func (wctx *writeRequestContext) cloneString(s string) string {
	bufLen := len(wctx.buf)
	wctx.buf = append(wctx.buf, s...)
	return bytesutil.ToUnsafeString(wctx.buf[bufLen:])
}

func (wctx *writeRequestContext) concat(a, b string) string {
	bufLen := len(wctx.buf)
	wctx.buf = append(wctx.buf, a...)
	wctx.buf = append(wctx.buf, b...)
	return bytesutil.ToUnsafeString(wctx.buf[bufLen:])
}

func getWriteRequestContext() *writeRequestContext {
	v := wctxPool.Get()
	if v == nil {
		return &writeRequestContext{}
	}
	return v.(*writeRequestContext)
}

func putWriteRequestContext(wctx *writeRequestContext) {
	if 4*len(wctx.buf) < cap(wctx.buf) && cap(wctx.buf) > 4*1024*1024 {
		// do not return it to the pool if the actual inuse size is a lot smaller than the cap, which could be
		// over expanded by previous request.
		// see: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10378
		return
	}
	wctx.reset()
	wctxPool.Put(wctx)
}

var wctxPool sync.Pool

var rowsRead = metrics.NewCounter(`vm_protoparser_rows_read_total{type="opentelemetry"}`)
