package common

import (
	"bufio"
	"fmt"
	"io"
)

// OriginFS is an interface for remote origin filesystem.
//
// This filesystem is used for performing server-side file copies
// instead of uploading data from local filesystem.
type OriginFS interface {
	// MustStop must be called when the RemoteFS is no longer needed.
	MustStop()

	// String must return human-readable representation of OriginFS.
	String() string

	// ListParts must return all the parts for the OriginFS.
	ListParts() ([]Part, error)
}

// RemoteFS is a filesystem where backups are stored.
type RemoteFS interface {
	// MustStop must be called when the RemoteFS is no longer needed.
	MustStop()

	// String must return human-readable representation of RemoteFS.
	String() string

	// ListParts must return all the parts for the RemoteFS.
	ListParts() ([]Part, error)

	// DeletePart must delete part p from RemoteFS.
	DeletePart(p Part) error

	// RemoveEmptyDirs must recursively remove empty directories in RemoteFS.
	RemoveEmptyDirs() error

	// CopyPart must copy part p from dstFS to RemoteFS.
	CopyPart(dstFS OriginFS, p Part) error

	// DownloadPart must download part p from RemoteFS to w.
	DownloadPart(p Part, w io.Writer) error

	// UploadPart must upload part p from r to RemoteFS.
	UploadPart(p Part, r io.Reader) error

	// DeleteFile deletes filePath at RemoteFS.
	//
	// filePath must use / as directory delimiters.
	DeleteFile(filePath string) error

	// CreateFile creates filePath at RemoteFS and puts data into it.
	//
	// filePath must use / as directory delimiters.
	CreateFile(filePath string, data []byte) error

	// HasFile returns true if filePath exists at RemoteFS.
	//
	// filePath must use / as directory delimiters.
	HasFile(filePath string) (bool, error)

	// ReadFile returns file contents at the given filePath.
	//
	// filePath must use / as directory delimiters.
	ReadFile(filePath string) ([]byte, error)
}

// CrossTypeCopy downloads part p from src and uploads to dst using streaming.
// This is used when src and dst are different RemoteFS types (e.g., S3 to GCS).
// It uses io.Pipe with buffering to allow download to run ahead of upload for better throughput.
func CrossTypeCopy(src RemoteFS, dst RemoteFS, p Part) error {
	pr, pw := io.Pipe()

	errCh := make(chan error, 1)

	// Download in goroutine with buffering to prevent lock-step serialization
	go func() {
		// Use 1MB buffer to allow download to run ahead of upload
		buf := bufio.NewWriterSize(pw, 1024*1024)
		err := src.DownloadPart(p, buf)
		if err == nil {
			err = buf.Flush()
		}
		// Propagate error to reader if download/flush failed
		pw.CloseWithError(err)
		errCh <- err
	}()

	// Upload from pipe
	uploadErr := dst.UploadPart(p, pr)
	pr.Close()

	// Wait for download to complete
	downloadErr := <-errCh

	// Check upload error first - if upload failed, that's the root cause
	if uploadErr != nil {
		return fmt.Errorf("cannot upload %s to %s: %w", &p, dst, uploadErr)
	}
	if downloadErr != nil {
		return fmt.Errorf("cannot download %s from %s: %w", &p, src, downloadErr)
	}

	return nil
}
