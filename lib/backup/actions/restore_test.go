package actions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
)

func TestRestoreDownloadConcurrency(t *testing.T) {
	testCases := []struct {
		name              string
		concurrency       int
		files             int
		skipPreallocation bool
		wholeObjects      bool
		wantActive        int
		wantPerFile       int
	}{
		{"same-file", 4, 1, false, false, 4, 4},
		{"multiple-files", 6, 2, false, false, 6, 4},
		{"global-limit", 3, 2, false, false, 3, 3},
		{"sequential-resume", 4, 2, true, false, 2, 1},
		{"non-range-source", 4, 2, false, true, 2, 1},
		{"zero-limit", 0, 1, false, false, 1, 1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := &concurrencyTrackingRemoteFS{data: make(map[common.Part][]byte)}
			perPath := make(map[string][]common.Part)
			for i := range tc.files {
				path := fmt.Sprintf("big/file%d", i)
				for j := 3; j >= 0; j-- {
					p := common.Part{Path: path, FileSize: 16, Offset: uint64(j * 4), Size: 4, ActualSize: 4}
					src.data[p] = bytes.Repeat([]byte{byte('a' + j)}, 4)
					perPath[path] = append(perPath[path], p)
				}
			}
			started := make(chan common.Part, len(src.data))
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			src.beforeDownload = func(p common.Part) error {
				started <- p
				<-release
				return nil
			}
			var source common.RemoteFS = src
			if tc.wholeObjects {
				source = struct{ common.RemoteFS }{src}
			}
			r := &Restore{
				Src:               source,
				Dst:               &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: !tc.skipPreallocation},
				SkipPreallocation: tc.skipPreallocation,
			}
			t.Cleanup(unblock)
			var downloaded atomic.Uint64
			done := make(chan error, 1)
			go func() {
				done <- r.runParallelPerPathInternal(context.Background(), tc.concurrency, perPath, &downloaded)
			}()
			for range tc.wantActive {
				select {
				case <-started:
				case err := <-done:
					t.Fatalf("restore finished before reaching expected concurrency: %v", err)
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for concurrent downloads")
				}
			}
			unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("unexpected restore error: %s", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for restore")
			}
			if src.maxActiveTotal != tc.wantActive || src.maxActivePerPath != tc.wantPerFile {
				t.Fatalf("unexpected maximum concurrency: total=%d, per file=%d; want %d, %d",
					src.maxActiveTotal, src.maxActivePerPath, tc.wantActive, tc.wantPerFile)
			}
			if downloaded.Load() != uint64(tc.files*16) {
				t.Fatalf("unexpected downloaded bytes: %d", downloaded.Load())
			}
			for path := range perPath {
				data, err := os.ReadFile(filepath.Join(r.Dst.Dir, filepath.FromSlash(path)))
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != "aaaabbbbccccdddd" {
					t.Fatalf("unexpected restored contents: %q", data)
				}
			}
			if tc.skipPreallocation || tc.wholeObjects {
				nextOffset := make(map[string]uint64)
				for _, p := range src.downloaded {
					if p.Offset != nextOffset[p.Path] {
						t.Fatalf("out-of-order resumable download: %s", &p)
					}
					nextOffset[p.Path] += p.Size
				}
			}
		})
	}
}

func TestBuildDownloadTasks(t *testing.T) {
	testCases := []struct {
		size        uint64
		concurrency int
		ranged      bool
		wantTasks   int
	}{
		{0, 10, true, 1},
		{100<<20 - 1, 10, true, 1},
		{100 << 20, 10, true, 2},
		{100<<20 + 1, 10, true, 2},
		{1 << 30, 2, true, 4},
		{1 << 30, 10, true, 20},
		{1 << 30, 1000, true, 20},
		{1 << 30, 1, true, 1},
		{1 << 30, 0, true, 1},
		{1 << 30, 10, false, 1},
	}
	for _, tc := range testCases {
		p := common.Part{Path: "data/file", FileSize: tc.size + 123, Offset: 123, Size: tc.size}
		tasks := buildDownloadTasks([]common.Part{p}, tc.concurrency, tc.ranged)
		if len(tasks) != tc.wantTasks {
			t.Fatalf("size=%d, concurrency=%d, ranged=%v: got %d tasks; want %d", tc.size, tc.concurrency, tc.ranged, len(tasks), tc.wantTasks)
		}
		offset := uint64(0)
		for _, task := range tasks {
			if task.part != p || task.offset != offset {
				t.Fatalf("object identity changed or range coverage has a gap: %+v", task)
			}
			if len(tasks) > 1 && (task.size < 50<<20 || task.size > 256<<20) {
				t.Fatalf("range size outside request bounds: %d", task.size)
			}
			offset += task.size
		}
		if offset != p.Size {
			t.Fatalf("incomplete range coverage: got %d; want %d", offset, p.Size)
		}
	}
}

