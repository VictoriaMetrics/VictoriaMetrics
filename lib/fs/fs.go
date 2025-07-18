package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
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
	mustSyncPath(path)
}

// MustWriteStreamSync writes src contents to the file at path and then calls fsync on the created file.
// The fsync guarantees that the written data survives hardware reset after successful call.
//
// This function may leave the file at the path in inconsistent state on app crash
// in the middle of the write.
// Use MustWriteAtomic if the file at the path must be either written in full
// or not written at all on app crash in the middle of the write.
func MustWriteStreamSync(path string, src io.WriterTo) {
	f := filestream.MustCreate(path, false)
	if _, err := src.WriteTo(f); err != nil {
		f.MustClose()
		// Do not call MustRemoveAll(path), so the user could inspect
		// the file contents during investigation of the issue.
		logger.Panicf("FATAL: cannot write data to %q: %s", path, err)
	}
	f.MustClose()
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
		// Do not call MustRemoveAll(path), so the user could inspect
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
		// do not call MustRemoveAll(tmpPath) here, so the user could inspect
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
func MustMkdirIfNotExist(path string) {
	if IsPathExist(path) {
		return
	}
	mustMkdirSync(path)
}

// MustMkdirFailIfExist creates the given path dir if it isn't exist.
//
// If the directory at the given path already exists, then the function logs the error and exits.
func MustMkdirFailIfExist(path string) {
	if IsPathExist(path) {
		logger.Panicf("FATAL: the %q already exists", path)
	}
	mustMkdirSync(path)
}

func mustMkdirSync(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logger.Panicf("FATAL: cannot create directory: %s", err)
	}
	// Sync the parent directory, so the created directory becomes visible
	// in the fs after power loss.
	parentDirPath := filepath.Dir(path)
	MustSyncPath(parentDirPath)
}

// RemoveDirContents removes all the contents of the given dir if it exists.
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
		logger.Panicf("FATAL: cannot open dir: %s", err)
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
		fullPath := filepath.Join(dir, name)
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

// MustRemoveDirAtomic removes the given dir atomically.
//
// It uses the following algorithm:
//
//  1. Atomically rename the "<dir>" to "<dir>.must-remove.<XYZ>",
//     where <XYZ> is an unique number.
//  2. Remove the "<dir>.must-remove.XYZ" in background.
//
// If the process crashes after the step 1, then the directory must be removed
// on the next process start by calling MustRemoveTemporaryDirs on the parent directory.
func MustRemoveDirAtomic(dir string) {
	if !IsPathExist(dir) {
		return
	}
	mustRemoveDirAtomic(dir)
	parentDir := filepath.Dir(dir)
	MustSyncPath(parentDir)
}

var atomicDirRemoveCounter = func() *atomicutil.Uint64 {
	var x atomicutil.Uint64
	x.Store(uint64(time.Now().UnixNano()))
	return &x
}()

// MustReadDir reads directory entries at the given dir.
func MustReadDir(dir string) []os.DirEntry {
	des, err := os.ReadDir(dir)
	if err != nil {
		logger.Panicf("FATAL: cannot read directory contents: %s", err)
	}
	return des
}

// MustRemoveTemporaryDirs removes all the subdirectories with ".must-remove.<XYZ>" suffix.
//
// Such directories may be left on unclean shutdown during MustRemoveDirAtomic call.
func MustRemoveTemporaryDirs(dir string) {
	des := MustReadDir(dir)
	for _, de := range des {
		if !IsDirOrSymlink(de) {
			// Skip non-directories
			continue
		}
		dirName := de.Name()
		if IsScheduledForRemoval(dirName) {
			fullPath := filepath.Join(dir, dirName)
			MustRemoveAll(fullPath)
		}
	}
	MustSyncPath(dir)
}

// MustHardLinkFiles makes hard links for all the files from srcDir in dstDir.
func MustHardLinkFiles(srcDir, dstDir string) {
	mustMkdirSync(dstDir)

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

// MustCopyDirectory copies all the files in srcPath to dstPath.
func MustCopyDirectory(srcPath, dstPath string) {
	des := MustReadDir(srcPath)
	MustMkdirIfNotExist(dstPath)
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
	freeSpaceMapLock.Lock()
	defer freeSpaceMapLock.Unlock()

	e, ok := freeSpaceMap[path]
	if ok && fasttime.UnixTimestamp()-e.updateTime < 2 {
		// Fast path - the entry is fresh.
		return e.freeSpace
	}

	// Slow path.
	// Determine the amount of free space at path.
	e.freeSpace = mustGetFreeSpace(path)
	e.updateTime = fasttime.UnixTimestamp()
	freeSpaceMap[path] = e
	return e.freeSpace
}

var (
	freeSpaceMap     = make(map[string]freeSpaceEntry)
	freeSpaceMapLock sync.Mutex
)

type freeSpaceEntry struct {
	updateTime uint64
	freeSpace  uint64
}

// IsScheduledForRemoval returns true if the filename contains .must-remove. substring
func IsScheduledForRemoval(filename string) bool {
	return strings.Contains(filename, ".must-remove.")
}

// IsDirOrSymlink returns true if de is directory or symlink.
func IsDirOrSymlink(de os.DirEntry) bool {
	return de.IsDir() || (de.Type()&os.ModeSymlink == os.ModeSymlink)
}
