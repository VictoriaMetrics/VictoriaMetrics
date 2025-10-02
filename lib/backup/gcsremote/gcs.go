package gcsremote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FS represents filesystem for backups in GCS.
//
// Init must be called before calling other FS methods.
type FS struct {
	// Path to GCP credentials file.
	//
	// Default credentials are used if empty.
	CredsFilePath string

	// GCS bucket to use.
	Bucket string

	// Directory in the bucket to write to.
	Dir string

	// Metadata to be set for uploaded objects.
	Metadata map[string]string

	bkt *storage.BucketHandle

	ctx    context.Context
	cancel context.CancelFunc
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init(ctx context.Context) error {
	if fs.bkt != nil {
		logger.Panicf("BUG: fs.Init has been already called")
	}

	fs.ctx, fs.cancel = context.WithCancel(ctx)

	for strings.HasPrefix(fs.Dir, "/") {
		fs.Dir = fs.Dir[1:]
	}
	if !strings.HasSuffix(fs.Dir, "/") {
		fs.Dir += "/"
	}

	var client *storage.Client
	if len(fs.CredsFilePath) > 0 {
		creds := option.WithCredentialsFile(fs.CredsFilePath)
		c, err := storage.NewClient(fs.ctx, creds)
		if err != nil {
			return fmt.Errorf("cannot create gcs client with credsFile %q: %w", fs.CredsFilePath, err)
		}
		client = c
	} else {
		c, err := storage.NewClient(fs.ctx)
		if err != nil {
			return fmt.Errorf("cannot create default gcs client: %w", err)
		}
		client = c
	}

	client.SetRetry(
		storage.WithPolicy(storage.RetryAlways),
		storage.WithBackoff(gax.Backoff{
			Initial:    time.Second,
			Max:        time.Minute * 3,
			Multiplier: 3,
		}))
	fs.bkt = client.Bucket(fs.Bucket)
	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	if fs.cancel != nil {
		fs.cancel()
	}
	fs.bkt = nil
}

// String returns human-readable description for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("GCS{bucket: %q, dir: %q}", fs.Bucket, fs.Dir)
}

// selectAttrs contains object attributes to select in ListParts.
var selectAttrs = []string{
	"Name",
	"Size",
}

// ListParts returns all the parts for fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	dir := fs.Dir
	q := &storage.Query{
		Prefix: dir,
	}
	if err := q.SetAttrSelection(selectAttrs); err != nil {
		return nil, fmt.Errorf("error in SetAttrSelection: %w", err)
	}
	it := fs.bkt.Objects(fs.ctx, q)
	var parts []common.Part
	for {
		attr, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return parts, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error when iterating objects at %q: %w", dir, err)
		}
		file := attr.Name
		if !strings.HasPrefix(file, dir) {
			return nil, fmt.Errorf("unexpected prefix for gcs key %q; want %q", file, dir)
		}
		if fscommon.IgnorePath(file) {
			continue
		}
		var p common.Part
		if !p.ParseFromRemotePath(file[len(dir):]) {
			logger.Infof("skipping unknown object %q", file)
			continue
		}
		p.ActualSize = uint64(attr.Size)
		parts = append(parts, p)
	}
}

// DeletePart deletes part p from fs.
func (fs *FS) DeletePart(p common.Part) error {
	path := p.RemotePath(fs.Dir)
	return fs.delete(path)
}

// RemoveEmptyDirs recursively removes empty dirs in fs.
func (fs *FS) RemoveEmptyDirs() error {
	// GCS has no directories, so nothing to remove.
	return nil
}

// CopyPart copies p from srcFS to fs.
func (fs *FS) CopyPart(srcFS common.OriginFS, p common.Part) error {
	src, ok := srcFS.(*FS)
	if !ok {
		return fmt.Errorf("cannot perform server-side copying from %s to %s: both of them must be GCS", srcFS, fs)
	}
	srcObj := src.object(p)
	dstObj := fs.object(p)

	copier := dstObj.CopierFrom(srcObj)
	if len(fs.Metadata) > 0 {
		copier.Metadata = fs.Metadata
	}
	attr, err := copier.Run(fs.ctx)
	if err != nil {
		return fmt.Errorf("cannot copy %q from %s to %s: %w", p.Path, src, fs, err)
	}
	if uint64(attr.Size) != p.Size {
		return fmt.Errorf("unexpected %q size after copying from %s to %s; got %d bytes; want %d bytes", p.Path, src, fs, attr.Size, p.Size)
	}
	return nil
}

