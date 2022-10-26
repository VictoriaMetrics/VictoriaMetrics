package azremote

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	bc := fs.clientForPart(p)
	ctx := context.Background()
	if _, err := bc.Delete(ctx, &blob.DeleteOptions{}); err != nil {
		return fmt.Errorf("cannot delete %q at %s (remote path %q): %w", p.Path, fs, bc.URL(), err)
	}
	return nil
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

	t, err := sbc.GetSASURL(ssCopyPermission, time.Now().Add(-10*time.Minute), time.Now().Add(30*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to generate SAS token of src %q: %w", p.Path, err)
	}

	// Hotfix for SDK issue: https://github.com/Azure/azure-sdk-for-go/issues/19245
	t = strings.Replace(t, "/?", "?", -1)
	ctx := context.Background()
	_, err = dbc.CopyFromURL(ctx, t, &blob.CopyFromURLOptions{})
	if err != nil {
		return fmt.Errorf("cannot copy %q from %s to %s: %w", p.Path, src, fs, err)
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

	path := fs.Dir + filePath
	bc := fs.clientForPath(path)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if _, err := bc.Delete(ctx, nil); err != nil {
		return fmt.Errorf("cannot delete %q at %s (remote path %q): %w", filePath, fs, bc.URL(), err)
	}
	return nil
}

// CreateFile creates filePath at fs and puts data into it.
//
// The file is overwritten if it exists.
func (fs *FS) CreateFile(filePath string, data []byte) error {
	path := fs.Dir + filePath
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

// HasFile returns ture if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := fs.Dir + filePath

	bc := fs.clientForPath(path)

	ctx := context.Background()
	_, err := bc.GetProperties(ctx, nil)
	logger.Errorf("GetProperties(%q) returned %s", bc.URL(), err)
	var azerr *azcore.ResponseError
	if errors.As(err, &azerr) {
		if azerr.ErrorCode == storageErrorCodeBlobNotFound {
			return false, nil
		}
		return false, fmt.Errorf("unexpected error when obtaining properties for %q at %s (remote path %q): %w", filePath, fs, bc.URL(), err)
	}

	return true, nil
}
