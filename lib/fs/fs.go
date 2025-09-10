package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fsutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var tmpFileNum atomicutil.Uint64

// MustSyncPathAndParentDir fsyncs the path and the parent dir.
//
// This guarantees that the path is visible and readable after unclean shutdown.
func MustSyncPathAndParentDir(path string) {
	MustSyncPath(path)
	parentDirPath := filepath.Dir(path)
	MustSyncPath(parentDirPath)
}

// MustSyncPath syncs contents of the given path.
func MustSyncPath(path string) {
	if fsutil.IsFsyncDisabled() {
		// Just check that the path exists
		if !IsPathExist(path) {
			logger.Panicf("FATAL: cannot fsync missing %q", path)
		}
		return
	}
	mustSyncPath(path)
}

// MustWriteSync writes data to the file at path and then calls fsync on the created file.
//
// The fsync guarantees that the written data survives hardware reset after successful call.
//
// This function may leave the file at the path in inconsistent state on app crash
// in the middle of the write.
// Use MustWriteAtomic if the file at the path must be either written in full
// or not written at all on app crash in the middle of the write.
func MustWriteSync(path string, data []byte) {
	f := filestream.MustCreate(path, false)
	if _, err := f.Write(data); err != nil {
		f.MustClose()
		// Do not call MustRemovePath(path), so the user could inspect
		// the file contents during investigation of the issue.
		logger.Panicf("FATAL: cannot write %d bytes to %q: %s", len(data), path, err)
	}
	f.MustClose()
}

// MustWriteAtomic atomically writes data to the given file path.
//
// This function returns only after the file is fully written and synced
// to the underlying storage.
//
// This function guarantees that the file at path either fully written or not written at all on app crash
// in the middle of the write.
//
// If the file at path already exists, then the file is overwritten atomically if canOverwrite is true.
// Otherwise, error is returned.
func MustWriteAtomic(path string, data []byte, canOverwrite bool) {
	// Check for the existing file. It is expected that
	// the MustWriteAtomic function cannot be called concurrently
	// with the same `path`.
	if IsPathExist(path) && !canOverwrite {
		logger.Panicf("FATAL: cannot create file %q, since it already exists", path)
	}

	// Write data to a temporary file.
	n := tmpFileNum.Add(1)
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, n)
	MustWriteSync(tmpPath, data)

	// Atomically move the temporary file from tmpPath to path.
	if err := os.Rename(tmpPath, path); err != nil {
		// do not call MustRemovePath(tmpPath) here, so the user could inspect
		// the file contents during investigation of the issue.
		logger.Panicf("FATAL: cannot move temporary file %q to %q: %s", tmpPath, path, err)
	}

	// Sync the containing directory, so the file is guaranteed to appear in the directory.
	// See https://www.quora.com/When-should-you-fsync-the-containing-directory-in-addition-to-the-file-itself
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.Panicf("FATAL: cannot obtain absolute path to %q: %s", path, err)
	}
	parentDirPath := filepath.Dir(absPath)
	MustSyncPath(parentDirPath)
}

// IsTemporaryFileName returns true if fn matches temporary file name pattern
// from MustWriteAtomic.
func IsTemporaryFileName(fn string) bool {
	return tmpFileNameRe.MatchString(fn)
}

// tmpFileNameRe is regexp for temporary file name - see MustWriteAtomic for details.
var tmpFileNameRe = regexp.MustCompile(`\.tmp\.\d+$`)

// MustMkdirIfNotExist creates the given path dir if it isn't exist.
//
// The caller is responsible for MustSyncPath() call for the parent directory for the path.
func MustMkdirIfNotExist(path string) {
	if IsPathExist(path) {
		return
	}
	mustMkdir(path)
}

// MustMkdirFailIfExist creates the given path dir if it isn't exist.
//
// If the directory at the given path already exists, then the function logs the fatal error and exits the process.
//
// The caller is responsible for MustSyncPath() call for the parent directory for the path.
func MustMkdirFailIfExist(path string) {
	if IsPathExist(path) {
		logger.Panicf("FATAL: the %q already exists", path)
	}
	mustMkdir(path)
}

func mustMkdir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logger.Panicf("FATAL: cannot create directory: %s", err)
	}
	// Do not sync the parent directory - this is the responsibility of the caller.
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

// MustReadDir reads directory entries at the given dir.
func MustReadDir(dir string) []os.DirEntry {
	des, err := os.ReadDir(dir)
	if err != nil {
		logger.Panicf("FATAL: cannot read directory contents: %s", err)
	}
	return des
}

// MustHardLinkFiles creates dstDir and makes hard links for all the files from srcDir in dstDir.
//
// The caller is responsible for calling MustSyncPath for the parent directory of dstDir.
func MustHardLinkFiles(srcDir, dstDir string) {
	mustMkdir(dstDir)

	des := MustReadDir(srcDir)
	for _, de := range des {
		if IsDirOrSymlink(de) {
			// Skip directories.
			continue
		}
		fn := de.Name()
		srcPath := filepath.Join(srcDir, fn)
		dstPath := filepath.Join(dstDir, fn)
		if err := os.Link(srcPath, dstPath); err != nil {
			logger.Panicf("FATAL: cannot link files: %s", err)
		}
	}

	MustSyncPath(dstDir)
}

