package stream

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses data sent from vminsert to bc and calls callback for parsed rows.
// Optional function isReadOnly must return true if the storage cannot accept new data.
// In this case the data read from bc isn't accepted and the readonly status is sent back bc.
//
// The callback can be called concurrently multiple times for streamed data from req.
//
// callback shouldn't hold block after returning.
func Parse(bc *handshake.BufferedConn, callback func(rows []storage.MetricRow) error, isReadOnly func() bool) error {
	wcr := writeconcurrencylimiter.GetReader(bc)
	defer writeconcurrencylimiter.PutReader(wcr)
	r := io.Reader(wcr)

	var wg sync.WaitGroup
	var (
		callbackErrLock sync.Mutex
		callbackErr     error
	)
	for {
		reqBuf, err := readBlock(nil, r, bc, isReadOnly)
		if err != nil {
			wg.Wait()
			if err == io.EOF {
				// Remote end gracefully closed the connection.
				return nil
			}
			return err
		}
		blocksRead.Inc()
		uw := getUnmarshalWork()
		uw.reqBuf = reqBuf
		uw.callback = func(rows []storage.MetricRow) {
			if err := callback(rows); err != nil {
				processErrors.Inc()
				callbackErrLock.Lock()
				if callbackErr == nil {
					callbackErr = fmt.Errorf("error when processing native block: %w", err)
				}
				callbackErrLock.Unlock()
			}
		}
		uw.wg = &wg
		wg.Add(1)
		common.ScheduleUnmarshalWork(uw)
		wcr.DecConcurrency()
	}
}

// readBlock reads the next data block from vminsert-initiated bc, appends it to dst and returns the result.
func readBlock(dst []byte, r io.Reader, bc *handshake.BufferedConn, isReadOnly func() bool) ([]byte, error) {
	sizeBuf := auxBufPool.Get()
	defer auxBufPool.Put(sizeBuf)
	sizeBuf.B = bytesutil.ResizeNoCopyMayOverallocate(sizeBuf.B, 8)
	if _, err := io.ReadFull(r, sizeBuf.B); err != nil {
		if err != io.EOF {
			readErrors.Inc()
			err = fmt.Errorf("cannot read packet size: %w", err)
		}
		return dst, err
	}
	packetSize := encoding.UnmarshalUint64(sizeBuf.B)
	if packetSize > consts.MaxInsertPacketSizeForVMStorage {
		parseErrors.Inc()
		return dst, fmt.Errorf("too big packet size: %d; shouldn't exceed %d", packetSize, consts.MaxInsertPacketSizeForVMStorage)
	}
	dstLen := len(dst)
	dst = bytesutil.ResizeWithCopyMayOverallocate(dst, dstLen+int(packetSize))
	if n, err := io.ReadFull(r, dst[dstLen:]); err != nil {
		readErrors.Inc()
		return dst, fmt.Errorf("cannot read packet with size %d bytes: %w; read only %d bytes", packetSize, err, n)
	}
	if isReadOnly != nil && isReadOnly() {
		// The vmstorage is in readonly mode, so drop the read block of data
		// and send `read only` status to vminsert.
		dst = dst[:dstLen]
		if err := sendAck(bc, 2); err != nil {
			writeErrors.Inc()
			return dst, fmt.Errorf("cannot send readonly status to vminsert: %w", err)
		}
		return dst, nil
	}
	// Send `ack` to vminsert that the packet has been received.
	if err := sendAck(bc, 1); err != nil {
		writeErrors.Inc()
		return dst, fmt.Errorf("cannot send `ack` to vminsert: %w", err)
	}
	return dst, nil
}

func sendAck(bc *handshake.BufferedConn, status byte) error {
	deadline := time.Now().Add(5 * time.Second)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("cannot set write deadline: %w", err)
	}
	b := auxBufPool.Get()
	defer auxBufPool.Put(b)
	b.B = append(b.B[:0], status)
	if _, err := bc.Write(b.B); err != nil {
		return err
	}
	return bc.Flush()
}

var auxBufPool bytesutil.ByteBufferPool

var (
	readErrors  = metrics.NewCounter(`vm_protoparser_read_errors_total{type="clusternative"}`)
	writeErrors = metrics.NewCounter(`vm_protoparser_write_errors_total{type="clusternative"}`)
	rowsRead    = metrics.NewCounter(`vm_protoparser_rows_read_total{type="clusternative"}`)
	blocksRead  = metrics.NewCounter(`vm_protoparser_blocks_read_total{type="clusternative"}`)

	parseErrors   = metrics.NewCounter(`vm_protoparser_parse_errors_total{type="clusternative"}`)
	processErrors = metrics.NewCounter(`vm_protoparser_process_errors_total{type="clusternative"}`)
)

type unmarshalWork struct {
	wg       *sync.WaitGroup
	callback func(rows []storage.MetricRow)
	reqBuf   []byte
	mrs      []storage.MetricRow
}

func (uw *unmarshalWork) reset() {
	uw.wg = nil
	uw.callback = nil
	// Zero reqBuf, since it may occupy big amounts of memory (consts.MaxInsertPacketSizeForVMStorage).
	uw.reqBuf = nil
	uw.mrs = uw.mrs[:0]
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	reqBuf := uw.reqBuf
	for len(reqBuf) > 0 {
		// Limit the number of rows passed to callback in order to reduce memory usage
		// when processing big packets of rows.
		mrs, tail, err := storage.UnmarshalMetricRows(uw.mrs[:0], reqBuf, maxRowsPerCallback)
		uw.mrs = mrs
		if err != nil {
			parseErrors.Inc()
			logger.Errorf("cannot unmarshal MetricRow from clusternative block with size %d (remaining %d bytes): %s", len(reqBuf), len(tail), err)
			break
		}
		rowsRead.Add(len(mrs))
		uw.callback(mrs)
		reqBuf = tail
	}
	wg := uw.wg
	wg.Done()
	putUnmarshalWork(uw)
}

const maxRowsPerCallback = 10000

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
