package influx

import (
	"flag"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

var (
	trimTimestamp = flag.Duration("influxTrimTimestamp", time.Millisecond, "Trim timestamps for Influx line protocol data to this duration. "+
		"Minimum practical duration is 1ms. Higher duration (i.e. 1s) may be used for reducing disk space usage for timestamp data")
)

// ParseStream parses r with the given args and calls callback for the parsed rows.
//
// The callback can be called multiple times for streamed data from r.
//
// callback shouldn't hold rows after returning.
func ParseStream(r io.Reader, isGzipped bool, precision, db string, callback func(db string, rows []Row) error) error {
	readCalls.Inc()
	if isGzipped {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped influx line protocol data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	// Default precision is 'ns'. See https://docs.influxdata.com/influxdb/v1.7/write_protocols/line_protocol_tutorial/#timestamp
	tsMultiplier := int64(1e6)
	switch precision {
	case "ns":
		tsMultiplier = 1e6
	case "u":
		tsMultiplier = 1e3
	case "ms":
		tsMultiplier = 1
	case "s":
		tsMultiplier = -1e3
	case "m":
		tsMultiplier = -1e3 * 60
	case "h":
		tsMultiplier = -1e3 * 3600
	}

	ctx := getStreamContext()
	defer putStreamContext(ctx)
	for ctx.Read(r, tsMultiplier) {
		if err := callback(db, ctx.Rows.Rows); err != nil {
			return err
		}
	}
	return ctx.Error()
}

func (ctx *streamContext) Read(r io.Reader, tsMultiplier int64) bool {
	if ctx.err != nil {
		return false
	}
	ctx.reqBuf, ctx.tailBuf, ctx.err = common.ReadLinesBlock(r, ctx.reqBuf, ctx.tailBuf)
	if ctx.err != nil {
		if ctx.err != io.EOF {
			readErrors.Inc()
			ctx.err = fmt.Errorf("cannot read influx line protocol data: %w", ctx.err)
		}
		return false
	}
	ctx.Rows.Unmarshal(bytesutil.ToUnsafeString(ctx.reqBuf))
	rowsRead.Add(len(ctx.Rows.Rows))

	rows := ctx.Rows.Rows

	// Adjust timestamps according to tsMultiplier
	currentTs := time.Now().UnixNano() / 1e6
	if tsMultiplier >= 1 {
		for i := range rows {
			row := &rows[i]
			if row.Timestamp == 0 {
				row.Timestamp = currentTs
			} else {
				row.Timestamp /= tsMultiplier
			}
		}
	} else if tsMultiplier < 0 {
		tsMultiplier = -tsMultiplier
		currentTs -= currentTs % tsMultiplier
		for i := range rows {
			row := &rows[i]
			if row.Timestamp == 0 {
				row.Timestamp = currentTs
			} else {
				row.Timestamp *= tsMultiplier
			}
		}
	}

	// Trim timestamps if required.
	if tsTrim := trimTimestamp.Milliseconds(); tsTrim > 1 {
		for i := range rows {
			row := &rows[i]
			row.Timestamp -= row.Timestamp % tsTrim
		}
	}

	return true
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="influx"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="influx"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="influx"}`)
)

type streamContext struct {
	Rows    Rows
	reqBuf  []byte
	tailBuf []byte
	err     error
}

func (ctx *streamContext) Error() error {
	if ctx.err == io.EOF {
		return nil
	}
	return ctx.err
}

func (ctx *streamContext) reset() {
	ctx.Rows.Reset()
	ctx.reqBuf = ctx.reqBuf[:0]
	ctx.tailBuf = ctx.tailBuf[:0]
	ctx.err = nil
}

func getStreamContext() *streamContext {
	select {
	case ctx := <-streamContextPoolCh:
		return ctx
	default:
		if v := streamContextPool.Get(); v != nil {
			return v.(*streamContext)
		}
		return &streamContext{}
	}
}

func putStreamContext(ctx *streamContext) {
	ctx.reset()
	select {
	case streamContextPoolCh <- ctx:
	default:
		streamContextPool.Put(ctx)
	}
}

var streamContextPool sync.Pool
var streamContextPoolCh = make(chan *streamContext, runtime.GOMAXPROCS(-1))
