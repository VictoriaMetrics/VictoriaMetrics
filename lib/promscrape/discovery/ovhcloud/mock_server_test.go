package ovhcloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func newMockOVHCloudServer(jsonResponse func(path string) ([]byte, error)) *ovhcloudServer {
	rw := &ovhcloudServer{}
	rw.Server = httptest.NewServer(http.HandlerFunc(rw.handler))
	rw.jsonResponse = jsonResponse
	return rw
}

type ovhcloudServer struct {
	*httptest.Server
	jsonResponse func(path string) ([]byte, error)
}

func (rw *ovhcloudServer) err(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func (rw *ovhcloudServer) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		rw.err(w, fmt.Errorf("bad method %q", r.Method))
		return
	}

	resp, err := rw.jsonResponse(r.RequestURI)
	if err != nil {
		rw.err(w, err)
		return
	}

	w.Write(resp)
	w.WriteHeader(http.StatusOK)
}
