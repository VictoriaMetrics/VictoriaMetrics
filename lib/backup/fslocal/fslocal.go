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

	UseTmpFiles bool
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
			n := min(size-offset, common.MaxPartSize)
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

// NewDirectWriteCloser returns an io.WriteCloser that writes directly to the
// underlying file without buffering, enabling large IO sizes from the caller.
// On platforms with preallocation, writes go to a .tmp file that must be
// finalized with FinalizeFile.
func (fs *FS) NewDirectWriteCloser(p common.Part) (io.WriteCloser, error) {
	path := fs.writePath(p)
	if err := fs.mkdirAll(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %q: %w", path, err)
	}
	if _, err := f.Seek(int64(p.Offset), io.SeekStart); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("cannot seek to offset %d in %q: %w", p.Offset, path, err)
	}
	dwc := &directWriteCloser{
		f: f,
		n: p.Size,
	}
	if fs.bl == nil {
		return dwc, nil
	}
	// with a bandwidth limiter max throughput is not a concern
	// so we fallback to the filestream backed limited writerCloser
	return fs.bl.NewWriteCloser(dwc), nil
}

// PreallocateFile pre-allocates disk space for the file being written.
func (fs *FS) PreallocateFile(p common.Part) error {
	path := fs.writePath(p)
	if err := fs.mkdirAll(path); err != nil {
		return err
	}
	return preallocateFile(path, int64(p.FileSize))
}

// FinalizeFile syncs the completed file to disk. On platforms with
// preallocation, it first renames the .tmp file to its final path.
func (fs *FS) FinalizeFile(p common.Part) error {
	finalPath := fs.path(p)
	if canPreallocate && fs.UseTmpFiles {
		tmpPath := fs.tmpPath(p)
		if err := os.Rename(tmpPath, finalPath); err != nil {
			return fmt.Errorf("cannot rename %q to %q: %w", tmpPath, finalPath, err)
		}
	}
	f, err := os.Open(finalPath)
	if err != nil {
		return fmt.Errorf("cannot open %q for fsync: %w", finalPath, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("cannot fsync %q: %w", finalPath, err)
	}
	return f.Close()
}

// CleanupTmpFiles removes leftover .tmp files from interrupted restores.
// On platforms without preallocation this is a no-op.
func (fs *FS) CleanupTmpFiles() error {
	if !canPreallocate {
		return nil
	}
	if _, err := os.Stat(fs.Dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.Walk(fs.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".tmp") {
			logger.Infof("removing incomplete temporary file %q", path)
			return os.Remove(path)
		}
		return nil
	})
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

func (fs *FS) tmpPath(p common.Part) string {
	return fs.path(p) + ".tmp"
}

func (fs *FS) writePath(p common.Part) string {
	if canPreallocate && fs.UseTmpFiles {
		return fs.tmpPath(p)
	}
	return fs.path(p)
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

type directWriteCloser struct {
	f *os.File
	n uint64
}

func (dwc *directWriteCloser) Write(p []byte) (int, error) {
	n, err := dwc.f.Write(p)
	if uint64(n) > dwc.n {
		return n, fmt.Errorf("too much data written; got %d bytes; want %d bytes", n, dwc.n)
	}
	dwc.n -= uint64(n)
	return n, err
}

func (dwc *directWriteCloser) Close() error {
	if err := dwc.f.Close(); err != nil {
		return fmt.Errorf("cannot close file %q: %w", dwc.f.Name(), err)
	}
	if dwc.n != 0 {
		return fmt.Errorf("missing data writes for %d bytes", dwc.n)
	}
	return nil
}
