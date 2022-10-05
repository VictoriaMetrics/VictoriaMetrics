package azremote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	envStorageAcctName = "AZURE_STORAGE_ACCOUNT_NAME"
	envStorageAccKey   = "AZURE_STORAGE_ACCOUNT_KEY"
	envStorageAccCs    = "AZURE_STORAGE_ACCOUNT_CONNECTION_STRING"
)

// FS represents filesystem for backups in Azure Blob Storage.
//
// Init must be called before calling other FS methods.
type FS struct {
	// Azure Blob Storage bucket to use.
	Container string

	// Directory in the bucket to write to.
	Dir string

	client *azblob.ContainerClient
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

	var sc *azblob.ServiceClient
	var err error
	if cs, ok := os.LookupEnv(envStorageAccCs); ok {
		sc, err = azblob.NewServiceClientFromConnectionString(cs, nil)
		if err != nil {
			return fmt.Errorf("failed to create AZBlob service client from connection string: %w", err)
		}
	}

	accountName, ok1 := os.LookupEnv(envStorageAcctName)
	accountKey, ok2 := os.LookupEnv(envStorageAccKey)
	if ok1 && ok2 {
		creds, err := azblob.NewSharedKeyCredential(accountName, accountKey)
		if err != nil {
			return fmt.Errorf("failed to create AZBlob credentials from account name and key: %w", err)
		}
		serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)

		sc, err = azblob.NewServiceClientWithSharedKey(serviceURL, creds, nil)
		if err != nil {
			return fmt.Errorf("failed to create AZBlob service client from account name and key: %w", err)
		}
	}

	if sc == nil {
		return fmt.Errorf(`failed to detect any credentials type for AZBlob. Ensure there is connection string set at %q, or shared key at %q and %q`, envStorageAccCs, envStorageAcctName, envStorageAccKey)
	}

	containerClient, err := sc.NewContainerClient(fs.Container)
	if err != nil {
		return fmt.Errorf("failed to create AZBlob container client: %w", err)
	}

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

	opts := &azblob.ContainerListBlobsFlatOptions{
		Prefix: &dir,
	}

	pager := fs.client.ListBlobsFlat(opts)
	var parts []common.Part
	for pager.NextPage(ctx) {
		resp := pager.PageResponse()

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
				logger.Infof("skipping unknown object %q", file)
				continue
			}

			p.ActualSize = uint64(*v.Properties.ContentLength)
			parts = append(parts, p)
		}

	}

	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("error when iterating objects at %q: %w", dir, err)
	}

	return parts, nil
}

