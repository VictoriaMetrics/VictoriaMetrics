package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses /api/v1/import/native lines from req and calls callback for parsed blocks.
//
// The callback can be called concurrently multiple times for streamed data from r.
//
// callback shouldn't hold block after returning.
func Parse(r io.Reader, isGzip bool, callback func(block *Block) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	if isGzip {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped vmimport data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}
	br := getBufferedReader(r)
	defer putBufferedReader(br)

	// Read time range (tr)
	trBuf := make([]byte, 16)
	var tr storage.TimeRange
	if _, err := io.ReadFull(br, trBuf); err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read time range: %w", err)
	}
	tr.MinTimestamp = encoding.UnmarshalInt64(trBuf)
	tr.MaxTimestamp = encoding.UnmarshalInt64(trBuf[8:])

	// Read native blocks and feed workers with work.
	sizeBuf := make([]byte, 4)

	ctx := &streamContext{}
	for {
		uw := getUnmarshalWork()
		uw.tr = tr
		uw.ctx = ctx
		uw.callback = callback

		// Read uw.metricNameBuf
		if _, err := io.ReadFull(br, sizeBuf); err != nil {
			if err == io.EOF {
				// End of stream
				putUnmarshalWork(uw)
				ctx.wg.Wait()
				return ctx.err
			}
			readErrors.Inc()
			ctx.wg.Wait()
			return fmt.Errorf("cannot read metricName size: %w", err)
		}
		readCalls.Inc()
		bufSize := encoding.UnmarshalUint32(sizeBuf)
		if bufSize > 1024*1024 {
			parseErrors.Inc()
			ctx.wg.Wait()
			return fmt.Errorf("too big metricName size; got %d; shouldn't exceed %d", bufSize, 1024*1024)
		}
		uw.metricNameBuf = bytesutil.ResizeNoCopyMayOverallocate(uw.metricNameBuf, int(bufSize))
		if _, err := io.ReadFull(br, uw.metricNameBuf); err != nil {
			readErrors.Inc()
			ctx.wg.Wait()
			return fmt.Errorf("cannot read metricName with size %d bytes: %w", bufSize, err)
		}
		readCalls.Inc()

		// Read uw.blockBuf
		if _, err := io.ReadFull(br, sizeBuf); err != nil {
			readErrors.Inc()
			ctx.wg.Wait()
			return fmt.Errorf("cannot read native block size: %w", err)
		}
		readCalls.Inc()
		bufSize = encoding.UnmarshalUint32(sizeBuf)
		if bufSize > 1024*1024 {
			parseErrors.Inc()
			ctx.wg.Wait()
			return fmt.Errorf("too big native block size; got %d; shouldn't exceed %d", bufSize, 1024*1024)
		}
		uw.blockBuf = bytesutil.ResizeNoCopyMayOverallocate(uw.blockBuf, int(bufSize))
		if _, err := io.ReadFull(br, uw.blockBuf); err != nil {
			readErrors.Inc()
			ctx.wg.Wait()
			return fmt.Errorf("cannot read native block with size %d bytes: %w", bufSize, err)
		}
		readCalls.Inc()
		blocksRead.Inc()

		ctx.wg.Add(1)
		common.ScheduleUnmarshalWork(uw)
		wcr.DecConcurrency()
	}
}

type streamContext struct {
	wg      sync.WaitGroup
	errLock sync.Mutex
	err     error
}

// Block is a single block from `/api/v1/import/native` request.
type Block struct {
	MetricName storage.MetricName
	Values     []float64
	Timestamps []int64
}

func (b *Block) reset() {
	b.MetricName.Reset()
	b.Values = b.Values[:0]
	b.Timestamps = b.Timestamps[:0]
}

var (
	readCalls  = metrics.NewCounter(`vm_protoparser_read_calls_total{type="native"}`)
	readErrors = metrics.NewCounter(`vm_protoparser_read_errors_total{type="native"}`)
	rowsRead   = metrics.NewCounter(`vm_protoparser_rows_read_total{type="native"}`)
	blocksRead = metrics.NewCounter(`vm_protoparser_blocks_read_total{type="native"}`)

	parseErrors   = metrics.NewCounter(`vm_protoparser_parse_errors_total{type="native"}`)
	processErrors = metrics.NewCounter(`vm_protoparser_process_errors_total{type="native"}`)
)

type unmarshalWork struct {
	tr            storage.TimeRange
	ctx           *streamContext
	callback      func(block *Block) error
	metricNameBuf []byte
	blockBuf      []byte
	block         Block
}

func (uw *unmarshalWork) reset() {
	uw.ctx = nil
	uw.callback = nil
	uw.metricNameBuf = uw.metricNameBuf[:0]
	uw.blockBuf = uw.blockBuf[:0]
	uw.block.reset()
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	err := uw.unmarshal()
	if err != nil {
		parseErrors.Inc()
	} else {
		err = uw.callback(&uw.block)
	}
	ctx := uw.ctx
	if err != nil {
		processErrors.Inc()
		ctx.errLock.Lock()
		if ctx.err == nil {
			ctx.err = fmt.Errorf("error when processing native block: %w", err)
		}
		ctx.errLock.Unlock()
	}
	ctx.wg.Done()
	putUnmarshalWork(uw)
}

func (uw *unmarshalWork) unmarshal() error {
	block := &uw.block
	if err := block.MetricName.Unmarshal(uw.metricNameBuf); err != nil {
		return fmt.Errorf("cannot unmarshal metricName from %d bytes: %w", len(uw.metricNameBuf), err)
	}
	tmpBlock := blockPool.Get().(*storage.Block)
	defer blockPool.Put(tmpBlock)
	tail, err := tmpBlock.UnmarshalPortable(uw.blockBuf)
	if err != nil {
		return fmt.Errorf("cannot unmarshal native block from %d bytes: %w", len(uw.blockBuf), err)
	}
	if len(tail) > 0 {
		return fmt.Errorf("unexpected non-empty tail left after unmarshaling native block from %d bytes; len(tail)=%d bytes", len(uw.blockBuf), len(tail))
	}
	block.Timestamps, block.Values = tmpBlock.AppendRowsWithTimeRangeFilter(block.Timestamps[:0], block.Values[:0], uw.tr)
	rowsRead.Add(len(block.Timestamps))
	return nil
}

var blockPool = &sync.Pool{
	New: func() interface{} {
		return &storage.Block{}
	},
}

func getUnmarshalWork() *unmarshalWork {
	v := unmarshalWorkPool.Get()
	if v == nil {
		return &unmarshalWork{}
	}
	return v.(*unmarshalWork)
}

func putUnmarshalWork(uw *unmarshalWork) {
	uw.reset()
	unmarshalWorkPool.Put(uw)
}

var unmarshalWorkPool sync.Pool

func getBufferedReader(r io.Reader) *bufio.Reader {
	v := bufferedReaderPool.Get()
	if v == nil {
		return bufio.NewReaderSize(r, 64*1024)
	}
	br := v.(*bufio.Reader)
	br.Reset(r)
	return br
}

func putBufferedReader(br *bufio.Reader) {
	br.Reset(nil)
	bufferedReaderPool.Put(br)
}

var bufferedReaderPool sync.Pool
