package fscommon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FsyncFile fsyncs path contents and the parent directory contents.
func FsyncFile(path string) error {
	if err := fsync(path); err != nil {
		_ = os.RemoveAll(path)
		return fmt.Errorf("cannot fsync file %q: %w", path, err)
	}
	dir := filepath.Dir(path)
	if err := fsync(dir); err != nil {
		return fmt.Errorf("cannot fsync dir %q: %w", dir, err)
	}
	return nil
}

// FsyncDir fsyncs dir contents.
func FsyncDir(dir string) error {
	return fsync(dir)
}

func fsync(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// AppendFiles appends all the files from dir to dst.
//
// All the appended files will have dir prefix.
func AppendFiles(dst []string, dir string) ([]string, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", dir, err)
	}
	dst, err = appendFilesInternal(dst, d)
	if err1 := d.Close(); err1 != nil {
		err = err1
	}
	return dst, err
}

func appendFilesInternal(dst []string, d *os.File) ([]string, error) {
	dir := d.Name()
	dfi, err := d.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat %q: %w", dir, err)
	}
	if !dfi.IsDir() {
		return nil, fmt.Errorf("%q isn't a directory", dir)
	}
	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory contents in %q: %w", dir, err)
	}
	for _, fi := range fis {
		name := fi.Name()
		if name == "." || name == ".." {
			continue
		}
		if name == "flock.lock" {
			// Do not take into account flock.lock files, since they are used
			// for preventing from concurrent access.
			continue
		}
		path := dir + "/" + name
		if fi.IsDir() {
			// Process directory
			dst, err = AppendFiles(dst, path)
			if err != nil {
				return nil, fmt.Errorf("cannot list %q: %w", path, err)
			}
			continue
		}
		if fi.Mode()&os.ModeSymlink != os.ModeSymlink {
			// Process file
			dst = append(dst, path)
			continue
		}
		pathOrig := path
	again:
		// Process symlink
		pathReal, err := filepath.EvalSymlinks(pathOrig)
		if err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory") {
				// Skip symlink that points to nowhere.
				continue
			}
			return nil, fmt.Errorf("cannot resolve symlink %q: %w", pathOrig, err)
		}
		sfi, err := os.Stat(pathReal)
		if err != nil {
			return nil, fmt.Errorf("cannot stat %q from symlink %q: %w", pathReal, path, err)
		}
		if sfi.IsDir() {
			// Symlink points to directory
			dstNew, err := AppendFiles(dst, pathReal)
			if err != nil {
				return nil, fmt.Errorf("cannot list files at %q from symlink %q: %w", pathReal, path, err)
			}
			pathReal += "/"
			for i := len(dst); i < len(dstNew); i++ {
				x := dstNew[i]
				if !strings.HasPrefix(x, pathReal) {
					return nil, fmt.Errorf("unexpected prefix for path %q; want %q", x, pathReal)
				}
				dstNew[i] = path + "/" + x[len(pathReal):]
			}
			dst = dstNew
			continue
		}
		if sfi.Mode()&os.ModeSymlink != os.ModeSymlink {
			// Symlink points to file
			dst = append(dst, path)
			continue
		}
		// Symlink points to symlink. Process it again.
		pathOrig = pathReal
		goto again
	}
	return dst, nil
}

// RemoveEmptyDirs recursively removes empty directories under the given dir.
func RemoveEmptyDirs(dir string) error {
	_, err := removeEmptyDirs(dir)
	return err
}

func removeEmptyDirs(dir string) (bool, error) {
	d, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	ok, err := removeEmptyDirsInternal(d)
	if err1 := d.Close(); err1 != nil {
		err = err1
	}
	if err != nil {
		return false, err
	}
	return ok, nil
}

func removeEmptyDirsInternal(d *os.File) (bool, error) {
	dir := d.Name()
	dfi, err := d.Stat()
	if err != nil {
		return false, fmt.Errorf("cannot stat %q: %w", dir, err)
	}
	if !dfi.IsDir() {
		return false, fmt.Errorf("%q isn't a directory", dir)
	}
	fis, err := d.Readdir(-1)
	if err != nil {
		return false, fmt.Errorf("cannot read directory contents in %q: %w", dir, err)
	}
	dirEntries := 0
	hasFlock := false
	for _, fi := range fis {
		name := fi.Name()
		if name == "." || name == ".." {
			continue
		}
		path := dir + "/" + name
		if fi.IsDir() {
			// Process directory
			ok, err := removeEmptyDirs(path)
			if err != nil {
				return false, fmt.Errorf("cannot list %q: %w", path, err)
			}
			if !ok {
				dirEntries++
			}
			continue
		}
		if fi.Mode()&os.ModeSymlink != os.ModeSymlink {
			if name == "flock.lock" {
				hasFlock = true
				continue
			}
			// Skip plain files.
			dirEntries++
			continue
		}
		pathOrig := path
	again:
		// Process symlink
		pathReal, err := filepath.EvalSymlinks(pathOrig)
		if err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory") {
				// Remove symlink that points to nowere.
				logger.Infof("removing broken symlink %q", pathOrig)
				if err := os.Remove(pathOrig); err != nil {
					return false, fmt.Errorf("cannot remove %q: %w", pathOrig, err)
				}
				continue
			}
			return false, fmt.Errorf("cannot resolve symlink %q: %w", pathOrig, err)
		}
		sfi, err := os.Stat(pathReal)
		if err != nil {
			return false, fmt.Errorf("cannot stat %q from symlink %q: %w", pathReal, path, err)
		}
		if sfi.IsDir() {
			// Symlink points to directory
			ok, err := removeEmptyDirs(pathReal)
			if err != nil {
				return false, fmt.Errorf("cannot list files at %q from symlink %q: %w", pathReal, path, err)
			}
			if !ok {
				dirEntries++
			} else {
				// Remove the symlink
				logger.Infof("removing symlink that points to empty dir %q", pathOrig)
				if err := os.Remove(pathOrig); err != nil {
					return false, fmt.Errorf("cannot remove %q: %w", pathOrig, err)
				}
			}
			continue
		}
		if sfi.Mode()&os.ModeSymlink != os.ModeSymlink {
			// Symlink points to file. Skip it.
			dirEntries++
			continue
		}
		// Symlink points to symlink. Process it again.
		pathOrig = pathReal
		goto again
	}
	if dirEntries > 0 {
		return false, nil
	}
	logger.Infof("removing empty dir %q", dir)
	if hasFlock {
		flockFilepath := dir + "/flock.lock"
		if err := os.Remove(flockFilepath); err != nil {
			return false, fmt.Errorf("cannot remove %q: %w", flockFilepath, err)
		}
	}
	if err := os.Remove(dir); err != nil {
		return false, fmt.Errorf("cannot remove %q: %w", dir, err)
	}
	return true, nil
}

// IgnorePath returns true if the given path must be ignored.
func IgnorePath(path string) bool {
	return strings.HasSuffix(path, ".ignore")
}

// BackupCompleteFilename is a filename, which is created in the destination fs when backup is complete.
const BackupCompleteFilename = "backup_complete.ignore"
