package remoteread

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

const (
	defaultReadTimeout = 5 * time.Minute
	remoteReadPath     = "/api/v1/read"
)

// StreamCallback is a callback function for processing time series
type StreamCallback func(series *vm.TimeSeries) error

// Client is an HTTP client for reading
// time series via remote read protocol.
type Client struct {
	addr              string
	disablePathAppend bool
	c                 *http.Client
	user              string
	password          string
	useStream         bool
	headers           []keyValue
	matchers          []*prompb.LabelMatcher
}

// Config is config for remote read.
type Config struct {
	// Addr of remote storage
	Addr string
	// Transport allows specifying custom http.Transport
	Transport *http.Transport
	// DisablePathAppend disable automatic appending of the remote read path
	DisablePathAppend bool
	// Timeout defines timeout for HTTP requests
	// made by remote read client
	Timeout time.Duration
	// Username is the remote read username, optional.
	Username string
	// Password is the remote read password, optional.
	Password string
	// UseStream defines whether to use SAMPLES or STREAMED_XOR_CHUNKS mode
	// see https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/#samples
	// https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/#streamed-chunks
	UseStream bool
	// Headers optional HTTP headers to send with each request to the corresponding remote storage
	Headers string
	// LabelName, LabelValue stands for label=~value pair used for read requests.
	// Is optional.
	LabelName, LabelValue string
}

// Filter defines a list of filters applied to requested data
type Filter struct {
	StartTimestampMs int64
	EndTimestampMs   int64
}

// NewClient returns client for
// reading time series via remote read protocol.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("config.Addr can't be empty")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultReadTimeout
	}

	var hdrs []string
	if cfg.Headers != "" {
		hdrs = strings.Split(cfg.Headers, "^^")
	}

	headers, err := parseHeaders(hdrs)
	if err != nil {
		return nil, err
	}

	var m *prompb.LabelMatcher
	if cfg.LabelName != "" && cfg.LabelValue != "" {
		m = &prompb.LabelMatcher{
			Type:  prompb.LabelMatcher_RE,
			Name:  cfg.LabelName,
			Value: cfg.LabelValue,
		}
	}

	client := &http.Client{Timeout: cfg.Timeout}
	if cfg.Transport != nil {
		client.Transport = cfg.Transport
	}

	c := &Client{
		c:                 client,
		addr:              strings.TrimSuffix(cfg.Addr, "/"),
		disablePathAppend: cfg.DisablePathAppend,
		user:              cfg.Username,
		password:          cfg.Password,
		useStream:         cfg.UseStream,
		headers:           headers,
		matchers:          []*prompb.LabelMatcher{m},
	}

	return c, nil
}

// Read fetch data from remote read source
func (c *Client) Read(ctx context.Context, filter *Filter, streamCb StreamCallback) error {
	req := &prompb.ReadRequest{
		Queries: []*prompb.Query{
			{
				StartTimestampMs: filter.StartTimestampMs,
				EndTimestampMs:   filter.EndTimestampMs - 1,
				Matchers:         c.matchers,
			},
		},
	}
	if c.useStream {
		req.AcceptedResponseTypes = []prompb.ReadRequest_ResponseType{prompb.ReadRequest_STREAMED_XOR_CHUNKS}
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("unable to marshal read request: %w", err)
	}

	b := snappy.Encode(nil, data)
	if err := c.fetch(ctx, b, streamCb); err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("fetch request has ben cancelled")
		}
		return fmt.Errorf("error while fetching data from remote storage: %s", err)
	}
	return nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	for _, h := range c.headers {
		req.Header.Add(h.key, h.value)
	}
	return c.c.Do(req)
}

