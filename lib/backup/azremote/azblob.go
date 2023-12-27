package azremote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	envStorageAcctName           = "AZURE_STORAGE_ACCOUNT_NAME"
	envStorageAccKey             = "AZURE_STORAGE_ACCOUNT_KEY"
	envStorageAccCs              = "AZURE_STORAGE_ACCOUNT_CONNECTION_STRING"
	storageErrorCodeBlobNotFound = "BlobNotFound"
)

// FS represents filesystem for backups in Azure Blob Storage.
//
// Init must be called before calling other FS methods.
type FS struct {
	// Azure Blob Storage bucket to use.
	Container string

	// Directory in the bucket to write to.
	Dir string

	client *container.Client
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init() error {
	if fs.client != nil {
		logger.Panicf("BUG: fs.Init has been already called")
	}

	for strings.HasPrefix(fs.Dir, "/") {
		fs.Dir = fs.Dir[1:]
	}
	if !strings.HasSuffix(fs.Dir, "/") {
		fs.Dir += "/"
	}

	var sc *service.Client
	var err error
	if cs, ok := envtemplate.LookupEnv(envStorageAccCs); ok {
		sc, err = service.NewClientFromConnectionString(cs, nil)
		if err != nil {
			return fmt.Errorf("failed to create AZBlob service client from connection string: %w", err)
		}
	}

	accountName, ok1 := envtemplate.LookupEnv(envStorageAcctName)
	accountKey, ok2 := envtemplate.LookupEnv(envStorageAccKey)
	if ok1 && ok2 {
		creds, err := azblob.NewSharedKeyCredential(accountName, accountKey)
		if err != nil {
			return fmt.Errorf("failed to create AZBlob credentials from account name and key: %w", err)
		}
		serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)

		sc, err = service.NewClientWithSharedKeyCredential(serviceURL, creds, nil)
		if err != nil {
			return fmt.Errorf("failed to create AZBlob service client from account name and key: %w", err)
		}
	}

	if sc == nil {
		return fmt.Errorf(`failed to detect any credentials type for AZBlob. Ensure there is connection string set at %q, or shared key at %q and %q`, envStorageAccCs, envStorageAcctName, envStorageAccKey)
	}

	containerClient := sc.NewContainerClient(fs.Container)
	fs.client = containerClient

	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	fs.client = nil
}

// String returns human-readable description for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("AZBlob{container: %q, dir: %q}", fs.Container, fs.Dir)
}

// ListParts returns all the parts for fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	dir := fs.Dir
	ctx := context.Background()

	opts := &azblob.ListBlobsFlatOptions{
		Prefix: &dir,
	}

	pager := fs.client.NewListBlobsFlatPager(opts)
	var parts []common.Part
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot list blobs at %s (remote path %q): %w", fs, fs.Container, err)
		}

		for _, v := range resp.Segment.BlobItems {
			file := *v.Name
			if !strings.HasPrefix(file, dir) {
				return nil, fmt.Errorf("unexpected prefix for AZBlob key %q; want %q", file, dir)
			}
			if fscommon.IgnorePath(file) {
				continue
			}
			var p common.Part
			if !p.ParseFromRemotePath(file[len(dir):]) {
				logger.Errorf("skipping unknown object %q", file)
				continue
			}

			p.ActualSize = uint64(*v.Properties.ContentLength)
			parts = append(parts, p)
		}

	}

	return parts, nil
}

// DeletePart deletes part p from fs.
func (fs *FS) DeletePart(p common.Part) error {
	return fs.delete(p.RemotePath(fs.Dir))
}

// RemoveEmptyDirs recursively removes empty dirs in fs.
func (fs *FS) RemoveEmptyDirs() error {
	// Blob storage has no directories, so nothing to remove.
	return nil
}

// CopyPart copies p from srcFS to fs.
func (fs *FS) CopyPart(srcFS common.OriginFS, p common.Part) error {
	src, ok := srcFS.(*FS)
	if !ok {
		return fmt.Errorf("cannot perform server-side copying from %s to %s: both of them must be AZBlob", srcFS, fs)
	}

	sbc := src.client.NewBlobClient(p.RemotePath(src.Dir))
	dbc := fs.clientForPart(p)

	ssCopyPermission := sas.BlobPermissions{
		Read:   true,
		Create: true,
		Write:  true,
	}

	startTime := time.Now().Add(-10 * time.Minute)
	o := &blob.GetSASURLOptions{
		StartTime: &startTime,
	}
	t, err := sbc.GetSASURL(ssCopyPermission, time.Now().Add(30*time.Minute), o)
	if err != nil {
		return fmt.Errorf("failed to generate SAS token of src %q: %w", p.Path, err)
	}

	ctx := context.Background()

	// In order to support copy of files larger than 256MB, we need to use the async copy
	// Ref: https://learn.microsoft.com/en-us/rest/api/storageservices/copy-blob-from-url
	_, err = dbc.StartCopyFromURL(ctx, t, &blob.StartCopyFromURLOptions{})
	if err != nil {
		return fmt.Errorf("cannot start async copy %q from %s to %s: %w", p.Path, src, fs, err)
	}

	var copyStatus *blob.CopyStatusType
	var copyStatusDescription *string
	for {
		r, err := dbc.GetProperties(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to check copy status, cannot get properties of %q at %s: %w", p.Path, fs, err)
		}

		// After the copy will be finished status will be changed to success/failed/aborted
		// Ref: https://learn.microsoft.com/en-us/rest/api/storageservices/get-blob-properties#response-headers - x-ms-copy-status
		if *r.CopyStatus != blob.CopyStatusTypePending {
			copyStatus = r.CopyStatus
			copyStatusDescription = r.CopyStatusDescription
			break
		}
		time.Sleep(5 * time.Second)
	}

	if *copyStatus != blob.CopyStatusTypeSuccess {
		return fmt.Errorf("copy of %q from %s to %s failed: expected status %q, received %q (description: %q)", p.Path, src, fs, blob.CopyStatusTypeSuccess, *copyStatus, *copyStatusDescription)
	}

	return nil
}

