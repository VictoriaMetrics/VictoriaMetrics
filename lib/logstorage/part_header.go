package logstorage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// partHeader contains the information about a single part
type partHeader struct {
	// CompressedSizeBytes is physical size of the part
	CompressedSizeBytes uint64

	// UncompressedSizeBytes is the original size of log entries stored in the part
	UncompressedSizeBytes uint64

	// RowsCount is the number of log entries in the part
	RowsCount uint64

	// BlocksCount is the number of blocks in the part
	BlocksCount uint64

	// MinTimestamp is the minimum timestamp seen in the part
	MinTimestamp int64

	// MaxTimestamp is the maximum timestamp seen in the part
	MaxTimestamp int64
}

// reset resets ph for subsequent re-use
func (ph *partHeader) reset() {
	ph.CompressedSizeBytes = 0
	ph.UncompressedSizeBytes = 0
	ph.RowsCount = 0
	ph.BlocksCount = 0
	ph.MinTimestamp = 0
	ph.MaxTimestamp = 0
}

// String returns string represenation for ph.
func (ph *partHeader) String() string {
	return fmt.Sprintf("{CompressedSizeBytes=%d, UncompressedSizeBytes=%d, RowsCount=%d, BlocksCount=%d, MinTimestamp=%s, MaxTimestamp=%s}",
		ph.CompressedSizeBytes, ph.UncompressedSizeBytes, ph.RowsCount, ph.BlocksCount, timestampToString(ph.MinTimestamp), timestampToString(ph.MaxTimestamp))
}

func (ph *partHeader) mustReadMetadata(partPath string) {
	ph.reset()

	metadataPath := filepath.Join(partPath, metadataFilename)
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		logger.Panicf("FATAL: cannot read %q: %s", metadataPath, err)
	}
	if err := json.Unmarshal(metadata, ph); err != nil {
		logger.Panicf("FATAL: cannot parse %q: %s", metadataPath, err)
	}

	// Perform various checks
	if ph.MinTimestamp > ph.MaxTimestamp {
		logger.Panicf("FATAL: MinTimestamp cannot exceed MaxTimestamp; got %d vs %d", ph.MinTimestamp, ph.MaxTimestamp)
	}
}

func (ph *partHeader) mustWriteMetadata(partPath string) {
	metadata, err := json.Marshal(ph)
	if err != nil {
		logger.Panicf("BUG: cannot marshal partHeader: %s", err)
	}
	metadataPath := filepath.Join(partPath, metadataFilename)
	fs.MustWriteSync(metadataPath, metadata)
}

func timestampToString(timestamp int64) string {
	t := time.Unix(0, timestamp).UTC()
	return strings.Replace(t.Format(timestampForPathname), ".", "", 1)
}

const timestampForPathname = "20060102150405.000000000"
