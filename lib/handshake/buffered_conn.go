package handshake

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
)

type bufferedWriter interface {
	Write(p []byte) (int, error)
	Flush() error
}

// BufferedConn is a net.Conn with Flush suport.
type BufferedConn struct {
	net.Conn

	br io.Reader
	bw bufferedWriter

	readDeadline  time.Time
	writeDeadline time.Time
}

const bufferSize = 64 * 1024

// newBufferedConn returns buffered connection with the given compression level.
func newBufferedConn(c net.Conn, compressionLevel int, isReadCompressed bool) *BufferedConn {
	bc := &BufferedConn{
		Conn: c,
	}
	if compressionLevel <= 0 {
		bc.bw = bufio.NewWriterSize(c, bufferSize)
	} else {
		bc.bw = zstd.NewWriterLevel(c, compressionLevel)
	}
	if !isReadCompressed {
		bc.br = bufio.NewReaderSize(c, bufferSize)
	} else {
		bc.br = zstd.NewReader(c)
	}
	return bc
}

// SetDeadline sets read and write deadlines for bc to t.
//
// Deadline is checked on each Read and Write call.
func (bc *BufferedConn) SetDeadline(t time.Time) error {
	bc.readDeadline = t
	bc.writeDeadline = t
	return bc.Conn.SetDeadline(t)
}

// SetReadDeadline sets read deadline for bc to t.
//
// Deadline is checked on each Read call.
func (bc *BufferedConn) SetReadDeadline(t time.Time) error {
	bc.readDeadline = t
	return bc.Conn.SetReadDeadline(t)
}

// SetWriteDeadline sets write deadline for bc to t.
//
// Deadline is checked on each Write call.
func (bc *BufferedConn) SetWriteDeadline(t time.Time) error {
	bc.writeDeadline = t
	return bc.Conn.SetWriteDeadline(t)
}

// Read reads up to len(p) from bc to p.
func (bc *BufferedConn) Read(p []byte) (int, error) {
	startTime := time.Now()
	if deadlineExceeded(bc.readDeadline, startTime) {
		return 0, os.ErrDeadlineExceeded
	}
	n, err := bc.br.Read(p)
	if err != nil && err != io.EOF {
		err = fmt.Errorf("cannot read data in %.3f seconds: %w", time.Since(startTime).Seconds(), err)
	}
	return n, err
}

// Write writes p to bc.
//
// Do not forget to call Flush if needed.
func (bc *BufferedConn) Write(p []byte) (int, error) {
	startTime := time.Now()
	if deadlineExceeded(bc.writeDeadline, startTime) {
		return 0, os.ErrDeadlineExceeded
	}
	n, err := bc.bw.Write(p)
	if err != nil {
		err = fmt.Errorf("cannot write data in %.3f seconds: %w", time.Since(startTime).Seconds(), err)
	}
	return n, err
}

func deadlineExceeded(deadline, currentTime time.Time) bool {
	if deadline.IsZero() {
		return false
	}
	return currentTime.After(deadline)
}

// Close closes bc.
func (bc *BufferedConn) Close() error {
	// Close the Conn at first. It is expected that all the required data
	// is already flushed to the Conn.
	err := bc.Conn.Close()
	bc.Conn = nil

	if zr, ok := bc.br.(*zstd.Reader); ok {
		zr.Release()
	}
	bc.br = nil

	if zw, ok := bc.bw.(*zstd.Writer); ok {
		// Do not call zw.Close(), since we already closed the underlying conn.
		zw.Release()
	}
	bc.bw = nil

	return err
}

// Flush flushes internal write buffers to the underlying conn.
func (bc *BufferedConn) Flush() error {
	return bc.bw.Flush()
}
