package storage

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
}

// String returns string representation of ph.
func (ph *partHeader) String() string {
	return fmt.Sprintf("%d_%d_%s_%s", ph.RowsCount, ph.BlocksCount, toUserReadableTimestamp(ph.MinTimestamp), toUserReadableTimestamp(ph.MaxTimestamp))
}

func toUserReadableTimestamp(timestamp int64) string {
	t := timestampToTime(timestamp)
	return t.Format(userReadableTimeFormat)
}

func fromUserReadableTimestamp(s string) (int64, error) {
	t, err := time.Parse(userReadableTimeFormat, s)
	if err != nil {
		return 0, err
	}
	return timestampFromTime(t), nil
}

const userReadableTimeFormat = "20060102150405.000"

// Path returns a path to part header with the given prefix and suffix.
//
// Suffix must be random.
func (ph *partHeader) Path(prefix string, suffix uint64) string {
	prefix = filepath.Clean(prefix)
	s := ph.String()
	return fmt.Sprintf("%s/%s_%016X", prefix, s, suffix)
}

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

	return nil
}

// Reset resets the ph.
func (ph *partHeader) Reset() {
	ph.RowsCount = 0
	ph.BlocksCount = 0
	ph.MinTimestamp = (1 << 63) - 1
	ph.MaxTimestamp = -1 << 63
}
