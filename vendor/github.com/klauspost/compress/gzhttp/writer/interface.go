package writer

import "io"

// GzipWriter implements the functions needed for compressing content.
type GzipWriter interface {
	Write(p []byte) (int, error)
	Close() error
	Flush() error
}

// GzipWriterFactory contains the information needed for custom gzip implementations.
type GzipWriterFactory struct {
	// Must return the minimum and maximum supported level.
	Levels func() (min, max int)

	// New must return a new GzipWriter.
	// level will always be within the return limits above.
	New func(writer io.Writer, level int) GzipWriter
}
