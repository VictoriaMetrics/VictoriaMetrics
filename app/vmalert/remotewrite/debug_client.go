package remotewrite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// DebugClient won't push series periodically, but will write data to remote endpoint
// immediately when Push() is called
type DebugClient struct {
	addr string
	c    *http.Client

	wg sync.WaitGroup
}

// NewDebugClient returns a debug remotewrite client
func NewDebugClient() (*DebugClient, error) {
	if *addr == "" {
		return nil, nil
	}

	t, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	c := &DebugClient{
		c: &http.Client{
			Timeout:   *sendTimeout,
			Transport: t,
		},
		addr: strings.TrimSuffix(*addr, "/"),
	}
	return c, nil
}

// Push adds timeseries into queue for writing into remote storage.
// Push returns and error if client is stopped or if queue is full.
func (c *DebugClient) Push(s prompbmarshal.TimeSeries) error {
	c.wg.Add(1)
	defer c.wg.Done()
	wr := &prompbmarshal.WriteRequest{Timeseries: []prompbmarshal.TimeSeries{s}}
	data, err := wr.Marshal()
	if err != nil {
		return err
	}
	b := snappy.Encode(nil, data)
	return c.send(context.Background(), b)
}

// Close stops the client
func (c *DebugClient) Close() error {
	c.wg.Wait()
	return nil
}

func (c *DebugClient) send(ctx context.Context, data []byte) error {
	r := bytes.NewReader(data)
	req, err := http.NewRequest(http.MethodPost, c.addr, r)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// RFC standard compliant headers
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")

	// Prometheus compliant headers
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	if !*disablePathAppend {
		req.URL.Path = path.Join(req.URL.Path, "/api/v1/write")
	}
	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error while sending request to %s: %w; Data len %d(%d)",
			req.URL.Redacted(), err, len(data), r.Size())
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	// according to https://prometheus.io/docs/concepts/remote_write_spec/
	// Prometheus remote Write compatible receivers MUST
	switch resp.StatusCode / 100 {
	case 2:
		// respond with a HTTP 2xx status code when the write is successful.
		return nil
	case 4:
		if resp.StatusCode != http.StatusTooManyRequests {
			// MUST NOT retry write requests on HTTP 4xx responses other than 429
			return &nonRetriableError{fmt.Errorf("unexpected response code %d for %s. Response body %q",
				resp.StatusCode, req.URL.Redacted(), body)}
		}
		fallthrough
	default:
		return fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL.Redacted(), body)
	}
}
