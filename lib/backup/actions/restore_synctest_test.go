//go:build synctest

package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
)

func TestRestoreDownloadFinalizesAfterAllParts(t *testing.T) {
	for _, ranged := range []bool{false, true} {
		t.Run(fmt.Sprintf("ranged=%v", ranged), func(t *testing.T) {
			testRestoreFinalizesAfterAllParts(t, ranged)
		})
	}
}

func testRestoreFinalizesAfterAllParts(t *testing.T, ranged bool) {
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
				wantData := []byte("datadatadata")
				wantDownloaded := uint64(8)
				if ranged {
					wantData = bytes.Repeat([]byte("data"), (192<<20)/4)
					p := common.Part{Path: "data/file", FileSize: uint64(len(wantData)), Size: uint64(len(wantData)), ActualSize: uint64(len(wantData))}
					parts = []common.Part{p}
					src.data = map[common.Part][]byte{p: wantData}
					wantDownloaded = 128 << 20
				}
				releaseFirst := make(chan struct{})
				downloadErr := errors.New("first part failed")
				src.beforeRange = func(task downloadTask) error {
					if task.part.Offset+task.offset == 0 {
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
				if downloaded.Load() != wantDownloaded {
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
				if err != nil || !bytes.Equal(data, wantData) {
					t.Fatalf("restored contents don't match the backup; err=%v", err)
				}
			})
		})
	}
}
