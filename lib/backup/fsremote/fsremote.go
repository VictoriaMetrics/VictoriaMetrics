package fsremote

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
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
	dir += "/"
	for _, file := range files {
		if !strings.HasPrefix(file, dir) {
			logger.Panicf("BUG: unexpected prefix for file %q; want %q", file, dir)
		}
		var p common.Part
		if !p.ParseFromRemotePath(file[len(dir):]) {
			logger.Infof("skipping unknown file %s", file)
			continue
		}
		// Check for correct part size.
		fi, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("cannot stat file %q for part %q: %s", file, p.Path, err)
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
		return fmt.Errorf("cannot remove %q: %s", path, err)
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
		return fscommon.FsyncFile(dstPath)
	}

	// Cannot create hardlink. Just copy file contents
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cannot open file %q: %s", srcPath, err)
	}
	dstFile, err := os.Create(dstPath)
	if err != nil {
		_ = srcFile.Close()
		return fmt.Errorf("cannot create file %q: %s", dstPath, err)
	}
	n, err := io.Copy(dstFile, srcFile)
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
	if err := fscommon.FsyncFile(dstPath); err != nil {
		_ = os.RemoveAll(dstPath)
		return err
	}
	return nil
}

// DownloadPart download part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	path := fs.path(p)
	r, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open %q: %s", path, err)
	}
	n, err := io.Copy(w, r)
	if err1 := r.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot download data from %q: %s", path, err)
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
		return fmt.Errorf("cannot create file %q: %s", path, err)
	}
	n, err := io.Copy(w, r)
	if err1 := w.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		_ = os.RemoveAll(path)
		return fmt.Errorf("cannot upload data to %q: %s", path, err)
	}
	if uint64(n) != p.Size {
		_ = os.RemoveAll(path)
		return fmt.Errorf("wrong data size uploaded to %q; got %d bytes; want %d bytes", path, n, p.Size)
	}
	if err := fscommon.FsyncFile(path); err != nil {
		_ = os.RemoveAll(path)
		return err
	}
	return nil
}

func (fs *FS) mkdirAll(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create directory %q: %s", dir, err)
	}
	return nil
}

func (fs *FS) path(p common.Part) string {
	return p.RemotePath(fs.Dir)
}
