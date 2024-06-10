package fslocal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FS represents local filesystem.
//
// Backups are made from local fs.
// Data is restored from backups to local fs.
type FS struct {
	// Dir is a path to local directory to work with.
	Dir string

	// MaxBytesPerSecond is the maximum bandwidth usage during backups or restores.
	MaxBytesPerSecond int

	bl *bandwidthLimiter
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init() error {
	if fs.MaxBytesPerSecond > 0 {
		fs.bl = newBandwidthLimiter(fs.MaxBytesPerSecond)
	}
	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	if fs.bl == nil {
		return
	}
	fs.bl.MustStop()
	fs.bl = nil
}

// String returns user-readable representation for the fs.
func (fs *FS) String() string {
	return fmt.Sprintf("fslocal %q", fs.Dir)
}

// ListParts returns all the parts for fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	dir := fs.Dir
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			// Return empty part list for non-existing directory.
			// The directory will be created later.
			return nil, nil
		}
		return nil, err
	}
	files, err := fscommon.AppendFiles(nil, dir)
	if err != nil {
		return nil, err
	}

	var parts []common.Part
	dir += string(filepath.Separator)
	for _, file := range files {
		if !strings.HasPrefix(file, dir) {
			logger.Panicf("BUG: unexpected prefix for file %q; want %q", file, dir)
		}
		fi, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("cannot stat %q: %w", file, err)
		}
		path := common.ToCanonicalPath(file[len(dir):])
		size := uint64(fi.Size())
		if size == 0 {
			parts = append(parts, common.Part{
				Path:   path,
				Offset: 0,
				Size:   0,
			})
			continue
		}
		offset := uint64(0)
		for offset < size {
			n := size - offset
			if n > common.MaxPartSize {
				n = common.MaxPartSize
			}
			parts = append(parts, common.Part{
				Path:       path,
				FileSize:   size,
				Offset:     offset,
				Size:       n,
				ActualSize: n,
			})
			offset += n
		}
	}
	return parts, nil
}

// NewReadCloser returns io.ReadCloser for the given part p located in fs.
func (fs *FS) NewReadCloser(p common.Part) (io.ReadCloser, error) {
	path := fs.path(p)
	r, err := filestream.OpenReaderAt(path, int64(p.Offset), true)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q at %q: %w", p.Path, fs.Dir, err)
	}
	lrc := &limitedReadCloser{
		r: r,
		n: p.Size,
	}
	if fs.bl == nil {
		return lrc, nil
	}
	blrc := fs.bl.NewReadCloser(lrc)
	return blrc, nil
}

// NewWriteCloser returns io.WriteCloser for the given part p located in fs.
func (fs *FS) NewWriteCloser(p common.Part) (io.WriteCloser, error) {
	path := fs.path(p)
	if err := fs.mkdirAll(path); err != nil {
		return nil, err
	}
	w, err := filestream.OpenWriterAt(path, int64(p.Offset), true)
	if err != nil {
		return nil, fmt.Errorf("cannot open writer for %q at offset %d: %w", path, p.Offset, err)
	}
	wc := &writeCloser{
		w:    w,
		n:    p.Size,
		path: path,
	}
	if fs.bl == nil {
		return wc, nil
	}
	blwc := fs.bl.NewWriteCloser(wc)
	return blwc, nil
}

// DeletePath deletes the given path from fs and returns the size for the deleted file.
//
// The path must be in canonical form, e.g. it must have `/` directory separators
func (fs *FS) DeletePath(path string) (uint64, error) {
	p := common.Part{
		Path: path,
	}
	fullPath := fs.path(p)
	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// The file could be deleted earlier via symlink.
			return 0, nil
		}
		return 0, fmt.Errorf("cannot open %q: %w", path, err)
	}
	fi, err := f.Stat()
	_ = f.Close()
	if err != nil {
		return 0, fmt.Errorf("cannot stat %q at %q: %w", path, fullPath, err)
	}
	size := uint64(fi.Size())
	if err := os.Remove(fullPath); err != nil {
		return 0, fmt.Errorf("cannot remove %q: %w", fullPath, err)
	}
	return size, nil
}

// RemoveEmptyDirs recursively removes all the empty directories in fs.
func (fs *FS) RemoveEmptyDirs() error {
	return fscommon.RemoveEmptyDirs(fs.Dir)
}

func (fs *FS) mkdirAll(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create directory %q: %w", dir, err)
	}
	return nil
}

func (fs *FS) path(p common.Part) string {
	return p.LocalPath(fs.Dir)
}

type limitedReadCloser struct {
	r *filestream.Reader
	n uint64
}

func (lrc *limitedReadCloser) Read(p []byte) (int, error) {
	if lrc.n == 0 {
		return 0, io.EOF
	}
	if uint64(len(p)) > lrc.n {
		p = p[:lrc.n]
	}
	n, err := lrc.r.Read(p)
	if n > len(p) {
		return n, fmt.Errorf("too much data read; got %d bytes; want %d bytes", n, len(p))
	}
	lrc.n -= uint64(n)
	return n, err
}

func (lrc *limitedReadCloser) Close() error {
	lrc.r.MustClose()
	return nil
}

type writeCloser struct {
	w    *filestream.Writer
	n    uint64
	path string
}

func (wc *writeCloser) Write(p []byte) (int, error) {
	n, err := wc.w.Write(p)
	if uint64(n) > wc.n {
		return n, fmt.Errorf("too much data written; got %d bytes; want %d bytes", n, wc.n)
	}
	wc.n -= uint64(n)
	return n, err
}

func (wc *writeCloser) Close() error {
	wc.w.MustFlush(true)
	wc.w.MustClose()
	if wc.n != 0 {
		return fmt.Errorf("missing data writes for %d bytes", wc.n)
	}

	return nil
}
