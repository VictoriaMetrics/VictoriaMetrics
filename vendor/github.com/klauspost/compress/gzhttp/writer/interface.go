// Package writer provides interfaces for pluggable compression implementations
// used by gzhttp. Custom gzip and zstd encoders can be provided by implementing
// the GzipWriter/ZstdWriter interfaces and corresponding factory types.
package writer

import (
	"io"
	"time"
)

// GzipWriter implements the functions needed for compressing content.
type GzipWriter interface {
	Write(p []byte) (int, error)
	Close() error
	Flush() error
}

// GzipWriterExt implements the functions needed for compressing content
// and optional extensions.
type GzipWriterExt interface {
	GzipWriter

	// SetHeader will populate header fields with non-nil values in h.
	SetHeader(h Header)
}

// Header is a gzip header.
type Header struct {
	Comment string    // comment
	Extra   []byte    // "extra data"
	ModTime time.Time // modification time
	Name    string    // file name
	OS      byte      // operating system type
}

// GzipWriterFactory contains the information needed for custom gzip implementations.
type GzipWriterFactory struct {
	// Must return the minimum and maximum supported level.
	Levels func() (min, max int)

	// New must return a new GzipWriter.
	// level will always be within the return limits above.
	New func(writer io.Writer, level int) GzipWriter
}

// ZstdWriter implements the functions needed for zstd compression.
type ZstdWriter interface {
	Write(p []byte) (int, error)
	Close() error
	Flush() error
}

// ZstdWriterFactory contains the information needed for custom zstd implementations.
type ZstdWriterFactory struct {
	// Must return the minimum and maximum supported level.
	Levels func() (min, max int)

	// New must return a new ZstdWriter.
	// level will always be within the return limits above.
	New func(writer io.Writer, level int) ZstdWriter
}
