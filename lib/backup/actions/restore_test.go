package actions

import (
	"bytes"
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
