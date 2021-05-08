package clusternative

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/consts"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

// ParseStream parses data sent from vminsert to bc and calls callback for parsed rows.
//
// The callback can be called concurrently multiple times for streamed data from req.
//
// callback shouldn't hold block after returning.
func ParseStream(bc *handshake.BufferedConn, callback func(rows []storage.MetricRow) error) error {
	var wg sync.WaitGroup
	var (
		callbackErrLock sync.Mutex
		callbackErr     error
	)
	for {
		uw := getUnmarshalWork()
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
		var err error
		uw.reqBuf, err = readBlock(uw.reqBuf[:0], bc)
		if err != nil {
			wg.Wait()
			if err == io.EOF {
				// Remote end gracefully closed the connection.
				putUnmarshalWork(uw)
				return nil
			}
			return fmt.Errorf("cannot read packet size: %w", err)
		}
		blocksRead.Inc()
		wg.Add(1)
		common.ScheduleUnmarshalWork(uw)
	}
}

// readBlock reads the next data block from vminsert-initiated bc, appends it to dst and returns the result.
func readBlock(dst []byte, bc *handshake.BufferedConn) ([]byte, error) {
	sizeBuf := sizeBufPool.Get()
	defer sizeBufPool.Put(sizeBuf)
	sizeBuf.B = bytesutil.Resize(sizeBuf.B, 8)
	if _, err := io.ReadFull(bc, sizeBuf.B); err != nil {
		if err != io.EOF {
			readErrors.Inc()
			err = fmt.Errorf("cannot read packet size: %w", err)
		}
		return dst, err
	}
	packetSize := encoding.UnmarshalUint64(sizeBuf.B)
	if packetSize > consts.MaxInsertPacketSize {
		parseErrors.Inc()
		return dst, fmt.Errorf("too big packet size: %d; shouldn't exceed %d", packetSize, consts.MaxInsertPacketSize)
	}
	dstLen := len(dst)
	dst = bytesutil.Resize(dst, dstLen+int(packetSize))
	if n, err := io.ReadFull(bc, dst[dstLen:]); err != nil {
		readErrors.Inc()
		return dst, fmt.Errorf("cannot read packet with size %d bytes: %w; read only %d bytes", packetSize, err, n)
	}
	// Send `ack` to vminsert that the packet has been received.
	deadline := time.Now().Add(5 * time.Second)
	if err := bc.SetWriteDeadline(deadline); err != nil {
		writeErrors.Inc()
		return dst, fmt.Errorf("cannot set write deadline for sending `ack` to vminsert: %w", err)
	}
	sizeBuf.B[0] = 1
	if _, err := bc.Write(sizeBuf.B[:1]); err != nil {
		writeErrors.Inc()
		return dst, fmt.Errorf("cannot send `ack` to vminsert: %w", err)
	}
	if err := bc.Flush(); err != nil {
		writeErrors.Inc()
		return dst, fmt.Errorf("cannot flush `ack` to vminsert: %w", err)
	}
	return dst, nil
}

var sizeBufPool bytesutil.ByteBufferPool

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
	lastResetTime uint64
}

func (uw *unmarshalWork) reset() {
	if len(uw.reqBuf)*4 < cap(uw.reqBuf) && fasttime.UnixTimestamp()-uw.lastResetTime > 10 {
		// Periodically reset reqBuf and mrs in order to prevent from gradual memory usage growth
		// when ceratin entries in mr contain too long labels.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/490 for details.
		uw.reqBuf = nil
		uw.mrs = nil
		uw.lastResetTime = fasttime.UnixTimestamp()
	}
	uw.wg = nil
	uw.callback = nil
	uw.reqBuf = uw.reqBuf[:0]
	mrs := uw.mrs
	for i := range mrs {
		mrs[i].ResetX()
	}
	uw.mrs = uw.mrs[:0]
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	defer uw.wg.Done()
	if err := uw.unmarshal(); err != nil {
		parseErrors.Inc()
		logger.Errorf("error when unmarshaling clusternative block: %s", err)
		putUnmarshalWork(uw)
		return
	}
	mrs := uw.mrs
	for len(mrs) > maxRowsPerCallback {
		// Limit the number of rows passed to callback in order to reduce memory usage
		// when processing big packets of rows.
		uw.callback(mrs[:maxRowsPerCallback])
		mrs = mrs[maxRowsPerCallback:]
	}
	uw.callback(mrs)
	putUnmarshalWork(uw)
}

const maxRowsPerCallback = 10000

func (uw *unmarshalWork) unmarshal() error {
	var err error
	uw.mrs, err = storage.UnmarshalMetricRows(uw.mrs[:0], uw.reqBuf)
	if err != nil {
		return fmt.Errorf("cannot unmarshal MetricRow from clusternative block: %s", err)
	}
	rowsRead.Add(len(uw.mrs))
	return nil
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
