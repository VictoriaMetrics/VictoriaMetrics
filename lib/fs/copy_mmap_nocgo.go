// +build !cgo

package fs

// copyMmap copies len(dst) bytes from src to dst.
func copyMmap(dst, src []byte) {
	// This may lead to goroutines stalls when the copied data isn't available in RAM.
	// In this case the OS triggers reading the data from file.
	// See https://medium.com/@valyala/mmap-in-go-considered-harmful-d92a25cb161d for details.
	// TODO: fix this
	copy(dst, src)
}
