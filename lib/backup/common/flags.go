package common

import (
	"flag"
)

var (
	// DeleteAllObjectVersions is a flag for whether to prune previous object versions when deleting an object.
	DeleteAllObjectVersions = flag.Bool("deleteAllObjectVersions", false, "Whether to prune previous object versions when deleting an object. "+
		"By default, when object storage has versioning enabled deleting the file removes only current version. "+
		"This option forces removal of all previous versions. "+
		"See: https://docs.victoriametrics.com/victoriametrics/vmbackup/#permanent-deletion-of-objects-in-s3-compatible-storages")
)