func (c *Client) fetch(ctx context.Context, data []byte, streamCb StreamCallback) error {
	r := bytes.NewReader(data)
	// by default, we are using a common remote read path
	u, err := url.JoinPath(c.addr, remoteReadPath)
	if err != nil {
		return fmt.Errorf("error create url from addr %s and default remote read path %s", c.addr, remoteReadPath)
	}
	// we should use full address from the remote-read-src-addr flag
	if c.disablePathAppend {
		u = c.addr
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, r)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Add("Accept-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	if c.useStream {
		req.Header.Set("Content-Type", "application/x-streamed-protobuf; proto=prometheus.ChunkedReadResponse")
	}
	req.Header.Set("X-Prometheus-Remote-Read-Version", "0.1.0")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("error while sending request to %s: %w; Data len %d(%d)",
			req.URL.Redacted(), err, len(data), r.Size())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL.Redacted(), body)
	}

	if c.useStream {
		return processStreamResponse(resp.Body, streamCb)
	}

	return processResponse(resp.Body, streamCb)
}

func processResponse(body io.ReadCloser, callback StreamCallback) error {
	d, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}
	uncompressed, err := snappy.Decode(nil, d)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	var readResp prompb.ReadResponse
	err = proto.Unmarshal(uncompressed, &readResp)
	if err != nil {
		return fmt.Errorf("unable to unmarshal response body: %w", err)
	}
	// response could have no results for the given filter, but that
	// shouldn't be accounted as an error.
	for _, res := range readResp.Results {
		for _, ts := range res.Timeseries {
			vmTs := convertSamples(ts.Samples, ts.Labels)
			if err := callback(vmTs); err != nil {
				return err
			}
		}
	}

	return nil
}

var bbPool bytesutil.ByteBufferPool

func processStreamResponse(body io.ReadCloser, callback StreamCallback) error {
	bb := bbPool.Get()
	defer func() { bbPool.Put(bb) }()

	stream := remote.NewChunkedReader(body, remote.DefaultChunkedReadLimit, bb.B)
	for {
		res := &prompb.ChunkedReadResponse{}
		err := stream.NextProto(res)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		for _, series := range res.ChunkedSeries {
			samples := make([]prompb.Sample, 0)
			for _, chunk := range series.Chunks {
				s, err := parseSamples(chunk.Data)
				if err != nil {
					return err
				}
				samples = append(samples, s...)
			}

			ts := convertSamples(samples, series.Labels)
			if err := callback(ts); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseSamples(chunk []byte) ([]prompb.Sample, error) {
	c, err := chunkenc.FromData(chunkenc.EncXOR, chunk)
	if err != nil {
		return nil, fmt.Errorf("error read chunk: %w", err)
	}

	var samples []prompb.Sample
	it := c.Iterator(nil)
	for {
		typ := it.Next()
		if typ == chunkenc.ValNone {
			break
		}
		if typ != chunkenc.ValFloat {
			// Skip unsupported values
			continue
		}
		if it.Err() != nil {
			return nil, fmt.Errorf("error iterate over chunks: %w", it.Err())
		}

		ts, v := it.At()
		s := prompb.Sample{
			Timestamp: ts,
			Value:     v,
		}
		samples = append(samples, s)
	}

	return samples, it.Err()
}

type keyValue struct {
	key   string
	value string
}

func parseHeaders(headers []string) ([]keyValue, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	kvs := make([]keyValue, len(headers))
	for i, h := range headers {
		n := strings.IndexByte(h, ':')
		if n < 0 {
			return nil, fmt.Errorf(`missing ':' in header %q; expecting "key: value" format`, h)
		}
		kv := &kvs[i]
		kv.key = strings.TrimSpace(h[:n])
		kv.value = strings.TrimSpace(h[n+1:])
	}
	return kvs, nil
}

func convertSamples(samples []prompb.Sample, labels []prompb.Label) *vm.TimeSeries {
	labelPairs := make([]vm.LabelPair, 0, len(labels))
	nameValue := ""
	for _, label := range labels {
		if label.Name == "__name__" {
			nameValue = label.Value
			continue
		}
		labelPairs = append(labelPairs, vm.LabelPair{Name: label.Name, Value: label.Value})
	}

	n := len(samples)
	values := make([]float64, 0, n)
	timestamps := make([]int64, 0, n)
	for _, sample := range samples {
		values = append(values, sample.Value)
		timestamps = append(timestamps, sample.Timestamp)
	}
	return &vm.TimeSeries{
		Name:       nameValue,
		LabelPairs: labelPairs,
		Timestamps: timestamps,
		Values:     values,
	}
}
