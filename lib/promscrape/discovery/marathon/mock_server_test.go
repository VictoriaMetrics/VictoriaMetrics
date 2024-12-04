package marathon

import (
	"net/http"
	"net/http/httptest"
)

type mockMarathonServer struct {
	*httptest.Server
	responseFunc func() []byte
}

func newMockMarathonServer(responseFunc func() []byte) *mockMarathonServer {
	var s mockMarathonServer
	s.responseFunc = responseFunc
	s.Server = httptest.NewServer(http.HandlerFunc(s.handler))
	return &s
}

func (s *mockMarathonServer) handler(w http.ResponseWriter, _ *http.Request) {
	data := s.responseFunc()
	w.Write(data)
}