// DeletePart deletes part p from fs.
func (fs *FS) DeletePart(p common.Part) error {
	bc, err := fs.clientForPart(p)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if _, err := bc.Delete(ctx, &azblob.BlobDeleteOptions{}); err != nil {
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

	sbc, err := src.clientForPart(p)
	if err != nil {
		return fmt.Errorf("failed to initialize server-side copy of src %q: %w", p.Path, err)
	}
	dbc, err := fs.clientForPart(p)
	if err != nil {
		return fmt.Errorf("failed to initialize server-side copy of dst %q: %w", p.Path, err)
	}

	ssCopyPermission := azblob.BlobSASPermissions{
		Read:   true,
		Create: true,
		Write:  true,
	}
	t, err := sbc.GetSASToken(ssCopyPermission, time.Now(), time.Now().Add(30*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to generate SAS token of src %q: %w", p.Path, err)
	}

	srcURL := sbc.URL() + "?" + t.Encode()

	ctx := context.Background()
	_, err = dbc.CopyFromURL(ctx, srcURL, &azblob.BlockBlobCopyFromURLOptions{})
	if err != nil {
		return fmt.Errorf("cannot copy %q from %s to %s: %w", p.Path, src, fs, err)
	}

	return nil
}

// DownloadPart downloads part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	bc, err := fs.clientForPart(p)
	if err != nil {
		return err
	}

	ctx := context.Background()
	r, err := bc.Download(ctx, &azblob.BlobDownloadOptions{})
	if err != nil {
		return fmt.Errorf("cannot open reader for %q at %s (remote path %q): %w", p.Path, fs, bc.URL(), err)
	}

	body := r.Body(&azblob.RetryReaderOptions{})
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
	bc, err := fs.clientForPart(p)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = bc.UploadStream(ctx, r, azblob.UploadStreamOptions{})

	if err != nil {
		return fmt.Errorf("cannot upload data to %q at %s (remote path %q): %w", p.Path, fs, bc.URL(), err)
	}

	return nil
}

func (fs *FS) clientForPart(p common.Part) (*azblob.BlockBlobClient, error) {
	path := p.RemotePath(fs.Dir)

	return fs.clientForPath(path)
}

func (fs *FS) clientForPath(path string) (*azblob.BlockBlobClient, error) {
	bc, err := fs.client.NewBlockBlobClient(path)
	if err != nil {
		return nil, fmt.Errorf("unexpected error when creating client for blob %q: %w", path, err)
	}

	return bc, nil
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
	bc, err := fs.clientForPath(path)
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
	bc, err := fs.clientForPath(path)
	if err != nil {
		return err
	}

	ctx := context.Background()
	r, err := bc.UploadBuffer(ctx, data, azblob.UploadOption{
		Parallelism: 1,
	})
	defer func() { _ = r.Body.Close() }()

	if err != nil {
		return fmt.Errorf("cannot upload %d bytes to %q at %s (remote path %q): %w", len(data), filePath, fs, bc.URL(), err)
	}

	return nil
}

// HasFile returns ture if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := fs.Dir + filePath

	bc, err := fs.clientForPath(path)
	if err != nil {
		return false, err
	}

	ctx := context.Background()
	_, err = bc.GetProperties(ctx, nil)

	var azerr *azblob.InternalError
	var sterr *azblob.StorageError

	if errors.As(err, &azerr) && azerr.As(&sterr) {
		if sterr.ErrorCode == azblob.StorageErrorCodeBlobNotFound {
			return false, nil
		}
		return false, fmt.Errorf("unexpected error when obtaining properties for %q at %s (remote path %q): %w", filePath, fs, bc.URL(), err)
	}

	return true, nil
}

// ListDirs returns list of subdirectories in given directory
func (fs *FS) ListDirs(subpath string) ([]string, error) {
	path := strings.TrimPrefix(filepath.Join(fs.Dir, subpath), "/")
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}

	var dirs []string

	const dirsDelimiter = "/"
	pager := fs.client.ListBlobsHierarchy(dirsDelimiter, &azblob.ContainerListBlobsHierarchyOptions{
		Prefix: &fs.Container,
	})
	ctx := context.Background()
	for pager.NextPage(ctx) {
		resp := pager.PageResponse()

		const dirsDelimiter = "/"

		for _, v := range resp.Segment.BlobPrefixes {
			dir := *v.Name
			if !strings.HasPrefix(dir, path) {
				return nil, fmt.Errorf("unexpected prefix for AZBlob key %q; want %q", dir, dir)
			}
			dir = strings.TrimPrefix(dir, path)
			if fscommon.IgnorePath(dir) || !strings.Contains(dir, dirsDelimiter) {
				continue
			}
			dirs = append(dirs, strings.TrimSuffix(dir, dirsDelimiter))
		}
	}

	return dirs, nil
}

// DeleteFiles deletes files at fs.
//
// The function does nothing if the files don't exist.
func (fs *FS) DeleteFiles(filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}
	for _, filePath := range filePaths {
		path := filePath
		if fs.Dir != "/" {
			path = filepath.Join(fs.Dir + path)
		}

		ctx := context.Background()

		opts := &azblob.ContainerListBlobsFlatOptions{
			Prefix: &path,
		}

		pager := fs.client.ListBlobsFlat(opts)
		for pager.NextPage(ctx) {
			resp := pager.PageResponse()

			for _, v := range resp.Segment.BlobItems {
				file := *v.Name

				bc, err := fs.clientForPath(file)
				if err != nil {
					return err
				}

				_, err = bc.Delete(ctx, &azblob.BlobDeleteOptions{})
				if err != nil {
					return fmt.Errorf("cannot delete %q at %s (remote dir %q): %w", file, fs, fs.Dir, err)
				}
			}
		}
	}
	return nil
}
