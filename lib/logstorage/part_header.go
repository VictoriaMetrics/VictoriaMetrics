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
	// FormatVersion is the version of the part format
	FormatVersion uint

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

	// BloomValuesShardsCount is the number of (bloom, values) shards in the part.
	BloomValuesShardsCount uint64
}

// reset resets ph for subsequent reuse
func (ph *partHeader) reset() {
	ph.FormatVersion = 0
	ph.CompressedSizeBytes = 0
	ph.UncompressedSizeBytes = 0
	ph.RowsCount = 0
	ph.BlocksCount = 0
	ph.MinTimestamp = 0
	ph.MaxTimestamp = 0
	ph.BloomValuesShardsCount = 0
}

// String returns string representation for ph.
func (ph *partHeader) String() string {
	return fmt.Sprintf("{FormatVersion=%d, CompressedSizeBytes=%d, UncompressedSizeBytes=%d, RowsCount=%d, BlocksCount=%d, "+
		"MinTimestamp=%s, MaxTimestamp=%s, BloomValuesShardsCount=%d}",
		ph.FormatVersion, ph.CompressedSizeBytes, ph.UncompressedSizeBytes, ph.RowsCount, ph.BlocksCount,
		timestampToString(ph.MinTimestamp), timestampToString(ph.MaxTimestamp), ph.BloomValuesShardsCount)
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

	if ph.FormatVersion <= 1 {
		if ph.BloomValuesShardsCount != 0 {
			logger.Panicf("FATAL: %s: unexpected BloomValuesShardsCount for FormatVersion<=1; got %d; want 0", metadataPath, ph.BloomValuesShardsCount)
		}
		if ph.FormatVersion == 1 {
			ph.BloomValuesShardsCount = 8
		}
	}

	// Perform various checks
	if ph.FormatVersion > partFormatLatestVersion {
		logger.Panicf("FATAL: %s: unsupported part format version; got %d; mustn't exceed %d", metadataPath, ph.FormatVersion, partFormatLatestVersion)
	}
	if ph.MinTimestamp > ph.MaxTimestamp {
		logger.Panicf("FATAL: %s: MinTimestamp cannot exceed MaxTimestamp; got %d vs %d", metadataPath, ph.MinTimestamp, ph.MaxTimestamp)
	}
	if ph.BlocksCount > ph.RowsCount {
		logger.Panicf("FATAL: %s: BlocksCount=%d cannot exceed RowsCount=%d", metadataPath, ph.BlocksCount, ph.RowsCount)
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
