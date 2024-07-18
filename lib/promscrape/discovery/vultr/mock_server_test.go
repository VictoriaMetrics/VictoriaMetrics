package vultr

import (
	"net/http"
	"net/http/httptest"
)

type mockVultrServer struct {
	*httptest.Server
	responseFunc func() []byte
}

func newMockVultrServer(responseFunc func() []byte) *mockVultrServer {
	var s mockVultrServer
	s.responseFunc = responseFunc
	s.Server = httptest.NewServer(http.HandlerFunc(s.handler))
	return &s
}

func (s *mockVultrServer) handler(w http.ResponseWriter, _ *http.Request) {
	data := s.responseFunc()
	w.Write(data)
}
