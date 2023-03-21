package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// partHeader represents part header.
type partHeader struct {
	// RowsCount is the total number of rows in the part.
	RowsCount uint64

	// BlocksCount is the total number of blocks in the part.
	BlocksCount uint64

	// MinTimestamp is the minimum timestamp in the part.
	MinTimestamp int64

	// MaxTimestamp is the maximum timestamp in the part.
	MaxTimestamp int64

	// MinDedupInterval is minimal dedup interval in milliseconds across all the blocks in the part.
	MinDedupInterval int64
}

// String returns string representation of ph.
func (ph *partHeader) String() string {
	return fmt.Sprintf("partHeader{rowsCount=%d,blocksCount=%d,minTimestamp=%d,maxTimestamp=%d}", ph.RowsCount, ph.BlocksCount, ph.MinTimestamp, ph.MaxTimestamp)
}

// Reset resets the ph.
func (ph *partHeader) Reset() {
	ph.RowsCount = 0
	ph.BlocksCount = 0
	ph.MinTimestamp = (1 << 63) - 1
	ph.MaxTimestamp = -1 << 63
	ph.MinDedupInterval = 0
}

func (ph *partHeader) readMinDedupInterval(partPath string) error {
	filePath := partPath + "/min_dedup_interval"
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The minimum dedup interval may not exist for old parts.
			ph.MinDedupInterval = 0
			return nil
		}
		return fmt.Errorf("cannot read %q: %w", filePath, err)
	}
	dedupInterval, err := promutils.ParseDuration(string(data))
	if err != nil {
		return fmt.Errorf("cannot parse minimum dedup interval %q at %q: %w", data, filePath, err)
	}
	ph.MinDedupInterval = dedupInterval.Milliseconds()
	return nil
}

func fromUserReadableTimestamp(s string) (int64, error) {
	t, err := time.Parse(userReadableTimeFormat, s)
	if err != nil {
		return 0, err
	}
	return timestampFromTime(t), nil
}

const userReadableTimeFormat = "20060102150405.000"

// ParseFromPath extracts ph info from the given path.
func (ph *partHeader) ParseFromPath(path string) error {
	ph.Reset()

	path = filepath.Clean(path)

	// Extract encoded part name.
	n := strings.LastIndexByte(path, '/')
	if n < 0 {
		return fmt.Errorf("cannot find encoded part name in the path %q", path)
	}
	partName := path[n+1:]

	// PartName must have the following form:
	// RowsCount_BlocksCount_MinTimestamp_MaxTimestamp_Garbage
	a := strings.Split(partName, "_")
	if len(a) != 5 {
		return fmt.Errorf("unexpected number of substrings in the part name %q: got %d; want %d", partName, len(a), 5)
	}

	var err error

	ph.RowsCount, err = strconv.ParseUint(a[0], 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse rowsCount from partName %q: %w", partName, err)
	}
	ph.BlocksCount, err = strconv.ParseUint(a[1], 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse blocksCount from partName %q: %w", partName, err)
	}
	ph.MinTimestamp, err = fromUserReadableTimestamp(a[2])
	if err != nil {
		return fmt.Errorf("cannot parse minTimestamp from partName %q: %w", partName, err)
	}
	ph.MaxTimestamp, err = fromUserReadableTimestamp(a[3])
	if err != nil {
		return fmt.Errorf("cannot parse maxTimestamp from partName %q: %w", partName, err)
	}

	if ph.MinTimestamp > ph.MaxTimestamp {
		return fmt.Errorf("minTimestamp cannot exceed maxTimestamp; got %d vs %d", ph.MinTimestamp, ph.MaxTimestamp)
	}
	if ph.RowsCount <= 0 {
		return fmt.Errorf("rowsCount must be greater than 0; got %d", ph.RowsCount)
	}
	if ph.BlocksCount <= 0 {
		return fmt.Errorf("blocksCount must be greater than 0; got %d", ph.BlocksCount)
	}
	if ph.BlocksCount > ph.RowsCount {
		return fmt.Errorf("blocksCount cannot be bigger than rowsCount; got blocksCount=%d, rowsCount=%d", ph.BlocksCount, ph.RowsCount)
	}

	if err := ph.readMinDedupInterval(path); err != nil {
		return fmt.Errorf("cannot read min dedup interval: %w", err)
	}

	return nil
}

func (ph *partHeader) ReadMetadata(partPath string) error {
	ph.Reset()

	metadataPath := partPath + "/metadata.json"
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			// This is a part created before v1.90.0.
			// Fall back to reading the metadata from the partPath itsel
			return ph.ParseFromPath(partPath)
		}
		return fmt.Errorf("cannot read %q: %w", metadataPath, err)
	}
	if err := json.Unmarshal(metadata, ph); err != nil {
		return fmt.Errorf("cannot parse %q: %w", metadataPath, err)
	}

	// Perform various checks
	if ph.MinTimestamp > ph.MaxTimestamp {
		return fmt.Errorf("minTimestamp cannot exceed maxTimestamp; got %d vs %d", ph.MinTimestamp, ph.MaxTimestamp)
	}
	if ph.RowsCount <= 0 {
		return fmt.Errorf("rowsCount must be greater than 0; got %d", ph.RowsCount)
	}
	if ph.BlocksCount <= 0 {
		return fmt.Errorf("blocksCount must be greater than 0; got %d", ph.BlocksCount)
	}
	if ph.BlocksCount > ph.RowsCount {
		return fmt.Errorf("blocksCount cannot be bigger than rowsCount; got blocksCount=%d, rowsCount=%d", ph.BlocksCount, ph.RowsCount)
	}

	return nil
}

func (ph *partHeader) WriteMetadata(partPath string) error {
	metadata, err := json.Marshal(ph)
	if err != nil {
		logger.Panicf("BUG: cannot marshal partHeader metadata: %s", err)
	}
	metadataPath := partPath + "/metadata.json"
	if err := fs.WriteFileAtomically(metadataPath, metadata, false); err != nil {
		return fmt.Errorf("cannot create %q: %w", metadataPath, err)
	}
	return nil
}
