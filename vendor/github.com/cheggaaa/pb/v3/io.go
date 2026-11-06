package pb

import (
	"io"
)

// Reader it's a wrapper for given reader, but with progress handle
type Reader struct {
	io.Reader
	bar *ProgressBar
}

// Read reads bytes from wrapped reader and add amount of bytes to progress bar
func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.bar.Add(n)
	return
}

// Close the wrapped reader when it implements io.Closer
func (r *Reader) Close() (err error) {
	r.bar.Finish()
	if closer, ok := r.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return
}

// ReadSeeker wraps an io.ReadSeeker with progress tracking.
// Reads advance the bar, and successful seeks set its current value to the
// resulting position. Seeking may make speed and remaining-time estimates inaccurate.
type ReadSeeker struct {
	io.ReadSeeker
	bar *ProgressBar
}

// Read reads bytes from the wrapped reader and advances the progress bar.
func (r *ReadSeeker) Read(p []byte) (n int, err error) {
	return (&Reader{r.ReadSeeker, r.bar}).Read(p)
}

// Seek seeks in the wrapped reader and updates progress only on success.
func (r *ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	position, err := r.ReadSeeker.Seek(offset, whence)
	if err == nil {
		r.bar.SetCurrent(position)
	}
	return position, err
}

// Close finishes the bar and closes the wrapped reader when it implements io.Closer.
func (r *ReadSeeker) Close() error {
	return (&Reader{r.ReadSeeker, r.bar}).Close()
}

// Writer it's a wrapper for given writer, but with progress handle
type Writer struct {
	io.Writer
	bar *ProgressBar
}

// Write writes bytes to wrapped writer and add amount of bytes to progress bar
func (r *Writer) Write(p []byte) (n int, err error) {
	n, err = r.Writer.Write(p)
	r.bar.Add(n)
	return
}

// Close the wrapped reader when it implements io.Closer
func (r *Writer) Close() (err error) {
	r.bar.Finish()
	if closer, ok := r.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return
}
