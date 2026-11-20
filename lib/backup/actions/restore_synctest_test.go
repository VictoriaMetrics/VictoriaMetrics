//go:build synctest

package actions

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
)

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
