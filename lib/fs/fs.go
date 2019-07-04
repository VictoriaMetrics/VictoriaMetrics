package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// ReadAtCloser is rand-access read interface.
type ReadAtCloser interface {
	// ReadAt must read len(p) bytes from offset off to p.
	ReadAt(p []byte, off int64)

	// MustClose must close the reader.
	MustClose()
}

// ReaderAt implements rand-access read.
type ReaderAt struct {
	f *os.File
}

// ReadAt reads len(p) bytes from off to p.
func (ra *ReaderAt) ReadAt(p []byte, off int64) {
	if len(p) == 0 {
		return
	}
	n, err := ra.f.ReadAt(p, off)
	if err != nil {
		logger.Panicf("FATAL: cannot read %d bytes at offset %d of file %q: %s", len(p), off, ra.f.Name(), err)
	}
	if n != len(p) {
		logger.Panicf("FATAL: unexpected number of bytes read; got %d; want %d", n, len(p))
	}
	readCalls.Inc()
	readBytes.Add(len(p))
}

// MustClose closes ra.
func (ra *ReaderAt) MustClose() {
	if err := ra.f.Close(); err != nil {
		logger.Panicf("FATAL: cannot close file %q: %s", ra.f.Name(), err)
	}
	readersCount.Dec()
}

// OpenReaderAt opens a file on the given path for random-read access.
//
// The file must be closed with MustClose when no longer needed.
func OpenReaderAt(path string) (*ReaderAt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	readersCount.Inc()
	ra := &ReaderAt{
		f: f,
	}
	return ra, nil
}

var (
	readCalls    = metrics.NewCounter(`vm_fs_read_calls_total`)
	readBytes    = metrics.NewCounter(`vm_fs_read_bytes_total`)
	readersCount = metrics.NewCounter(`vm_fs_readers`)
)

// MustSyncPath syncs contents of the given path.
func MustSyncPath(path string) {
	d, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot open %q: %s", path, err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		logger.Panicf("FATAL: cannot flush %q to storage: %s", path, err)
	}
	if err := d.Close(); err != nil {
		logger.Panicf("FATAL: cannot close %q: %s", path, err)
	}
}

// WriteFile writes data to the given file path.
//
// WriteFile returns only after the file is fully written
// to the underlying storage.
func WriteFile(path string, data []byte) error {
	if IsPathExist(path) {
		return fmt.Errorf("cannot create file %q, since it already exists", path)
	}
	f, err := filestream.Create(path, false)
	if err != nil {
		return fmt.Errorf("cannot create file %q: %s", path, err)
	}
	if _, err := f.Write(data); err != nil {
		f.MustClose()
		return fmt.Errorf("cannot write %d bytes to file %q: %s", len(data), path, err)
	}

	// Sync and close the file.
	f.MustClose()

	// Sync the containing directory, so the file is guaranteed to appear in the directory.
	// See https://www.quora.com/When-should-you-fsync-the-containing-directory-in-addition-to-the-file-itself
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot obtain absolute path to %q: %s", path, err)
	}
	parentDirPath := filepath.Dir(absPath)
	MustSyncPath(parentDirPath)

	return nil
}

// MkdirAllIfNotExist creates the given path dir if it isn't exist.
func MkdirAllIfNotExist(path string) error {
	if IsPathExist(path) {
		return nil
	}
	return mkdirSync(path)
}

// MkdirAllFailIfExist creates the given path dir if it isn't exist.
//
// Returns error if path already exists.
func MkdirAllFailIfExist(path string) error {
	if IsPathExist(path) {
		return fmt.Errorf("the %q already exists", path)
	}
	return mkdirSync(path)
}

func mkdirSync(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	// Sync the parent directory, so the created directory becomes visible
	// in the fs after power loss.
	parentDirPath := filepath.Dir(path)
	MustSyncPath(parentDirPath)
	return nil
}

// RemoveDirContents removes all the contents of the given dir it it exists.
//
// It doesn't remove the dir itself, so the dir may be mounted
// to a separate partition.
func RemoveDirContents(dir string) {
	if !IsPathExist(dir) {
		// The path doesn't exist, so nothing to remove.
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		logger.Panicf("FATAL: cannot open dir %q: %s", dir, err)
	}
	defer MustClose(d)
	names, err := d.Readdirnames(-1)
	if err != nil {
		logger.Panicf("FATAL: cannot read contents of the dir %q: %s", dir, err)
	}
	for _, name := range names {
		if name == "." || name == ".." || name == "lost+found" {
			// Skip special dirs.
			continue
		}
		fullPath := dir + "/" + name
		MustRemoveAll(fullPath)
	}
	MustSyncPath(dir)
}

// MustClose must close the given file f.
func MustClose(f *os.File) {
	fname := f.Name()
	if err := f.Close(); err != nil {
		logger.Panicf("FATAL: cannot close %q: %s", fname, err)
	}
}

