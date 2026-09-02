package httputil

import (
	"io"
	"net/http"
	"sync"
)

// SyncBodyTransport makes it safe to reuse a buffer behind a request body.
//
// http.Transport writes and closes the request body in a separate goroutine,
// possibly after RoundTrip returned. If a server responds before reading the
// full request body, RoundTrip may return while the body is still being written,
// causing a data race if the buffer is reused. See the note at http.RoundTripper.
//
// The buffer becomes safe to reuse once the response body is closed.
type SyncBodyTransport struct {
	base http.RoundTripper
}

// NewSyncBodyTransport wraps base into a transport which makes it safe to reuse
// the buffer behind a request body.
//
// See SyncBodyTransport for details.
func NewSyncBodyTransport(base http.RoundTripper) *SyncBodyTransport {
	return &SyncBodyTransport{
		base: base,
	}
}

func (t *SyncBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody *requestBody
	if req.Body != nil && req.Body != http.NoBody {
		reqBody = newRequestBody(req.Body)
		// A RoundTripper must not modify the original request.
		// The same approach is used in http.Transport:
		// https://github.com/golang/go/blob/d90cb4e010dbbf7f39b993db5e64e2904c74e4a4/src/net/http/transport.go#L804
		reqCopy := *req
		req = &reqCopy
		req.Body = reqBody
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		// http.RountTripper states that implementations must always close
		// the request body, including on errors. So wait here too.
		if reqBody != nil {
			reqBody.waitForClose()
		}
		return nil, err
	}

	if reqBody != nil {
		// Wait for the request body to be closed only when the response
		// body is closed. http.Transport force-closes the request
		// body after some timeout when the response body is closed,
		// even if the request body is still in use.
		resp.Body = waitRequestBodyOnClose{
			responseBody: resp.Body,
			requestBody:  reqBody,
		}
	}
	return resp, nil
}

type requestBody struct {
	r      io.Reader
	doneCh chan struct{}
	once   sync.Once
}

func newRequestBody(body io.Reader) *requestBody {
	b := &requestBody{
		r:      body,
		doneCh: make(chan struct{}),
	}
	return b
}

func (b *requestBody) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func (b *requestBody) Close() error {
	defer b.once.Do(func() {
		close(b.doneCh)
	})

	rc, ok := b.r.(io.ReadCloser)
	if ok {
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (b *requestBody) waitForClose() {
	<-b.doneCh
}

type waitRequestBodyOnClose struct {
	responseBody io.ReadCloser
	requestBody  *requestBody
}

func (r waitRequestBodyOnClose) Read(p []byte) (n int, err error) {
	return r.responseBody.Read(p)
}

func (r waitRequestBodyOnClose) Close() error {
	// Close the response body.
	err := r.responseBody.Close()
	// Wait until request body reading is finished.
	// http.Transport force-closes the request body after some timeout,
	// so we cannot get stuck here longer than this timeout.
	r.requestBody.waitForClose()
	return err
}
