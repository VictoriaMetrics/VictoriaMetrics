package puppetdb

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func newMockPuppetDBServer(jsonResponse func(path string) ([]byte, error)) *puppetdbServer {
	rw := &puppetdbServer{}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	rw.jsonResponse = jsonResponse
	return rw
}

type puppetdbServer struct {
	*httptest.Server
	jsonResponse func(path string) ([]byte, error)
}

func (rw *puppetdbServer) err(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func (rw *puppetdbServer) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.err(w, fmt.Errorf("bad method %q", r.Method))
		return
	}

	resp, err := rw.jsonResponse(r.RequestURI)
	if err != nil {
		rw.err(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(resp)
	w.WriteHeader(http.StatusOK)
}
