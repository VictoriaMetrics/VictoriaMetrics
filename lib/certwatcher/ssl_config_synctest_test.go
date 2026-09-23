//go:build synctest

package certwatcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func mustWriteCertFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write file %q: %s", path, err)
	}
}

func TestGetConfigHashCached(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		certPath := filepath.Join(t.TempDir(), "cert.pem")
		mustWriteCertFile(t, certPath, "cert-v1")

		sc := &SSLConfig{CertFilePath: certPath}
		h1 := sc.getConfigHash()
		if h1 == 0 {
			t.Fatalf("getConfigHash() must return a non-zero hash for existing file content")
		}
		mustWriteCertFile(t, certPath, "cert-v2")
		// getConfigHash must return stale result
		if h2 := sc.getConfigHash(); h2 != h1 {
			t.Fatalf("getConfigHash() must return the cached hash %d within deadline interval; got %d", h1, h2)
		}

		// Advance past the deadline window and verify the new content is picked up.
		time.Sleep(6 * time.Second)
		if h3 := sc.getConfigHash(); h3 == h1 {
			t.Fatalf("getConfigHash() must return an updated after deadlinehas")
		}
	})
}

func TestCertWatcherRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		certPath := filepath.Join(t.TempDir(), "cert.pem")
		mustWriteCertFile(t, certPath, "cert-v1")

		sc := &SSLConfig{CertFilePath: certPath}
		stop := make(chan struct{})
		var updates int
		var wg sync.WaitGroup

		wg.Go(func() {
			Run(sc, stop, func() error {
				updates++
				return nil
			})
		})

		synctest.Wait()

		mustWriteCertFile(t, certPath, "cert-v2")

		// The ticker fires every 10s-11s (10s + up to 1s jitter)
		time.Sleep(22 * time.Second)
		synctest.Wait()

		if updates != 1 {
			t.Fatalf("unexpected number of onUpdate calls after cert change; got %d; want 1", updates)
		}

		// A further tick with no additional changes must not call onUpdate again.
		time.Sleep(contentCheckInterval + 2*time.Second)
		synctest.Wait()
		if updates != 1 {
			t.Fatalf("onUpdate must not be called again when cert content is unchanged; got %d calls", updates)
		}

		close(stop)
		wg.Wait()
		synctest.Wait()
	})
}
