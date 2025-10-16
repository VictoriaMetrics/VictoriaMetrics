package stream

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
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
	wrapErr := func(err error, isWriteErr bool) error {
		wg.Wait()
		if err == io.EOF {
			// Remote end gracefully closed the connection.
			return callbackErr
		}
		if isWriteErr {
			writeErrors.Inc()
		}
		return errors.Join(err, callbackErr)
	}
	for {
		reqBuf, err := readBlock(r)
		if err != nil {
			return wrapErr(err, false)
		}
		blocksRead.Inc()
		if isReadOnly != nil && isReadOnly() {
			// The vmstorage is in readonly mode, so drop the read block of data
			// and send `read only` status to vminsert.
			if err := sendAck(bc, consts.StorageStatusReadOnly); err != nil {
				return wrapErr(fmt.Errorf("cannot send readonly status to vminsert: %w", err), true)
			}
			wcr.DecConcurrency()
			continue
		}

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
		protoparserutil.ScheduleUnmarshalWork(uw)
		wcr.DecConcurrency()
		// Send `ack` to vminsert that the packet has been received and scheduled for processing
		if err := sendAck(bc, consts.StorageStatusAck); err != nil {
			return wrapErr(fmt.Errorf("cannot send `ack` to vminsert: %w", err), true)
		}
	}
}

// ParseBlock parses data block sent from vminsert to bc and calls callback for parsed rows.
// Optional function isReadOnly must return true if the storage cannot accept new data.
// In this case the data read from bc isn't accepted and storage.ErrReadOnly returned.
//
// callback shouldn't hold block after returning.
func ParseBlock(bc *handshake.BufferedConn, callback func(rows []storage.MetricRow) error, isReadOnly func() bool) error {
	wcr := writeconcurrencylimiter.GetReader(bc)
	defer writeconcurrencylimiter.PutReader(wcr)
	r := io.Reader(wcr)

	reqBuf, err := readBlock(r)
	if err != nil {
		if err == io.EOF {
			// Remote end gracefully closed the connection.
			return nil
		}

		return err
	}
	blocksRead.Inc()
	wcr.DecConcurrency()
	if isReadOnly != nil && isReadOnly() {
		return storage.ErrReadOnly
	}

	uw := getUnmarshalWork()
	uw.isZSTDEncoded = true
	uw.reqBuf = reqBuf
	uw.callback = func(rows []storage.MetricRow) {
		if err := callback(rows); err != nil {
			processErrors.Inc()
			logger.Errorf("error when processing native block: %s", err)
		}
	}
	protoparserutil.ScheduleUnmarshalWork(uw)
	return nil
}

// readBlock reads the next data block  and returns the result.
func readBlock(r io.Reader) ([]byte, error) {
	sizeBuf := auxBufPool.Get()
	defer auxBufPool.Put(sizeBuf)
	sizeBuf.B = bytesutil.ResizeNoCopyMayOverallocate(sizeBuf.B, 8)
	if _, err := io.ReadFull(r, sizeBuf.B); err != nil {
		if err != io.EOF {
			readErrors.Inc()
			err = fmt.Errorf("cannot read packet size: %w", err)
		}
		return nil, err
	}
	packetSize := encoding.UnmarshalUint64(sizeBuf.B)
	if packetSize > consts.MaxInsertPacketSizeForVMStorage {
		parseErrors.Inc()
		return nil, fmt.Errorf("too big packet size: %d; shouldn't exceed %d", packetSize, consts.MaxInsertPacketSizeForVMStorage)
	}
	dst := make([]byte, int(packetSize))
	if n, err := io.ReadFull(r, dst); err != nil {
		readErrors.Inc()
		return dst, fmt.Errorf("cannot read packet with size %d bytes: %w; read only %d bytes", packetSize, err, n)
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
	wg            *sync.WaitGroup
	callback      func(rows []storage.MetricRow)
	reqBuf        []byte
	mrs           []storage.MetricRow
	isZSTDEncoded bool
}

func (uw *unmarshalWork) reset() {
	uw.wg = nil
	uw.callback = nil
	// Zero reqBuf, since it may occupy big amounts of memory (consts.MaxInsertPacketSizeForVMStorage).
	uw.reqBuf = nil
	uw.mrs = uw.mrs[:0]
	uw.isZSTDEncoded = false
}

// Unmarshal implements protoparserutil.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	reqBuf := uw.reqBuf
	if uw.isZSTDEncoded {
		zb := zbPool.Get()
		defer zbPool.Put(zb)
		var err error
		zb.B, err = zstd.Decompress(zb.B[:0], reqBuf)
		if err != nil {
			parseErrors.Inc()
			logger.Errorf("cannot decompress clusternative block with size %d : %s", len(reqBuf), err)
			uw.wg.Done()
			putUnmarshalWork(uw)
			return
		}
		reqBuf = zb.B
	}
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
	if wg != nil {
		wg.Done()
	}
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

var zbPool bytesutil.ByteBufferPool
