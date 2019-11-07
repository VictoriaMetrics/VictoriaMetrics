package fsnil

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

// FS represents nil remote filesystem.
type FS struct{}

// String returns human-readable string representation for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("fsnil")
}

// ListParts returns all the parts from fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	return nil, nil
}
