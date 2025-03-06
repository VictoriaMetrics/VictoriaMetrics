package zstd

import "io"

// Reader is a common interface for cgo and pure implementations
type Reader interface {
	io.Reader
	Close()
	Reset(io.Reader) error
}
