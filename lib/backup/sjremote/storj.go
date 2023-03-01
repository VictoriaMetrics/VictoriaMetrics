package sjremote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"storj.io/uplink"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FS represents filesystem for backups in S3.
//
// Init must be called before calling other FS methods.
type FS struct {

	// Storj bucket to use.
	Bucket string

	// Directory in the bucket to write to.
	Dir string

	// AccessGrant contains all the connection information serialized
	AccessGrant string

	// Private controllers
	access  *uplink.Access
	project *uplink.Project
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init() error {
	if fs.project != nil {
		logger.Panicf("BUG: Init is already called")
	}
	for strings.HasPrefix(fs.Dir, "/") {
		fs.Dir = fs.Dir[1:]
	}
	if !strings.HasSuffix(fs.Dir, "/") {
		fs.Dir += "/"
	}

	// Parse access grant to obtain satellite address, API key and encryption key
	var err error
	if fs.access, err = uplink.ParseAccess(fs.AccessGrant); err != nil {
		return fmt.Errorf("could not parse access grant: %w", err)

	}
	// Open up the Project we will be working on.
	if fs.project, err = uplink.OpenProject(context.Background(), fs.access); err != nil {
		return fmt.Errorf("could not open project: %w", err)
	}

	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	if fs.project != nil {
		fs.project.Close()
	}
	fs.project = nil
	fs.access = nil
}

// String returns human-readable description for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("Storj{bucket: %q, dir: %q}", fs.Bucket, fs.Dir)
}

// ListParts returns all the parts for fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	iterator := fs.project.ListObjects(context.Background(), fs.Bucket, &uplink.ListObjectsOptions{
		Prefix:    fs.Dir,
		Recursive: true,
	})
	var (
		object *uplink.Object
		p      common.Part
		parts  []common.Part
	)
	for iterator.Next() {
		// Process item
		object = iterator.Item()
		if object.IsPrefix {
			logger.Infof("skipping prefix %q", object.Key)
			continue
		}
		if !strings.HasPrefix(object.Key, fs.Dir) {
			return nil, fmt.Errorf("unexpected prefix for storj key %q; want %q", object.Key, fs.Dir)
		}
		if fscommon.IgnorePath(object.Key) {
			continue
		}
		if !p.ParseFromRemotePath(object.Key[len(fs.Dir):]) {
			logger.Infof("skipping unknown object %q", object.Key)
			continue
		}
		p.ActualSize = uint64(object.System.ContentLength)
		parts = append(parts, p)
	}
	if iterator.Err() != nil {
		return nil, fmt.Errorf("storj list objects iteration error: %w", iterator.Err())
	}
	return parts, nil
}

// DeletePart deletes part p from fs.
func (fs *FS) DeletePart(p common.Part) error {
	path := fs.path(p)
	_, err := fs.project.DeleteObject(context.Background(), fs.Bucket, path)
	if err != nil {
		return fmt.Errorf("cannot delete %q at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	return nil
}

// RemoveEmptyDirs recursively removes empty dirs in fs.
func (fs *FS) RemoveEmptyDirs() error {
	// Storj has no directories, so nothing to remove.
	return nil
}

// CopyPart copies p from srcFS to fs.
func (fs *FS) CopyPart(srcFS common.OriginFS, p common.Part) error {
	src, ok := srcFS.(*FS)
	if !ok {
		return fmt.Errorf("cannot perform server-side copying from %s to %s: both of them must be Storj", srcFS, fs)
	}
	srcPath := src.path(p)
	dstPath := fs.path(p)
	copySource := fmt.Sprintf("/%s/%s", src.Bucket, srcPath)

	_, err := fs.project.CopyObject(context.Background(), src.Bucket, srcPath, fs.Bucket, dstPath, nil)
	if err != nil {
		return fmt.Errorf("cannot copy %q from %s to %s (copySource %q): %w", p.Path, src, fs, copySource, err)
	}
	return nil
}

// DownloadPart downloads part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	path := fs.path(p)
	r, err := fs.project.DownloadObject(context.Background(), fs.Bucket, path, nil)
	if err != nil {
		return fmt.Errorf("cannot open %q at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	n, err := io.Copy(w, r)
	if err1 := r.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot download %q from at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	if uint64(n) != p.Size {
		return fmt.Errorf("wrong data size downloaded from %q at %s; got %d bytes; want %d bytes", p.Path, fs, n, p.Size)
	}
	return nil
}

// UploadPart uploads part p from r to fs.
func (fs *FS) UploadPart(p common.Part, r io.Reader) error {
	path := fs.path(p)
	sr := &statReader{
		r: r,
	}
	upload, err := fs.project.UploadObject(context.Background(), fs.Bucket, path, nil)
	if err != nil {
		return fmt.Errorf("cannot start data upload to %q at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	if _, err = io.Copy(upload, sr); err != nil {
		_ = upload.Abort()
		return fmt.Errorf("could not upload data: %v", err)
	}
	if err = upload.Commit(); err != nil {
		return fmt.Errorf("could not commit uploaded object: %v", err)
	}
	if uint64(sr.size) != p.Size {
		return fmt.Errorf("wrong data size uploaded to %q at %s; got %d bytes; want %d bytes", p.Path, fs, sr.size, p.Size)
	}
	return nil
}

// DeleteFile deletes filePath from fs if it exists.
//
// The function does nothing if the file doesn't exist.
func (fs *FS) DeleteFile(filePath string) error {
	ok, err := fs.HasFile(filePath)
	if err != nil {
		return err
	}
	if !ok {
		// Missing file - nothing to delete.
		return nil
	}

	path := fs.Dir + filePath
	_, err = fs.project.DeleteObject(context.Background(), fs.Bucket, path)
	if err != nil {
		return fmt.Errorf("cannot delete %q at %s (remote path %q): %w", filePath, fs, path, err)
	}
	return nil
}

// CreateFile creates filePath at fs and puts data into it.
//
// The file is overwritten if it already exists.
func (fs *FS) CreateFile(filePath string, data []byte) error {
	path := fs.Dir + filePath
	sr := &statReader{
		r: bytes.NewReader(data),
	}
	upload, err := fs.project.UploadObject(context.Background(), fs.Bucket, path, nil)
	if err != nil {
		return fmt.Errorf("cannot start data upload to %q at %s (remote path %q): %w", filePath, fs, path, err)
	}
	if _, err = io.Copy(upload, sr); err != nil {
		_ = upload.Abort()
		return fmt.Errorf("could not upload data: %v", err)
	}
	if err = upload.Commit(); err != nil {
		return fmt.Errorf("could not commit uploaded object: %v", err)
	}
	l := int64(len(data))
	if sr.size != l {
		return fmt.Errorf("wrong data size uploaded to %q at %s; got %d bytes; want %d bytes", filePath, fs, sr.size, l)
	}
	return nil
}

// HasFile returns true if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := fs.Dir + filePath
	_, err := fs.project.StatObject(context.Background(), fs.Bucket, path)
	if err != nil {
		// ugly hack is ugly
		if strings.Contains(err.Error(), uplink.ErrObjectNotFound.Error()) {
			err = nil
		} else {
			err = fmt.Errorf("failed to stat object: %w", err)
		}
		return false, err
	}
	return true, nil
}

func (fs *FS) path(p common.Part) string {
	return p.RemotePath(fs.Dir)
}

type statReader struct {
	r    io.Reader
	size int64
}

func (sr *statReader) Read(p []byte) (int, error) {
	n, err := sr.r.Read(p)
	sr.size += int64(n)
	return n, err
}