func TestRestoreRangedDownloads(t *testing.T) {
	for _, skipPreallocation := range []bool{false, true} {
		t.Run(fmt.Sprintf("skipPreallocation=%v", skipPreallocation), func(t *testing.T) {
			const prefix = "prefix"
			data := bytes.Repeat([]byte("range data"), (200<<20)/10+1)
			p := common.Part{
				Path: "data/file", FileSize: uint64(len(prefix) + len(data)),
				Offset: uint64(len(prefix)), Size: uint64(len(data)), ActualSize: uint64(len(data)),
			}
			src := &concurrencyTrackingRemoteFS{data: map[common.Part][]byte{p: data}}
			started := make(chan struct{}, 16)
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			r := &Restore{
				Src:               src,
				Dst:               &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: !skipPreallocation},
				SkipPreallocation: skipPreallocation,
			}
			if err := os.MkdirAll(filepath.Dir(p.LocalPath(r.Dst.Dir)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p.LocalPath(r.Dst.Dir), []byte(prefix), 0600); err != nil {
				t.Fatal(err)
			}
			src.beforeRange = func(_ downloadTask) error {
				started <- struct{}{}
				<-release
				return nil
			}
			var downloaded atomic.Uint64
			done := make(chan error, 1)
			go func() {
				done <- r.runParallelPerPathInternal(context.Background(), 4, map[string][]common.Part{p.Path: {p}}, &downloaded)
			}()
			t.Cleanup(func() {
				unblock()
			})
			wantActive := 4
			if skipPreallocation {
				wantActive = 1
			}
			for range wantActive {
				select {
				case <-started:
				case err := <-done:
					t.Fatalf("restore finished before filling workers: %v", err)
				case <-time.After(5 * time.Second):
					t.Fatal("single-object restore didn't fill the worker pool")
				}
			}
			unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("ranged restore didn't finish")
			}
			if src.maxActiveTotal != wantActive || downloaded.Load() != p.Size {
				t.Fatalf("unexpected concurrency=%d or downloaded bytes=%d", src.maxActiveTotal, downloaded.Load())
			}
			sort.Slice(src.ranges, func(i, j int) bool { return src.ranges[i].offset < src.ranges[j].offset })
			offset := uint64(0)
			minSize, maxSize := p.Size, uint64(0)
			for _, task := range src.ranges {
				if task.part != p || task.offset != offset {
					t.Fatalf("object identity changed or range coverage has a gap: %+v", task)
				}
				minSize, maxSize = min(minSize, task.size), max(maxSize, task.size)
				offset += task.size
			}
			if offset != p.Size || maxSize-minSize > 1 {
				t.Fatalf("ranges don't cover the object evenly: offset=%d, min=%d, max=%d", offset, minSize, maxSize)
			}
			if skipPreallocation && len(src.ranges) != 1 {
				t.Fatalf("resumable download was split into %d ranges", len(src.ranges))
			}
			f, err := os.Open(p.LocalPath(r.Dst.Dir))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			gotHash := sha256.New()
			if _, err := io.Copy(gotHash, f); err != nil {
				t.Fatal(err)
			}
			wantHash := sha256.New()
			_, _ = io.WriteString(wantHash, prefix)
			_, _ = wantHash.Write(data)
			if !bytes.Equal(gotHash.Sum(nil), wantHash.Sum(nil)) {
				t.Fatal("range downloads corrupted the file or its existing prefix")
			}
		})
	}
}

