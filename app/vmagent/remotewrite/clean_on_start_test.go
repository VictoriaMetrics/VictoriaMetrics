package remotewrite

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
)

func TestRemoteWriteCleanOnStartDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("one-shot cleanup is rejected on Windows")
	}
	// Opaque queue blocks are enough to verify routing and ordering; the
	// synthetic receiver deliberately doesn't decode the remote-write body.
	received := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("cannot read remote-write body: %s", err)
		}
		received <- r.URL.Path + ":" + string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	oldPath, oldURLs, oldQueues := *tmpDataPath, *remoteWriteURLs, *queues
	*tmpDataPath = t.TempDir()
	*remoteWriteURLs = []string{srv.URL + "/one?new=value#fragment", srv.URL + "/two"}
	*queues = flagutil.ArrayInt{}
	defer func() {
		*tmpDataPath, *remoteWriteURLs, *queues = oldPath, oldURLs, oldQueues
	}()
	if err := queues.Set("1,1"); err != nil {
		t.Fatal(err)
	}

	urls := make([]*url.URL, 2)
	paths := make([]string, 2)
	for i, raw := range *remoteWriteURLs {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		urls[i] = u
		pqURL := *u
		pqURL.RawQuery, pqURL.Fragment = "", ""
		paths[i] = filepath.Join(*tmpDataPath, persistentQueueDirname, fmt.Sprintf("%d_%016X", i+1, xxhash.Sum64String(pqURL.String())))
		fq := persistentqueue.MustOpenFastQueue(paths[i], fmt.Sprintf("destination-%d", i), 0, 0, false)
		if !fq.TryWriteBlock([]byte("old")) {
			t.Fatal("cannot seed queue")
		}
		fq.MustClose()
	}
	marker := filepath.Join(paths[0], "clean_on_start")
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	rw1 := newRemoteWriteCtx(0, urls[0], "destination-0")
	defer rw1.MustStop()
	rw2 := newRemoteWriteCtx(1, urls[1], "destination-1")
	defer rw2.MustStop()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("marker wasn't consumed before client startup: %v", err)
	}
	if !rw1.fq.TryWriteBlock([]byte("fresh")) {
		t.Fatal("cannot send fresh data after cleanup")
	}
	want := map[string]bool{"/one:fresh": true, "/two:old": true}
	for range 2 {
		select {
		case got := <-received:
			if !want[got] {
				t.Fatalf("unexpected remote-write block: %q", got)
			}
			delete(want, got)
		case <-time.After(5 * time.Second):
			t.Fatalf("remote-write delivery timed out; missing %v", want)
		}
	}
}
