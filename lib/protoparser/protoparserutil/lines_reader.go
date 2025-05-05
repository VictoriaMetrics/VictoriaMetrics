package protoparserutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// The maximum size of a single line returned by ReadLinesBlock.
const maxLineSize = 256 * 1024

// Default size in bytes of a single block returned by ReadLinesBlock.
const defaultBlockSize = 64 * 1024

// ReadLinesBlock reads a block of lines delimited by '\n' from tailBuf and r into dstBuf.
//
// Trailing chars after the last newline are put into tailBuf.
//
// Returns (dstBuf, tailBuf).
//
// It is expected that read timeout on r exceeds 1 second.
func ReadLinesBlock(r io.Reader, dstBuf, tailBuf []byte) ([]byte, []byte, error) {
	return ReadLinesBlockExt(r, dstBuf, tailBuf, maxLineSize)
}

// ReadLinesBlockExt reads a block of lines delimited by '\n' from tailBuf and r into dstBuf.
//
// Trailing chars after the last newline are put into tailBuf.
//
// Returns (dstBuf, tailBuf).
//
// maxLineLen limits the maximum length of a single line.
//
// It is expected that read timeout on r exceeds 1 second.
func ReadLinesBlockExt(r io.Reader, dstBuf, tailBuf []byte, maxLineLen int) ([]byte, []byte, error) {
	startTime := time.Now()
	if cap(dstBuf) < defaultBlockSize {
		dstBuf = bytesutil.ResizeNoCopyNoOverallocate(dstBuf, defaultBlockSize)
	}
	dstBuf = append(dstBuf[:0], tailBuf...)
	tailBuf = tailBuf[:0]
again:
	n, err := r.Read(dstBuf[len(dstBuf):cap(dstBuf)])
	// Check for error only if zero bytes read from r, i.e. no forward progress made.
	// Otherwise process the read data.
	if n == 0 {
		if err == nil {
			return dstBuf, tailBuf, fmt.Errorf("no forward progress made")
		}
		isEOF := isEOFLikeError(err)
		if isEOF && len(dstBuf) > 0 {
			// Missing newline in the end of stream. This is OK,
			// so suppress io.EOF for now. It will be returned during the next
			// call to ReadLinesBlock.
			// This fixes https://github.com/VictoriaMetrics/VictoriaMetrics/issues/60 .
			return dstBuf, tailBuf, nil
		}
		if !isEOF {
			err = fmt.Errorf("cannot read a block of data in %.3fs: %w", time.Since(startTime).Seconds(), err)
		} else {
			err = io.EOF
		}
		return dstBuf, tailBuf, err
	}
	dstBuf = dstBuf[:len(dstBuf)+n]

	// Search for the last newline in dstBuf and put the rest into tailBuf.
	nn := bytes.LastIndexByte(dstBuf[len(dstBuf)-n:], '\n')
	if nn < 0 {
		// Didn't find at least a single line.
		if len(dstBuf) > maxLineLen {
			return dstBuf, tailBuf, fmt.Errorf("too long line: more than %d bytes", maxLineLen)
		}
		if cap(dstBuf) < 2*len(dstBuf) {
			// Increase dsbBuf capacity, so more data could be read into it.
			dstBufLen := len(dstBuf)
			dstBuf = bytesutil.ResizeWithCopyNoOverallocate(dstBuf, 2*cap(dstBuf))
			dstBuf = dstBuf[:dstBufLen]
		}
		goto again
	}

	// Found at least a single line. Return it.
	nn += len(dstBuf) - n
	tailBuf = append(tailBuf[:0], dstBuf[nn+1:]...)
	dstBuf = dstBuf[:nn]
	return dstBuf, tailBuf, nil
}

func isEOFLikeError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "reset by peer")
}
