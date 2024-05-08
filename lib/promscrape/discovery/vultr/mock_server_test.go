package vultr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func newMockVultrServer(jsonResponse func() ([]byte, error)) *vultrServer {
	rw := &vultrServer{}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	rw.jsonResponse = jsonResponse
	return rw
}

type vultrServer struct {
	*httptest.Server
	jsonResponse func() ([]byte, error)
}

func (rw *vultrServer) err(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func (rw *vultrServer) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		rw.err(w, fmt.Errorf("bad method %q", r.Method))
		return
	}

	resp, err := rw.jsonResponse()
	if err != nil {
		rw.err(w, err)
		return
	}

	w.Write(resp)
	w.WriteHeader(http.StatusOK)
}