// MustFileSize returns file size for the given path.
func MustFileSize(path string) uint64 {
	fi, err := os.Stat(path)
	if err != nil {
		logger.Panicf("FATAL: cannot stat %q: %s", path, err)
	}
	if fi.IsDir() {
		logger.Panicf("FATAL: %q must be a file, not a directory", path)
	}
	return uint64(fi.Size())
}

// IsPathExist returns whether the given path exists.
func IsPathExist(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		logger.Panicf("FATAL: cannot stat %q: %s", path, err)
	}
	return true
}

func mustSyncParentDirIfExists(path string) {
	parentDirPath := filepath.Dir(path)
	if !IsPathExist(parentDirPath) {
		return
	}
	MustSyncPath(parentDirPath)
}

// MustRemoveAll removes path with all the contents.
//
// It properly handles NFS issue https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 .
func MustRemoveAll(path string) {
	err := os.RemoveAll(path)
	if err == nil {
		// Make sure the parent directory doesn't contain references
		// to the current directory.
		mustSyncParentDirIfExists(path)
		return
	}
	if !isTemporaryNFSError(err) {
		logger.Panicf("FATAL: cannot remove %q: %s", path, err)
	}
	// NFS prevents from removing directories with open files.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 .
	// Schedule for later directory removal.
	select {
	case removeDirCh <- path:
	default:
		logger.Panicf("FATAL: cannot schedule %s for removal, since the removal queue is full (%d entries)", path, cap(removeDirCh))
	}
}

var removeDirCh = make(chan string, 1024)

func dirRemover() {
	for path := range removeDirCh {
		attempts := 0
		for {
			err := os.RemoveAll(path)
			if err == nil {
				break
			}
			if !isTemporaryNFSError(err) {
				logger.Panicf("FATAL: cannot remove %q: %s", path, err)
			}
			// NFS prevents from removing directories with open files.
			// Sleep for a while and try again in the hope open files will be closed.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 .
			attempts++
			if attempts > 10 {
				logger.Panicf("FATAL: cannot remove %q in %d attempts: %s", path, attempts, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
		// Make sure the parent directory doesn't contain references
		// to the current directory.
		mustSyncParentDirIfExists(path)
	}
}

func isTemporaryNFSError(err error) bool {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 for details.
	errStr := err.Error()
	return strings.Contains(errStr, "directory not empty") || strings.Contains(errStr, "device or resource busy")
}

func init() {
	go dirRemover()
}

// HardLinkFiles makes hard links for all the files from srcDir in dstDir.
func HardLinkFiles(srcDir, dstDir string) error {
	if err := mkdirSync(dstDir); err != nil {
		return fmt.Errorf("cannot create dstDir=%q: %s", dstDir, err)
	}

	d, err := os.Open(srcDir)
	if err != nil {
		return fmt.Errorf("cannot open srcDir=%q: %s", srcDir, err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			logger.Panicf("FATAL: cannot close %q: %s", srcDir, err)
		}
	}()

	fis, err := d.Readdir(-1)
	if err != nil {
		return fmt.Errorf("cannot read files in scrDir=%q: %s", srcDir, err)
	}
	for _, fi := range fis {
		if IsDirOrSymlink(fi) {
			// Skip directories.
			continue
		}
		fn := fi.Name()
		srcPath := srcDir + "/" + fn
		dstPath := dstDir + "/" + fn
		if err := os.Link(srcPath, dstPath); err != nil {
			return err
		}
	}

	MustSyncPath(dstDir)
	return nil
}

// IsDirOrSymlink returns true if fi is directory or symlink.
func IsDirOrSymlink(fi os.FileInfo) bool {
	return fi.IsDir() || (fi.Mode()&os.ModeSymlink == os.ModeSymlink)
}

// SymlinkRelative creates relative symlink for srcPath in dstPath.
func SymlinkRelative(srcPath, dstPath string) error {
	baseDir := filepath.Dir(dstPath)
	srcPathRel, err := filepath.Rel(baseDir, srcPath)
	if err != nil {
		return fmt.Errorf("cannot make relative path for srcPath=%q: %s", srcPath, err)
	}
	return os.Symlink(srcPathRel, dstPath)
}

// ReadFullData reads len(data) bytes from r.
func ReadFullData(r io.Reader, data []byte) error {
	n, err := io.ReadFull(r, data)
	if err != nil {
		if err == io.EOF {
			return io.EOF
		}
		return fmt.Errorf("cannot read %d bytes; read only %d bytes; error: %s", len(data), n, err)
	}
	if n != len(data) {
		logger.Panicf("BUG: io.ReadFull read only %d bytes; must read %d bytes", n, len(data))
	}
	return nil
}

// MustWriteData writes data to w.
func MustWriteData(w io.Writer, data []byte) {
	if len(data) == 0 {
		return
	}
	n, err := w.Write(data)
	if err != nil {
		logger.Panicf("FATAL: cannot write %d bytes: %s", len(data), err)
	}
	if n != len(data) {
		logger.Panicf("BUG: writer wrote %d bytes instead of %d bytes", n, len(data))
	}
}
