package fsnil

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

// FS represents nil remote filesystem.
type FS struct{}

// MustStop stops fs.
func (fs *FS) MustStop() {
	// Nothing to do
}

// String returns human-readable string representation for fs.
func (fs *FS) String() string {
	return "fsnil"
}

// ListParts returns all the parts from fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	return nil, nil
}

// DownloadPart returns an error, since fs has no parts.
func (fs *FS) DownloadPart(p common.Part, _ io.Writer) error {
	return fmt.Errorf("BUG: cannot download %s from fsnil", &p)
}
