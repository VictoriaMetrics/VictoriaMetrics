package fsremote

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	libfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FS represents remote filesystem.
//
// Backups are uploaded there.
// Data is downloaded from there during restore.
type FS struct {
	// Dir is a path to remote directory with backup data.
	Dir string
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	// Nothing to do
}

// String returns human-readable string representation for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("fsremote %q", fs.Dir)
}

// ListParts returns all the parts from fs.
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
		if fscommon.IgnorePath(file) {
			continue
		}
		var p common.Part
		remotePath := common.ToCanonicalPath(file[len(dir):])
		if !p.ParseFromRemotePath(remotePath) {
			logger.Infof("skipping unknown file %s", file)
			continue
		}
		// Check for correct part size.
		fi, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("cannot stat file %q for part %q: %w", file, p.Path, err)
		}
		p.ActualSize = uint64(fi.Size())
		parts = append(parts, p)
	}
	return parts, nil
}

// DeletePart deletes the given part p from fs.
func (fs *FS) DeletePart(p common.Part) error {
	path := fs.path(p)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove %q: %w", path, err)
	}
	return nil
}

// RemoveEmptyDirs recursively removes all the empty directories in fs.
func (fs *FS) RemoveEmptyDirs() error {
	return fscommon.RemoveEmptyDirs(fs.Dir)
}

// CopyPart copies the part p from srcFS to fs.
//
// srcFS must have *FS type.
func (fs *FS) CopyPart(srcFS common.OriginFS, p common.Part) error {
	src, ok := srcFS.(*FS)
	if !ok {
		return fmt.Errorf("cannot perform server-side copying from %s to %s: both of them must be fsremote", srcFS, fs)
	}
	srcPath := src.path(p)
	dstPath := fs.path(p)
	if err := fs.mkdirAll(dstPath); err != nil {
		return err
	}
	// Attempt to create hardlink from srcPath to dstPath.
	if err := os.Link(srcPath, dstPath); err == nil {
		libfs.MustSyncPath(dstPath)
		return nil
	}

	// Cannot create hardlink. Just copy file contents
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cannot open source file: %w", err)
	}
	dstFile, err := os.Create(dstPath)
	if err != nil {
		_ = srcFile.Close()
		return fmt.Errorf("cannot create destination file: %w", err)
	}
	n, err := io.Copy(dstFile, srcFile)
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("cannot fsync dstFile: %q: %w", dstFile.Name(), err)
	}
	if err1 := dstFile.Close(); err1 != nil {
		err = err1
	}
	if err1 := srcFile.Close(); err1 != nil {
		err = err1
	}
	if err != nil {
		_ = os.RemoveAll(dstPath)
		return err
	}
	if uint64(n) != p.Size {
		_ = os.RemoveAll(dstPath)
		return fmt.Errorf("unexpected number of bytes copied from %q to %q; got %d bytes; want %d bytes", srcPath, dstPath, n, p.Size)
	}
	return nil
}

// DownloadPart download part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	path := fs.path(p)
	r, err := os.Open(path)
	if err != nil {
		return err
	}
	n, err := io.Copy(w, r)
	if err1 := r.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot download data from %q: %w", path, err)
	}
	if uint64(n) != p.Size {
		return fmt.Errorf("wrong data size downloaded from %q; got %d bytes; want %d bytes", path, n, p.Size)
	}
	return nil
}

// UploadPart uploads p from r to fs.
func (fs *FS) UploadPart(p common.Part, r io.Reader) error {
	path := fs.path(p)
	if err := fs.mkdirAll(path); err != nil {
		return err
	}
	w, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create file %q: %w", path, err)
	}
	n, err := io.Copy(w, r)
	if err := w.Sync(); err != nil {
		return fmt.Errorf("cannot fsync file: %q: %w", w.Name(), err)
	}
	if err1 := w.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		_ = os.RemoveAll(path)
		return fmt.Errorf("cannot upload data to %q: %w", path, err)
	}
	if uint64(n) != p.Size {
		_ = os.RemoveAll(path)
		return fmt.Errorf("wrong data size uploaded to %q; got %d bytes; want %d bytes", path, n, p.Size)
	}
	return nil
}

func (fs *FS) mkdirAll(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create directory %q: %w", dir, err)
	}
	return nil
}

func (fs *FS) path(p common.Part) string {
	return filepath.Join(p.LocalPath(fs.Dir), fmt.Sprintf("%016X_%016X_%016X", p.FileSize, p.Offset, p.Size))
}

// DeleteFile deletes filePath at fs.
//
// The function does nothing if the filePath doesn't exist.
func (fs *FS) DeleteFile(filePath string) error {
	path := filepath.Join(fs.Dir, filePath)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove %q: %w", path, err)
	}
	return nil
}

// CreateFile creates filePath at fs and puts data into it.
//
// The file is overwritten if it exists.
func (fs *FS) CreateFile(filePath string, data []byte) error {
	path := filepath.Join(fs.Dir, filePath)
	if err := fs.mkdirAll(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write %d bytes to %q: %w", len(data), path, err)
	}
	return nil
}

// HasFile returns true if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := filepath.Join(fs.Dir, filePath)
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if fi.IsDir() {
		return false, fmt.Errorf("%q is directory, while file is needed", path)
	}
	return true, nil
}

// ReadFile returns the content of filePath at fs.
func (fs *FS) ReadFile(filePath string) ([]byte, error) {
	path := filepath.Join(fs.Dir, filePath)
	return os.ReadFile(path)
}
