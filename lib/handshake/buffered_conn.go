package handshake

import (
	"bufio"
	"io"
	"net"

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

// Read reads up to len(p) from bc to p.
func (bc *BufferedConn) Read(p []byte) (int, error) {
	return bc.br.Read(p)
}

// Write writes p to bc.
//
// Do not forget to call Flush if needed.
func (bc *BufferedConn) Write(p []byte) (int, error) {
	return bc.bw.Write(p)
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
