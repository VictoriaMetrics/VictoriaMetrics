package snapshotutil

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var snapshotNameRegexp = regexp.MustCompile(`^[0-9]{14}-[0-9A-Fa-f]+$`)

// Validate validates the snapshotName
func Validate(snapshotName string) error {
	_, err := Time(snapshotName)
	return err
}

// Time returns snapshot creation time from the given snapshotName
func Time(snapshotName string) (time.Time, error) {
	if !snapshotNameRegexp.MatchString(snapshotName) {
		return time.Time{}, fmt.Errorf("unexpected snapshot name=%q; it must match %q regexp", snapshotName, snapshotNameRegexp.String())
	}
	n := strings.IndexByte(snapshotName, '-')
	if n < 0 {
		logger.Panicf("BUG: cannot find `-` in snapshotName=%q", snapshotName)
	}
	s := snapshotName[:n]
	t, err := time.Parse("20060102150405", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("unexpected timestamp=%q in snapshot name: %w; it must match YYYYMMDDhhmmss pattern", s, err)
	}
	return t, nil
}

// NewName returns new name for new snapshot
func NewName() string {
	return fmt.Sprintf("%s-%08X", time.Now().UTC().Format("20060102150405"), nextSnapshotIdx())
}

func nextSnapshotIdx() uint64 {
	return snapshotIdx.Add(1)
}

var snapshotIdx = func() *atomic.Uint64 {
	var x atomic.Uint64
	x.Store(uint64(time.Now().UnixNano()))
	return &x
}()