// MustSymlinkRelative creates relative symlink for srcPath in dstPath.
//
// The caller is responsible for calling MustSyncPath() for the parent directory of dstPath.
func MustSymlinkRelative(srcPath, dstPath string) {
	baseDir := filepath.Dir(dstPath)
	srcPathRel, err := filepath.Rel(baseDir, srcPath)
	if err != nil {
		logger.Panicf("FATAL: cannot make relative path for srcPath=%q: %s", srcPath, err)
	}
	if err := os.Symlink(srcPathRel, dstPath); err != nil {
		logger.Panicf("FATAL: cannot make a symlink: %s", err)
	}
}

// MustCopyDirectory creates dstPath and copies all the files in srcPath to dstPath.
//
// The caller is responsible for calling MustSyncPath() for the parent directory of dstPath.
func MustCopyDirectory(srcPath, dstPath string) {
	mustMkdir(dstPath)

	des := MustReadDir(srcPath)
	for _, de := range des {
		if !de.Type().IsRegular() {
			// Skip non-files
			continue
		}
		src := filepath.Join(srcPath, de.Name())
		dst := filepath.Join(dstPath, de.Name())
		MustCopyFile(src, dst)
	}

	MustSyncPath(dstPath)
}

// MustCopyFile copies the file from srcPath to dstPath.
func MustCopyFile(srcPath, dstPath string) {
	src, err := os.Open(srcPath)
	if err != nil {
		logger.Panicf("FATAL: cannot open srcPath: %s", err)
	}
	defer MustClose(src)
	dst, err := os.Create(dstPath)
	if err != nil {
		logger.Panicf("FATAL: cannot create dstPath: %s", err)
	}
	defer MustClose(dst)
	if _, err := io.Copy(dst, src); err != nil {
		logger.Panicf("FATAL: cannot copy %q to %q: %s", srcPath, dstPath, err)
	}
	MustSyncPath(dstPath)
}

// MustReadData reads len(data) bytes from r.
func MustReadData(r filestream.ReadCloser, data []byte) {
	n, err := io.ReadFull(r, data)
	if err != nil {
		if err == io.EOF {
			return
		}
		logger.Panicf("FATAL: cannot read %d bytes from %s; read only %d bytes; error: %s", len(data), r.Path(), n, err)
	}
	if n != len(data) {
		logger.Panicf("BUG: io.ReadFull read only %d bytes from %s; must read %d bytes", n, r.Path(), len(data))
	}
}

// MustWriteData writes data to w.
func MustWriteData(w filestream.WriteCloser, data []byte) {
	if len(data) == 0 {
		return
	}
	n, err := w.Write(data)
	if err != nil {
		logger.Panicf("FATAL: cannot write %d bytes to %s: %s", len(data), w.Path(), err)
	}
	if n != len(data) {
		logger.Panicf("BUG: writer wrote %d bytes instead of %d bytes to %s", n, len(data), w.Path())
	}
}

// MustCreateFlockFile creates FlockFilename file in the directory dir
// and returns the handler to the file.
func MustCreateFlockFile(dir string) *os.File {
	flockFilepath := filepath.Join(dir, FlockFilename)
	f, err := createFlockFile(flockFilepath)
	if err != nil {
		logger.Panicf("FATAL: cannot create lock file: %s; make sure a single process has exclusive access to %q", err, dir)
	}
	return f
}

// FlockFilename is the filename for the file created by MustCreateFlockFile().
const FlockFilename = "flock.lock"

// MustGetFreeSpace returns free space for the given directory path.
func MustGetFreeSpace(path string) uint64 {
	// Try obtaining cached value at first.
	diskSpaceMapLock.Lock()
	defer diskSpaceMapLock.Unlock()

	e, ok := diskSpaceMap[path]
	if ok && fasttime.UnixTimestamp()-e.updateTime < 2 {
		// Fast path - the entry is fresh.
		return e.free
	}

	// Slow path.
	// Determine the amount of free space at path.
	e = updateDiskSpaceLocked(path)
	return e.free
}

// MustGetTotalSpace returns the total disk space for the given directory path.
func MustGetTotalSpace(path string) uint64 {
	// Try obtaining cached value at first.
	diskSpaceMapLock.Lock()
	defer diskSpaceMapLock.Unlock()

	e, ok := diskSpaceMap[path]
	if ok && fasttime.UnixTimestamp()-e.updateTime < 2 {
		// Fast path - the entry is fresh.
		return e.total
	}

	// Slow path.
	// Determine the amount of total space at path.
	e = updateDiskSpaceLocked(path)
	return e.total
}

func updateDiskSpaceLocked(path string) diskSpaceEntry {
	var e diskSpaceEntry
	e.total, e.free = mustGetDiskSpace(path)
	e.updateTime = fasttime.UnixTimestamp()
	diskSpaceMap[path] = e

	return e
}

var (
	diskSpaceMap     = make(map[string]diskSpaceEntry)
	diskSpaceMapLock sync.Mutex
)

type diskSpaceEntry struct {
	updateTime uint64
	free       uint64
	total      uint64
}

// IsDirOrSymlink returns true if de is directory or symlink.
func IsDirOrSymlink(de os.DirEntry) bool {
	return de.IsDir() || (de.Type()&os.ModeSymlink == os.ModeSymlink)
}