func TestRestoreLargeBackupObject(t *testing.T) {
	data := bytes.Repeat([]byte("vmrestore"), (100<<20)/9+1)
	p := common.Part{Path: "data/big/2026_11/file", FileSize: uint64(len(data)), Size: uint64(len(data)), ActualSize: uint64(len(data))}
	src := &fsremote.FS{Dir: t.TempDir()}
	if err := src.UploadPart(p, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	empty := common.Part{Path: "metadata/empty"}
	if err := src.UploadPart(empty, bytes.NewReader(nil)); err != nil {
		t.Fatal(err)
	}
	if err := src.CreateFile(backupnames.BackupCompleteFilename, nil); err != nil {
		t.Fatal(err)
	}
	r := &Restore{
		Src:         src,
		Dst:         &fslocal.FS{Dir: t.TempDir()},
		Concurrency: 10,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("cannot restore original backup object: %s", err)
	}
	f, err := os.Open(p.LocalPath(r.Dst.Dir))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(data)
	if n != int64(len(data)) || !bytes.Equal(h.Sum(nil), wantHash[:]) {
		t.Fatal("restored file differs from backup")
	}
	fi, err := os.Stat(empty.LocalPath(r.Dst.Dir))
	if err != nil || fi.Size() != 0 {
		t.Fatalf("empty file wasn't restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.Dst.Dir, backupnames.RestoreInProgressFilename)); !os.IsNotExist(err) {
		t.Fatalf("incomplete restore marker remains after success: %v", err)
	}
}

func TestRestoreDownloadFailureAndResume(t *testing.T) {
	data := []byte("restored data")
	p := common.Part{Path: "data/file", FileSize: uint64(len(data)), Size: uint64(len(data)), ActualSize: uint64(len(data))}
	src := &concurrencyTrackingRemoteFS{parts: []common.Part{p}, data: map[common.Part][]byte{p: data}}
	downloadErr := errors.New("interrupted download")
	src.downloadPart = func(_ common.Part, w io.Writer) error {
		if _, err := w.Write(data[:3]); err != nil {
			return err
		}
		return downloadErr
	}
	r := &Restore{
		Src:                     src,
		Dst:                     &fslocal.FS{Dir: t.TempDir()},
		Concurrency:             2,
		SkipBackupCompleteCheck: true,
	}
	if err := r.Run(context.Background()); !errors.Is(err, downloadErr) {
		t.Fatalf("unexpected restore error: %v", err)
	}
	if _, err := os.Stat(p.LocalPath(r.Dst.Dir)); !os.IsNotExist(err) {
		t.Fatalf("failed download was finalized: %v", err)
	}
	lockPath := filepath.Join(r.Dst.Dir, backupnames.RestoreInProgressFilename)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("missing incomplete restore marker: %s", err)
	}
	src.downloadPart = nil
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("cannot resume failed restore: %s", err)
	}
	got, err := os.ReadFile(p.LocalPath(r.Dst.Dir))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("unexpected resumed file: %q; err=%v", got, err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("incomplete restore marker remains after success: %v", err)
	}
}

func TestRestoreDownloadCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src := &concurrencyTrackingRemoteFS{data: make(map[common.Part][]byte)}
	var parts []common.Part
	for i := range 3 {
		p := common.Part{Path: "data/file", FileSize: 12, Offset: uint64(i * 4), Size: 4, ActualSize: 4}
		parts = append(parts, p)
		src.data[p] = []byte("data")
	}
	src.beforeDownload = func(_ common.Part) error {
		cancel()
		return nil
	}
	r := &Restore{Src: src, Dst: &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: true}}
	var downloaded atomic.Uint64
	err := r.runParallelPerPathInternal(ctx, 1, map[string][]common.Part{parts[0].Path: parts}, &downloaded)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cancellation error: %v", err)
	}
	if len(src.downloaded) != 1 {
		t.Fatalf("downloads continued after cancellation: %d", len(src.downloaded))
	}
	if _, err := os.Stat(parts[0].LocalPath(r.Dst.Dir)); !os.IsNotExist(err) {
		t.Fatalf("canceled file was finalized: %v", err)
	}
}

type concurrencyTrackingRemoteFS struct {
	parts          []common.Part
	data           map[common.Part][]byte
	beforeDownload func(common.Part) error
	beforeRange    func(downloadTask) error
	downloadPart   func(common.Part, io.Writer) error
	downloaded     []common.Part
	ranges         []downloadTask

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
	return fs.DownloadPartRange(p, 0, p.Size, w)
}

func (fs *concurrencyTrackingRemoteFS) DownloadPartRange(p common.Part, offset, size uint64, w io.Writer) error {
	if err := p.ValidateRange(offset, size); err != nil {
		return err
	}
	data, ok := fs.data[p]
	if !ok {
		return fmt.Errorf("unknown backup object: %s", &p)
	}
	fs.mu.Lock()
	if fs.activePerPath == nil {
		fs.activePerPath = make(map[string]int)
	}
	fs.activeTotal++
	fs.activePerPath[p.Path]++
	fs.maxActiveTotal = max(fs.maxActiveTotal, fs.activeTotal)
	fs.maxActivePerPath = max(fs.maxActivePerPath, fs.activePerPath[p.Path])
	fs.downloaded = append(fs.downloaded, p)
	task := downloadTask{part: p, offset: offset, size: size}
	fs.ranges = append(fs.ranges, task)
	fs.mu.Unlock()
	defer func() {
		fs.mu.Lock()
		fs.activeTotal--
		fs.activePerPath[p.Path]--
		fs.mu.Unlock()
	}()
	if fs.beforeDownload != nil {
		if err := fs.beforeDownload(p); err != nil {
			return err
		}
	}
	if fs.beforeRange != nil {
		if err := fs.beforeRange(task); err != nil {
			return err
		}
	}
	if fs.downloadPart != nil {
		return fs.downloadPart(p, w)
	}
	_, err := w.Write(data[offset : offset+size])
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
