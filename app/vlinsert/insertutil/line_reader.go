package insertutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// LineReader reads newline-delimited lines from the underlying reader
type LineReader struct {
	// Line contains the next line read after the call to NextLine
	//
	// The Line contents is valid until the next call to NextLine.
	Line []byte

	// name is the LineReader name
	name string

	// r is the underlying reader to read data from
	r io.Reader

	// buf is a buffer for reading the next line
	buf []byte

	// bufOffset is the offset at buf to read the next line from
	bufOffset int

	// err is the last error when reading data from r
	err error

	// eofReached is set to true when all the data is read from r
	eofReached bool
}

// NewLineReader returns LineReader for r.
func NewLineReader(name string, r io.Reader) *LineReader {
	return &LineReader{
		name: name,
		r:    r,
	}
}

// NextLine reads the next line from the underlying reader.
//
// It returns true if the next line is successfully read into Line.
// If the line length exceeds MaxLineSizeBytes, then this line is skipped
// and an empty line is returned instead.
//
// If false is returned, then no more lines left to read from r.
// Check for Err in this case.
func (lr *LineReader) NextLine() bool {
	for {
		lr.Line = nil
		if lr.bufOffset >= len(lr.buf) {
			if lr.err != nil || lr.eofReached {
				return false
			}
			if !lr.readMoreData() {
				return false
			}
			if lr.bufOffset >= len(lr.buf) && lr.eofReached {
				return false
			}
		}

		buf := lr.buf[lr.bufOffset:]
		if n := bytes.IndexByte(buf, '\n'); n >= 0 {
			lr.Line = buf[:n]
			lr.bufOffset += n + 1
			return true
		}
		if lr.eofReached {
			lr.Line = buf
			lr.bufOffset += len(buf)
			return true
		}
		if !lr.readMoreData() {
			return false
		}
	}
}

// Err returns the last error after NextLine call.
func (lr *LineReader) Err() error {
	if lr.err == nil {
		return nil
	}
	return fmt.Errorf("%s: %s", lr.name, lr.err)
}

func (lr *LineReader) readMoreData() bool {
	if lr.bufOffset > 0 {
		lr.buf = append(lr.buf[:0], lr.buf[lr.bufOffset:]...)
		lr.bufOffset = 0
	}

	bufLen := len(lr.buf)
	if bufLen >= MaxLineSizeBytes.IntN() {
		ok, skippedBytes := lr.skipUntilNextLine()
		logger.Warnf("%s: the line length exceeds -insert.maxLineSizeBytes=%d; skipping it; total skipped bytes=%d",
			lr.name, MaxLineSizeBytes.IntN(), skippedBytes)
		tooLongLinesSkipped.Inc()
		return ok
	}

	lr.buf = slicesutil.SetLength(lr.buf, MaxLineSizeBytes.IntN())
	n, err := lr.r.Read(lr.buf[bufLen:])
	lr.buf = lr.buf[:bufLen+n]
	if err != nil {
		if errors.Is(err, io.EOF) {
			lr.eofReached = true
			return true
		}
		lr.err = fmt.Errorf("cannot read the next line: %s", err)
	}
	return n > 0
}

var tooLongLinesSkipped = metrics.NewCounter("vl_too_long_lines_skipped_total")

func (lr *LineReader) skipUntilNextLine() (bool, int) {

	// Initialize skipped bytes count with MaxLineSizeBytes because
	// we've already read that many bytes without encountering a newline,
	// indicating the line size exceeds the maximum allowed limit.
	skipSizeBytes := MaxLineSizeBytes.IntN()

	for {
		lr.buf = slicesutil.SetLength(lr.buf, MaxLineSizeBytes.IntN())
		n, err := lr.r.Read(lr.buf)
		skipSizeBytes += n
		lr.buf = lr.buf[:n]
		if err != nil {
			if errors.Is(err, io.EOF) {
				lr.eofReached = true
				lr.buf = lr.buf[:0]
				return true, skipSizeBytes
			}
			lr.err = fmt.Errorf("cannot skip the current line: %s", err)
			return false, skipSizeBytes
		}
		if n := bytes.IndexByte(lr.buf, '\n'); n >= 0 {
			// Include skipped bytes before \n, including the newline itself.
			skipSizeBytes += n + 1 - len(lr.buf)
			// Include \n in the buf, so too long line is replaced with an empty line.
			// This is needed for maintaining synchorinzation consistency between lines
			// in protocols such as Elasticsearch bulk import.
			lr.buf = append(lr.buf[:0], lr.buf[n:]...)
			return true, skipSizeBytes
		}
	}
}
