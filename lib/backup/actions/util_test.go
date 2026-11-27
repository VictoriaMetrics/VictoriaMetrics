package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
)

func TestRestoreDownloadConcurrency(t *testing.T) {
	testCases := []struct {
		name              string
		concurrency       int
		files             int
		skipPreallocation bool
		wantActive        int
		wantPerFile       int
	}{
		{"same-file", 4, 1, false, 4, 4},
		{"multiple-files", 6, 2, false, 6, 4},
		{"global-limit", 3, 2, false, 3, 3},
		{"sequential-resume", 4, 2, true, 2, 1},
		{"zero-limit", 0, 1, false, 1, 1},
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
			r := &Restore{
				Src:               src,
				Dst:               &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: !tc.skipPreallocation},
				SkipPreallocation: tc.skipPreallocation,
			}
			t.Cleanup(unblock)
			var downloaded atomic.Uint64
			done := make(chan error, 1)
			go func() {
				done <- r.runParallelPerPath(context.Background(), tc.concurrency, perPath, &downloaded, func(time.Duration) {})
			}()
			initial := make(map[string]int)
			for range tc.wantActive {
				select {
				case p := <-started:
					initial[p.Path]++
				case err := <-done:
					t.Fatalf("restore finished before reaching expected concurrency: %v", err)
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for concurrent downloads")
				}
			}
			if !tc.skipPreallocation {
				minActive, maxActive := tc.wantActive, 0
				for _, n := range initial {
					minActive, maxActive = min(minActive, n), max(maxActive, n)
				}
				if len(initial) != min(tc.files, tc.wantActive) || maxActive-minActive > 1 {
					t.Errorf("initial downloads weren't spread across files: %v", initial)
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
			if src.maxActiveTotal != tc.wantActive || src.maxActivePerPath > tc.wantPerFile {
				t.Fatalf("unexpected maximum concurrency: total=%d, per file=%d; want total=%d, per file<=%d",
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
			if tc.skipPreallocation {
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

func TestRestoreDownloadInterleavesFiles(t *testing.T) {
	src := &concurrencyTrackingRemoteFS{data: make(map[common.Part][]byte)}
	perPath := make(map[string][]common.Part)
	partCounts := make(map[string]int)
	for i, n := range []int{2, 4, 3} {
		path := fmt.Sprintf("round-robin/file%d", i)
		partCounts[path] = n
		for j := n - 1; j >= 0; j-- {
			p := common.Part{Path: path, FileSize: uint64(n * 4), Offset: uint64(j * 4), Size: 4, ActualSize: 4}
			src.data[p] = bytes.Repeat([]byte{byte('a' + j)}, 4)
			perPath[path] = append(perPath[path], p)
		}
	}
	r := &Restore{Src: src, Dst: &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: true}}
	var downloaded atomic.Uint64
	if err := r.runParallelPerPathInternal(context.Background(), 1, perPath, &downloaded); err != nil {
		t.Fatal(err)
	}
	next := 0
	for partIndex := 0; partIndex < 4; partIndex++ {
		seen := make(map[string]bool)
		for _, n := range partCounts {
			if n <= partIndex {
				continue
			}
			if next >= len(src.downloaded) {
				t.Fatal("missing part download")
			}
			p := src.downloaded[next]
			next++
			if p.Offset != uint64(partIndex*4) || seen[p.Path] || partCounts[p.Path] <= partIndex {
				t.Fatalf("part wasn't queued in its file round: %s", &p)
			}
			seen[p.Path] = true
		}
	}
	if next != len(src.downloaded) || downloaded.Load() != 36 {
		t.Fatalf("unexpected downloads: parts=%d, bytes=%d", len(src.downloaded), downloaded.Load())
	}
	for path, n := range partCounts {
		data, err := os.ReadFile(filepath.Join(r.Dst.Dir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("aaaabbbbccccdddd")[:n*4]
		if !bytes.Equal(data, want) {
			t.Fatalf("unexpected restored contents for %q: %q", path, data)
		}
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
	downloadPart   func(common.Part, io.Writer) error
	downloaded     []common.Part

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
	if fs.downloadPart != nil {
		return fs.downloadPart(p, w)
	}
	_, err := w.Write(data)
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
