//go:build synctest

package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
)

func TestRestoreDownloadUnexpectedEOFRetries(t *testing.T) {
	otherErr := errors.New("non-retryable download failure")
	testCases := []struct {
		name              string
		skipPreallocation bool
		failures          int
		downloadErr       error
		cancelBackoff     bool
		wantCalls         int
		wantErr           error
	}{
		{"recover", false, 1, io.ErrUnexpectedEOF, false, 2, nil},
		{"last-attempt", false, 2, io.ErrUnexpectedEOF, false, 3, nil},
		{"exhausted", false, 3, io.ErrUnexpectedEOF, false, 3, io.ErrUnexpectedEOF},
		{"other-error", false, 1, otherErr, false, 1, otherErr},
		{"cancel-backoff", false, 3, io.ErrUnexpectedEOF, true, 1, context.Canceled},
		{"without-preallocation", true, 1, io.ErrUnexpectedEOF, false, 2, nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const prefix = "head"
				data := bytes.Repeat([]byte("data"), (9<<20)/4)
				p := common.Part{
					Path: "data/file", FileSize: uint64(len(prefix) + len(data)),
					Offset: uint64(len(prefix)), Size: uint64(len(data)), ActualSize: uint64(len(data)),
				}
				src := &concurrencyTrackingRemoteFS{data: map[common.Part][]byte{p: data}}
				calls := 0
				src.downloadPart = func(_ common.Part, w io.Writer) error {
					calls++
					if calls <= tc.failures {
						// Exceed the pipeline buffer so partial bytes reach disk before the error.
						if _, err := w.Write(data[:8<<20+1]); err != nil {
							return err
						}
						return fmt.Errorf("reading object body: %w", tc.downloadErr)
					}
					_, err := w.Write(data)
					return err
				}
				r := &Restore{
					Src:               src,
					Dst:               &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: !tc.skipPreallocation},
					SkipPreallocation: tc.skipPreallocation,
				}
				path := p.LocalPath(r.Dst.Dir)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(prefix), 0600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var progress atomic.Uint64
				progress.Store(37)
				run := func() error {
					return r.runParallelPerPathInternal(ctx, 2, map[string][]common.Part{p.Path: {p}}, &progress)
				}
				var err error
				if tc.cancelBackoff {
					done := make(chan error, 1)
					go func() { done <- run() }()
					synctest.Wait()
					cancel()
					err = <-done
				} else {
					err = run()
				}
				if !errors.Is(err, tc.wantErr) || calls != tc.wantCalls {
					t.Fatalf("unexpected result: err=%v, calls=%d; want err=%v, calls=%d", err, calls, tc.wantErr, tc.wantCalls)
				}
				if tc.wantErr != nil {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("failed or canceled part was finalized: %v", err)
					}
					if errors.Is(tc.wantErr, io.ErrUnexpectedEOF) && progress.Load() != 37 {
						t.Fatalf("failed EOF attempts remained in restore progress: got %d; want 37", progress.Load())
					}
					return
				}
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != len(prefix)+len(data) || string(got[:len(prefix)]) != prefix || !bytes.Equal(got[len(prefix):], data) {
					t.Fatal("retried bytes weren't overwritten at the original offset")
				}
				if progress.Load() != 37+p.Size {
					t.Fatalf("retry double-counted progress: got %d; want %d", progress.Load(), 37+p.Size)
				}
			})
		})
	}
}

func TestRestoreDownloadFinalizesAfterAllParts(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				src := &concurrencyTrackingRemoteFS{data: make(map[common.Part][]byte)}
				var parts []common.Part
				for i := range 3 {
					p := common.Part{Path: "data/file", FileSize: 12, Offset: uint64(i * 4), Size: 4, ActualSize: 4}
					parts = append(parts, p)
					src.data[p] = []byte("data")
				}
				releaseFirst := make(chan struct{})
				downloadErr := errors.New("first part failed")
				src.beforeDownload = func(p common.Part) error {
					if p.Offset == 0 {
						<-releaseFirst
						if fail {
							return downloadErr
						}
					}
					return nil
				}
				r := &Restore{
					Src: src,
					Dst: &fslocal.FS{Dir: t.TempDir(), UseTmpFiles: true},
				}
				var downloaded atomic.Uint64
				done := make(chan error, 1)
				go func() {
					done <- r.runParallelPerPathInternal(context.Background(), 3, map[string][]common.Part{parts[0].Path: parts}, &downloaded)
				}()
				synctest.Wait()
				if downloaded.Load() != 8 {
					t.Errorf("later parts didn't finish while the first was blocked: %d bytes", downloaded.Load())
				}
				path := parts[0].LocalPath(r.Dst.Dir)
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("file finalized before every part completed: %v", err)
				}
				close(releaseFirst)
				err := <-done
				if fail {
					if !errors.Is(err, downloadErr) {
						t.Fatalf("unexpected restore error: %v", err)
					}
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("file finalized despite a failed part: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "datadatadata" {
					t.Fatalf("restored contents don't match the backup; err=%v", err)
				}
			})
		})
	}
}