// DownloadPart downloads part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	o := fs.object(p)
	r, err := o.NewReader(fs.ctx)
	if err != nil {
		return fmt.Errorf("cannot open reader for %q at %s (remote path %q): %w", p.Path, fs, o.ObjectName(), err)
	}
	n, err := io.Copy(w, r)
	if err1 := r.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot download %q from at %s (remote path %q): %w", p.Path, fs, o.ObjectName(), err)
	}
	if uint64(n) != p.Size {
		return fmt.Errorf("wrong data size downloaded from %q at %s; got %d bytes; want %d bytes", p.Path, fs, n, p.Size)
	}
	return nil
}

// UploadPart uploads part p from r to fs.
func (fs *FS) UploadPart(p common.Part, r io.Reader) error {
	o := fs.object(p)
	w := o.NewWriter(fs.ctx)
	if len(fs.Metadata) > 0 {
		w.Metadata = fs.Metadata
	}
	n, err := io.Copy(w, r)
	if err1 := w.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot upload data to %q at %s (remote path %q): %w", p.Path, fs, o.ObjectName(), err)
	}
	if uint64(n) != p.Size {
		return fmt.Errorf("wrong data size uploaded to %q at %s; got %d bytes; want %d bytes", p.Path, fs, n, p.Size)
	}
	return nil
}

func (fs *FS) object(p common.Part) *storage.ObjectHandle {
	path := p.RemotePath(fs.Dir)
	return fs.bkt.Object(path)
}

// DeleteFile deletes filePath at fs if it exists.
//
// The function does nothing if the filePath doesn't exists.
func (fs *FS) DeleteFile(filePath string) error {
	path := path.Join(fs.Dir, filePath)
	return fs.delete(path)
}

func (fs *FS) delete(path string) error {
	if *common.DeleteAllObjectVersions {
		return fs.deleteObjectWithGenerations(path)
	}
	return fs.deleteObject(path)
}

// deleteObjectWithGenerations deletes object at path and all its generations.
func (fs *FS) deleteObjectWithGenerations(path string) error {
	it := fs.bkt.Objects(fs.ctx, &storage.Query{
		Versions: true,
		Prefix:   path,
	})
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("cannot read %q at %s: %w", path, fs, err)
		}

		if err := fs.bkt.Object(path).Generation(attrs.Generation).Delete(fs.ctx); err != nil {
			if !errors.Is(err, storage.ErrObjectNotExist) {
				return fmt.Errorf("cannot delete %q at %s: %w", path, fs, err)
			}
		}
	}
}

// deleteObject deletes object at path.
// It does not specify a Generation, so it will delete the latest generation of the object.
func (fs *FS) deleteObject(path string) error {
	o := fs.bkt.Object(path)
	if err := o.Delete(fs.ctx); err != nil {
		if !errors.Is(err, storage.ErrObjectNotExist) {
			return fmt.Errorf("cannot delete %q at %s: %w", o.ObjectName(), fs, err)
		}
	}

	return nil
}

// CreateFile creates filePath at fs and puts data into it.
//
// The file is overwritten if it exists.
func (fs *FS) CreateFile(filePath string, data []byte) error {
	path := path.Join(fs.Dir, filePath)
	o := fs.bkt.Object(path)
	w := o.NewWriter(fs.ctx)
	if len(fs.Metadata) > 0 {
		w.Metadata = fs.Metadata
	}
	n, err := w.Write(data)
	if err != nil {
		_ = w.Close()
		return fmt.Errorf("cannot upload %d bytes to %q at %s (remote path %q): %w", len(data), filePath, fs, o.ObjectName(), err)
	}
	if n != len(data) {
		_ = w.Close()
		return fmt.Errorf("wrong data size uploaded to %q at %s (remote path %q); got %d bytes; want %d bytes", filePath, fs, o.ObjectName(), n, len(data))
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("cannot close %q at %s (remote path %q): %w", filePath, fs, o.ObjectName(), err)
	}
	return nil
}

// HasFile returns true if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := path.Join(fs.Dir, filePath)
	o := fs.bkt.Object(path)
	_, err := o.Attrs(fs.ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("unexpected error when obtaining attributes for %q at %s (remote path %q): %w", filePath, fs, o.ObjectName(), err)
	}
	return true, nil
}

// ReadFile returns the content of filePath at fs.
func (fs *FS) ReadFile(filePath string) ([]byte, error) {
	path := path.Join(fs.Dir, filePath)
	o := fs.bkt.Object(path)
	r, err := o.NewReader(fs.ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q at %s (remote path %q): %w", filePath, fs, o.ObjectName(), err)
	}
	defer r.Close()
	return io.ReadAll(r)
}
