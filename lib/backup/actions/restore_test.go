package actions

import (
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
)

func TestAdaptiveWorkQueueSplitsLargeParts(t *testing.T) {
	const (
		path     = "big/file"
		fileSize = 1000 * 1024 * 1024 // 1GB file
	)
	parts := []common.Part{
		{
			Path:       path,
			FileSize:   fileSize,
			Offset:     0,
			Size:       fileSize,
			ActualSize: fileSize,
		},
	}
	src := &concurrencyTrackingRemoteFS{
		parts: parts,
		data:  make(map[uint64][]byte),
	}
	
	r := &Restore{
		Src: src,
		Dst: &fslocal.FS{Dir: t.TempDir()},
	}

	perPath := map[string][]common.Part{
		path: parts,
	}

	// Build work queue with 4 concurrent workers
	workQueue := r.buildAdaptiveWorkQueue(perPath, 4)

	if len(workQueue) == 0 {
		t.Fatalf("expected work queue items, got none")
	}

	// Verify large file was split into multiple parts
	// With concurrency=4 and optimal size calculation, should create multiple chunks
	if len(workQueue) < 2 {
		t.Fatalf("expected file to be split into multiple parts; got %d", len(workQueue))
	}

	// Verify first part has preallocation flag, last has finalization flag
	firstItem := workQueue[0]
	lastItem := workQueue[len(workQueue)-1]

	if !firstItem.isFirstPart {
		t.Fatalf("expected first item to have isFirstPart=true")
	}
	if !lastItem.isLastPart {
		t.Fatalf("expected last item to have isLastPart=true")
	}

	// Verify all parts are within the file bounds
	totalDownloaded := uint64(0)
	for _, item := range workQueue {
		if item.part.Offset+item.part.Size > fileSize {
			t.Fatalf("part exceeds file bounds: offset=%d, size=%d, fileSize=%d",
				item.part.Offset, item.part.Size, fileSize)
		}
		totalDownloaded += item.part.Size
	}
	if totalDownloaded != fileSize {
		t.Fatalf("unexpected total downloaded size; got %d; want %d", totalDownloaded, fileSize)
	}
}

func TestAdaptiveWorkQueueSmallPartsNotSplit(t *testing.T) {
	const (
		path     = "small/file"
		fileSize = 10 * 1024 * 1024 // 10MB file
	)
	parts := []common.Part{
		{
			Path:       path,
			FileSize:   fileSize,
			Offset:     0,
			Size:       fileSize,
			ActualSize: fileSize,
		},
	}
	src := &concurrencyTrackingRemoteFS{
		parts: parts,
		data:  make(map[uint64][]byte),
	}
	
	r := &Restore{
		Src: src,
		Dst: &fslocal.FS{Dir: t.TempDir()},
	}

	perPath := map[string][]common.Part{
		path: parts,
	}

	// Build work queue with 4 concurrent workers
	workQueue := r.buildAdaptiveWorkQueue(perPath, 4)

	// Small file should not be split
	if len(workQueue) != 1 {
		t.Fatalf("expected single work item for small file; got %d", len(workQueue))
	}

	item := workQueue[0]
	if item.part.Size != fileSize {
		t.Fatalf("expected part size %d; got %d", fileSize, item.part.Size)
	}
}

func TestOptimalPartSizeCalculation(t *testing.T) {
	tests := []struct {
		concurrency int
		expectedMin uint64
		expectedMax uint64
	}{
		{1, 500 * 1024 * 1024, 500 * 1024 * 1024},     // 1 worker: max size
		{2, 500 * 1024 * 1024, 500 * 1024 * 1024},     // 2 workers: max size
		{4, 100 * 1024 * 1024, 100 * 1024 * 1024},     // 4 workers: 100MB
		{10, 50 * 1024 * 1024, 50 * 1024 * 1024},      // 10 workers: min size
		{16, 50 * 1024 * 1024, 50 * 1024 * 1024},      // 16 workers: min size
	}

	for _, tt := range tests {
		size := calculateOptimalPartSize(tt.concurrency)
		if size < tt.expectedMin || size > tt.expectedMax {
			t.Fatalf("concurrency=%d: expected size in [%d, %d], got %d",
				tt.concurrency, tt.expectedMin, tt.expectedMax, size)
		}
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
