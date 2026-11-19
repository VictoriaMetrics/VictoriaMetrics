package actions

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
)

func TestRestoreDownloadPathPartsConcurrencyPerFile(t *testing.T) {
	const (
		path     = "big/file"
		partSize = 4
	)
	parts := []common.Part{
		{
			Path:       path,
			FileSize:   3 * partSize,
			Offset:     0,
			Size:       partSize,
			ActualSize: partSize,
		},
		{
			Path:       path,
			FileSize:   3 * partSize,
			Offset:     partSize,
			Size:       partSize,
			ActualSize: partSize,
		},
		{
			Path:       path,
			FileSize:   3 * partSize,
			Offset:     2 * partSize,
			Size:       partSize,
			ActualSize: partSize,
		},
	}
	src := &concurrencyTrackingRemoteFS{
		parts: parts,
		data: map[uint64][]byte{
			0:            []byte("aaaa"),
			partSize:     []byte("bbbb"),
			2 * partSize: []byte("cccc"),
		},
	}
	dst := &fslocal.FS{
		Dir: t.TempDir(),
	}
	if err := dst.Init(); err != nil {
		t.Fatalf("unexpected error in dst.Init: %s", err)
	}
	defer dst.MustStop()

	r := &Restore{
		Src: src,
		Dst: dst,
	}
	var bytesDownloaded atomic.Uint64
	globalConcurrencyCh := make(chan struct{}, 3)
	if err := r.downloadPathParts(t.Context(), parts, 2, globalConcurrencyCh, &bytesDownloaded); err != nil {
		t.Fatalf("unexpected error in downloadPathParts: %s", err)
	}

	data, err := os.ReadFile(filepath.Join(dst.Dir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("cannot read restored file: %s", err)
	}
	if !bytes.Equal(data, []byte("aaaabbbbcccc")) {
		t.Fatalf("unexpected restored data; got %q; want %q", data, "aaaabbbbcccc")
	}
	if src.maxActivePerPath != 2 {
		t.Fatalf("unexpected max concurrency per file; got %d; want %d", src.maxActivePerPath, 2)
	}
	if src.maxActiveTotal > 3 {
		t.Fatalf("unexpected total concurrency; got %d; want <= 3", src.maxActiveTotal)
	}
}

type concurrencyTrackingRemoteFS struct {
	parts []common.Part
	data  map[uint64][]byte

	mu               sync.Mutex
	activeTotal      int
	activePerPath    map[string]int
	maxActiveTotal   int
	maxActivePerPath int
}

func (fs *concurrencyTrackingRemoteFS) MustStop() {}

func (fs *concurrencyTrackingRemoteFS) String() string {
	return "concurrencyTrackingRemoteFS"
}

func (fs *concurrencyTrackingRemoteFS) ListParts() ([]common.Part, error) {
	parts := append([]common.Part(nil), fs.parts...)
	return parts, nil
}

func (fs *concurrencyTrackingRemoteFS) DeletePart(_ common.Part) error {
	return nil
}

func (fs *concurrencyTrackingRemoteFS) RemoveEmptyDirs() error {
	return nil
}

func (fs *concurrencyTrackingRemoteFS) CopyPart(_ common.OriginFS, _ common.Part) error {
	return nil
}

func (fs *concurrencyTrackingRemoteFS) DownloadPart(p common.Part, w io.Writer) error {
	fs.mu.Lock()
	if fs.activePerPath == nil {
		fs.activePerPath = make(map[string]int)
	}
	fs.activeTotal++
	fs.activePerPath[p.Path]++
	fs.maxActiveTotal = max(fs.maxActiveTotal, fs.activeTotal)
	fs.maxActivePerPath = max(fs.maxActivePerPath, fs.activePerPath[p.Path])
	fs.mu.Unlock()

	time.Sleep(50 * time.Millisecond)
	_, err := w.Write(fs.data[p.Offset])

	fs.mu.Lock()
	fs.activeTotal--
	fs.activePerPath[p.Path]--
	fs.mu.Unlock()

	return err
}

func (fs *concurrencyTrackingRemoteFS) UploadPart(_ common.Part, _ io.Reader) error {
	return nil
}

func (fs *concurrencyTrackingRemoteFS) DeleteFile(_ string) error {
	return nil
}

func (fs *concurrencyTrackingRemoteFS) CreateFile(_ string, _ []byte) error {
	return nil
}

func (fs *concurrencyTrackingRemoteFS) HasFile(filePath string) (bool, error) {
	return false, nil
}

func (fs *concurrencyTrackingRemoteFS) ReadFile(filePath string) ([]byte, error) {
	return nil, fmt.Errorf("unexpected ReadFile call for %q", filePath)
}

func TestExtractPartitionName(t *testing.T) {
	f := func(partPath, expectedName string) {
		t.Helper()
		got := extractPartitionName(partPath)
		if got != expectedName {
			t.Fatalf("extractPartitionName(%q): got %q, want %q", partPath, got, expectedName)
		}
	}

	// Small partition files.
	f("data/small/2024_01/parts.json", "2024_01")
	f("data/small/2024_01/some/nested/file.bin", "2024_01")

	// Big partition files.
	f("data/big/2026_06/parts.json", "2026_06")

	// IndexDB partition files.
	f("data/indexdb/2023_12/index.dat", "2023_12")

	// Non-partition paths — expect empty string.
	f("data/small/file.bin", "")
	f("data/small/snapshots/file.bin", "")
	f("indexdb/1838A2E6BE5C4E9F/parts.json", "")
	f("metadata/tenantsMetadata.json", "")
	f("parts.json", "")
	f("backup_complete", "")
	f("", "")
}

func TestFilterPartitions(t *testing.T) {
	// Fixed reference time: 2026-06-28 00:00:00 UTC
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)

	f := func(restoreSince time.Duration, restorePartitionsRegexp string, wantPaths []string) {
		t.Helper()

		makePart := func(path string) common.Part {
			return common.Part{Path: path, FileSize: 100, Size: 100, ActualSize: 100}
		}

		allParts := []common.Part{
			makePart("data/small/2024_01/parts.json"),
			makePart("data/small/2024_01/file.bin"),
			makePart("data/big/2024_01/big.bin"),
			makePart("data/indexdb/2024_01/index.dat"),
			makePart("data/small/2024_06/parts.json"),
			makePart("data/small/2024_06/file.bin"),
			makePart("data/big/2024_06/big.bin"),
			makePart("data/indexdb/2024_06/index.dat"),
			makePart("data/small/2025_12/file.bin"),
			makePart("data/small/2026_06/parts.json"),
			makePart("data/small/2026_06/file.bin"),
			makePart("data/big/2026_06/big.bin"),
			makePart("data/indexdb/2026_06/index.dat"),
			makePart("data/big/snapshots/data.bin"),
			makePart("indexdb/1838A2E6BE5C4E9F/parts.json"),
			makePart("metadata/tenantsMetadata.json"),
		}

		var partitionRe *regexp.Regexp
		if restorePartitionsRegexp != "" {
			partitionRe = regexp.MustCompile(restorePartitionsRegexp)
		}
		got, err := filterPartitions(allParts, restoreSince, partitionRe, now)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		gotPaths := make([]string, len(got))
		for i, p := range got {
			gotPaths[i] = p.Path
		}
		if !reflect.DeepEqual(gotPaths, wantPaths) {
			t.Fatalf("unexpected parts\ngot\n%v\nwant\n%v", gotPaths, wantPaths)
		}
	}

	// No filters — all parts pass through except the legacy indexdb.
	f(0, "", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/small/2025_12/file.bin",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_01 — only Jan 2024 plus non-partition files.
	f(0, "2024_01", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_01|2026_06 — two explicit partitions plus non-partition files.
	f(0, "2024_01|2026_06", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_.* — every 2024 partition (regexp, not a literal list) plus non-partition files.
	f(0, "2024_.*", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restoreSince=2y from 2026-06-28: cutoff is 2024-06-28.
	// 2024_01 ends 2024-02-01 — before cutoff, excluded.
	// 2024_06 ends 2024-07-01 — after cutoff (Jul 1 > Jun 28), included.
	// 2025_12 ends 2026-01-01 — after cutoff, included.
	// 2026_06 ends 2026-07-01 — after cutoff, included.
	twoYears := 2 * 365 * 24 * time.Hour
	f(twoYears, "", []string{
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/small/2025_12/file.bin",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// December rollover: 2025_12 ends 2026-01-01 (time.Date normalizes month 13)
	// Cutoff 2025-12-31 23:00 — 2025_12 included.
	f(now.Sub(time.Date(2025, 12, 31, 23, 0, 0, 0, time.UTC)), "", []string{
		"data/small/2025_12/file.bin",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})
	// Cutoff exactly 2026-01-01 — 2025_12 excluded, since the partition end is exclusive.
	f(now.Sub(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), "", []string{
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// Both filters: restoreSince=2y AND restorePartitions=2026_06 — only 2026_06 plus non-partition files.
	f(twoYears, "2026_06", []string{
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})
}

func TestFilterPartitionsNothingSelected(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	parts := []common.Part{
		{Path: "data/small/2024_01/file.bin", FileSize: 100, Size: 100, ActualSize: 100},
		{Path: "data/small/2024_02/file.bin", FileSize: 100, Size: 100, ActualSize: 100},
		{Path: "metadata/tenantsMetadata.json", FileSize: 100, Size: 100, ActualSize: 100},
	}

	_, err := filterPartitions(parts, 0, regexp.MustCompile("9999_01"), now)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	errExpected := "-restoreSince and -restorePartitions don't select any of 2 partitions in the backup; check the flag values"
	if err.Error() != errExpected {
		t.Fatalf("unexpected error\ngot\n%s\nwant\n%s", err, errExpected)
	}
}
