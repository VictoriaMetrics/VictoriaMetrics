package common

import (
	"io"
)

// OriginFS is an interface for remote origin filesystem.
//
// This filesystem is used for performing server-side file copies
// instead of uploading data from local filesystem.
type OriginFS interface {
	// String must return human-readable representation of OriginFS.
	String() string

	// ListParts must return all the parts for the OriginFS.
	ListParts() ([]Part, error)
}

// RemoteFS is a filesystem where backups are stored.
type RemoteFS interface {
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

	// DeleteFile deletes filePath at RemoteFS
	DeleteFile(filePath string) error

	// CreateFile creates filePath at RemoteFS and puts data into it.
	CreateFile(filePath string, data []byte) error

	// HasFile returns true if filePath exists at RemoteFS.
	HasFile(filePath string) (bool, error)
}