// DownloadPart downloads part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	bc := fs.clientForPart(p)

	ctx := context.Background()
	r, err := bc.DownloadStream(ctx, &blob.DownloadStreamOptions{})
	if err != nil {
		return fmt.Errorf("cannot open reader for %q at %s (remote path %q): %w", p.Path, fs, bc.URL(), err)
	}

	body := r.NewRetryReader(ctx, &azblob.RetryReaderOptions{})
	n, err := io.Copy(w, body)
	if err1 := body.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot download %q from at %s (remote path %q): %w", p.Path, fs, bc.URL(), err)
	}
	if uint64(n) != p.Size {
		return fmt.Errorf("wrong data size downloaded from %q at %s; got %d bytes; want %d bytes", p.Path, fs, n, p.Size)
	}
	return nil
}

// UploadPart uploads part p from r to fs.
func (fs *FS) UploadPart(p common.Part, r io.Reader) error {
	bc := fs.clientForPart(p)

	ctx := context.Background()
	_, err := bc.UploadStream(ctx, r, &blockblob.UploadStreamOptions{})

	if err != nil {
		return fmt.Errorf("cannot upload data to %q at %s (remote path %q): %w", p.Path, fs, bc.URL(), err)
	}

	return nil
}

func (fs *FS) clientForPart(p common.Part) *blockblob.Client {
	path := p.RemotePath(fs.Dir)

	return fs.clientForPath(path)
}

func (fs *FS) clientForPath(path string) *blockblob.Client {
	bc := fs.client.NewBlockBlobClient(path)
	return bc
}

// DeleteFile deletes filePath at fs if it exists.
//
// The function does nothing if the filePath doesn't exists.
func (fs *FS) DeleteFile(filePath string) error {
	v, err := fs.HasFile(filePath)
	if err != nil {
		return err
	}
	if !v {
		return nil
	}

	path := path.Join(fs.Dir, filePath)
	return fs.delete(path)
}

func (fs *FS) delete(path string) error {
	if *common.DeleteAllObjectVersions {
		return fs.deleteObjectWithGenerations(path)
	}
	return fs.deleteObject(path)
}

func (fs *FS) deleteObjectWithGenerations(path string) error {
	pager := fs.client.NewListBlobsFlatPager(&azblob.ListBlobsFlatOptions{
		Prefix: &path,
		Include: azblob.ListBlobsInclude{
			Versions: true,
		},
	})

	ctx := context.Background()
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("cannot list blobs at %s (remote path %q): %w", path, fs.Container, err)
		}

		for _, v := range resp.Segment.BlobItems {
			var c *blob.Client
			// Either versioning is disabled or we are deleting the current version
			if v.VersionID == nil || (v.VersionID != nil && v.IsCurrentVersion != nil && *v.IsCurrentVersion) {
				c = fs.client.NewBlobClient(*v.Name)
			} else {
				c, err = fs.client.NewBlobClient(*v.Name).WithVersionID(*v.VersionID)
				if err != nil {
					return fmt.Errorf("cannot read blob at %q at %s: %w", path, fs.Container, err)
				}
			}

			if _, err := c.Delete(ctx, nil); err != nil {
				return fmt.Errorf("cannot delete %q at %s: %w", path, fs.Container, err)
			}
		}
	}

	return nil
}

func (fs *FS) deleteObject(path string) error {
	bc := fs.clientForPath(path)

	ctx := context.Background()
	if _, err := bc.Delete(ctx, nil); err != nil {
		return fmt.Errorf("cannot delete %q at %s: %w", bc.URL(), fs, err)
	}
	return nil
}

// CreateFile creates filePath at fs and puts data into it.
//
// The file is overwritten if it exists.
func (fs *FS) CreateFile(filePath string, data []byte) error {
	path := path.Join(fs.Dir, filePath)
	bc := fs.clientForPath(path)

	ctx := context.Background()
	_, err := bc.UploadBuffer(ctx, data, &blockblob.UploadBufferOptions{
		Concurrency: 1,
	})

	if err != nil {
		return fmt.Errorf("cannot upload %d bytes to %q at %s (remote path %q): %w", len(data), filePath, fs, bc.URL(), err)
	}

	return nil
}

// HasFile returns true if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := path.Join(fs.Dir, filePath)
	bc := fs.clientForPath(path)

	ctx := context.Background()
	_, err := bc.GetProperties(ctx, nil)
	var azerr *azcore.ResponseError
	if errors.As(err, &azerr) {
		if azerr.ErrorCode == storageErrorCodeBlobNotFound {
			return false, nil
		}
		logger.Errorf("GetProperties(%q) returned %s", bc.URL(), err)
		return false, fmt.Errorf("unexpected error when obtaining properties for %q at %s (remote path %q): %w", filePath, fs, bc.URL(), err)
	}

	return true, nil
}

// ReadFile returns the content of filePath at fs.
func (fs *FS) ReadFile(filePath string) ([]byte, error) {
	resp, err := fs.clientForPath(fs.Dir+filePath).DownloadStream(context.Background(), &blob.DownloadStreamOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot download %q at %s (remote dir %q): %w", filePath, fs, fs.Dir, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q at %s (remote dir %q): %w", filePath, fs, fs.Dir, err)
	}

	return b, nil
}
