package remotewrite

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

var (
	badRequestsCount    int
	normalRequestsCount int
)

func decodeWriteRequest(r io.Reader) (*prompb.WriteRequest, error) {
	compressed, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	_, err = snappy.Decode(nil, compressed)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func newTestRemoteWriteServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/write", func(w http.ResponseWriter, r *http.Request) {
		_, err := decodeWriteRequest(r.Body)
		if err != nil {
			badRequestsCount += 1
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		normalRequestsCount += 1
		w.WriteHeader(http.StatusNoContent)
	})
	return httptest.NewServer(mux)
}

func mustDeleteDir(path string) {
	if err := os.RemoveAll(path); err != nil {
		panic(fmt.Errorf("cannot remove dir %q: %w", path, err))
	}
}

func TestClientSendBlockHTTP(t *testing.T) {
	srv := newTestRemoteWriteServer()
	path := "test"
	defer mustDeleteDir(path)
	fq := persistentqueue.MustOpenFastQueue(path, "test", 100, 0, false)
	defer fq.MustClose()
	url := srv.URL + "/api/v1/write"
	c := newHTTPClient(1, url, url, fq, 1)
	c.init(1, 1, "")
	ok := c.sendBlockHTTP([]byte{})
	assert.Equal(t, true, ok)
	assert.Equal(t, 1, badRequestsCount)
	assert.Equal(t, 0, normalRequestsCount)          // no request sent to remote write server
	assert.Equal(t, uint64(0), c.retriesCount.Get()) // no retry
}
