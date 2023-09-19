package fscommon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// AppendFiles appends paths to all the files from local dir to dst.
//
// All the appended filepaths will have dir prefix.
// The returned paths have local OS-specific directory separators.
func AppendFiles(dst []string, dir string) ([]string, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot open directory: %w", err)
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
		if isSpecialFile(name) {
			// Do not take into account special files.
			continue
		}
		path := filepath.Join(dir, name)
		if fi.IsDir() {
			// Process directory
			dst, err = AppendFiles(dst, path)
			if err != nil {
				return nil, fmt.Errorf("cannot append files %q: %w", path, err)
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
			pathReal += string(filepath.Separator)
			for i := len(dst); i < len(dstNew); i++ {
				x := dstNew[i]
				if !strings.HasPrefix(x, pathReal) {
					return nil, fmt.Errorf("unexpected prefix for path %q; want %q", x, pathReal)
				}
				dstNew[i] = filepath.Join(path, x[len(pathReal):])
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

func isSpecialFile(name string) bool {
	return name == "flock.lock" || name == backupnames.RestoreInProgressFilename || name == backupnames.RestoreMarkFileName
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
	for _, fi := range fis {
		name := fi.Name()
		if name == "." || name == ".." {
			continue
		}
		path := filepath.Join(dir, name)
		if fi.IsDir() {
			// Process directory
			ok, err := removeEmptyDirs(path)
			if err != nil {
				return false, fmt.Errorf("cannot remove empty dirs %q: %w", path, err)
			}
			if !ok {
				dirEntries++
			}
			continue
		}
		if fi.Mode()&os.ModeSymlink != os.ModeSymlink {
			// isSpecialFile is not suitable for this function, because the root directory must be considered not empty
			// i.e. function must consider the markers of the restore in progress as files that are not allowed to be removed by this function.
			if name == "flock.lock" {
				continue
			}
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
	if err := d.Close(); err != nil {
		return false, fmt.Errorf("cannot close %q: %w", dir, err)
	}
	// Use os.RemoveAll() instead of os.Remove(), since the dir may contain special files such as flock.lock and backupnames.RestoreInProgressFilename,
	// which must be ignored.
	if err := os.RemoveAll(dir); err != nil {
		return false, fmt.Errorf("cannot remove %q: %w", dir, err)
	}
	return true, nil
}

// IgnorePath returns true if the given path must be ignored.
func IgnorePath(path string) bool {
	return strings.HasSuffix(path, ".ignore")
}
