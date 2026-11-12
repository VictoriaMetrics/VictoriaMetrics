package remoteread

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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
	// LabelNames, LabelValues stands for label=~value pair used for read requests.
	// Is optional.
	LabelNames, LabelValues []string
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

	var matchers []*prompb.LabelMatcher
	if len(cfg.LabelNames) > 0 || len(cfg.LabelValues) > 0 {
		if len(cfg.LabelNames) != len(cfg.LabelValues) {
			return nil, fmt.Errorf("the number of label names and label values must be the same")
		}

		for i := range cfg.LabelNames {
			if cfg.LabelNames[i] == "" {
				return nil, fmt.Errorf("label name cannot be empty")
			}
			matcher := &prompb.LabelMatcher{
				Type:  prompb.LabelMatcher_RE,
				Name:  cfg.LabelNames[i],
				Value: cfg.LabelValues[i],
			}
			matchers = append(matchers, matcher)
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
		matchers:          matchers,
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
		return fmt.Errorf("error while fetching data from remote storage: %w", err)
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
			// A series contains either float samples or native histogram samples.
			// Both fields are processed independently, since a series may switch
			// from float to native histogram representation at some point in time,
			// so the requested time range may contain samples of both types.
			if len(ts.Samples) > 0 {
				vmTs := convertSamples(ts.Samples, ts.Labels)
				if err := callback(vmTs); err != nil {
					return err
				}
			}
			if len(ts.Histograms) > 0 {
				hSamples := make([]histogramSample, 0, len(ts.Histograms))
				for _, h := range ts.Histograms {
					hSamples = append(hSamples, histogramSample{
						timestamp: h.Timestamp,
						fh:        h.ToFloatHistogram(),
					})
				}
				for _, vmTs := range convertHistograms(hSamples, ts.Labels) {
					if err := callback(vmTs); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

var bbPool bytesutil.ByteBufferPool

func processStreamResponse(body io.ReadCloser, callback StreamCallback) error {
	bb := bbPool.Get()
	defer func() { bbPool.Put(bb) }()

	stream := remote.NewChunkedReader(body, config.DefaultChunkedReadLimit, bb.B)
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
			var hSamples []histogramSample
			for _, chunk := range series.Chunks {
				switch chunk.Type {
				case prompb.Chunk_XOR, prompb.Chunk_UNKNOWN:
					// In proto3 the `type` field may be left unset (UNKNOWN) for XOR chunks.
					// Prometheus remote.proto: "REQUIREMENT: when using proto3, this field
					// MUST be set when using anything else than XOR". Senders before native
					// histograms support (Prometheus < 2.40) do not set this field at all,
					// so UNKNOWN chunks must be parsed as XOR ones.
					s, err := parseSamples(chunk.Data)
					if err != nil {
						return err
					}
					samples = append(samples, s...)
				case prompb.Chunk_HISTOGRAM, prompb.Chunk_FLOAT_HISTOGRAM:
					hs, err := parseHistograms(chunk.Type, chunk.Data)
					if err != nil {
						return err
					}
					hSamples = append(hSamples, hs...)
				default:
					return fmt.Errorf("unsupported chunk encoding %q", chunk.Type)
				}
			}

			// A series contains either XOR chunks or native histogram chunks.
			// Both are processed independently, since a series may switch
			// from float to native histogram representation at some point in time,
			// so the requested time range may contain chunks of both types.
			if len(samples) > 0 {
				ts := convertSamples(samples, series.Labels)
				if err := callback(ts); err != nil {
					return err
				}
			}
			for _, ts := range convertHistograms(hSamples, series.Labels) {
				if err := callback(ts); err != nil {
					return err
				}
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

// histogramSample represents a single native histogram sample.
type histogramSample struct {
	timestamp int64
	fh        *histogram.FloatHistogram
}

func parseHistograms(encoding prompb.Chunk_Encoding, chunk []byte) ([]histogramSample, error) {
	var enc chunkenc.Encoding
	switch encoding {
	case prompb.Chunk_HISTOGRAM:
		enc = chunkenc.EncHistogram
	case prompb.Chunk_FLOAT_HISTOGRAM:
		enc = chunkenc.EncFloatHistogram
	default:
		return nil, fmt.Errorf("unsupported histogram chunk encoding %q", encoding)
	}
	c, err := chunkenc.FromData(enc, chunk)
	if err != nil {
		return nil, fmt.Errorf("error read chunk: %w", err)
	}

	var hSamples []histogramSample
	it := c.Iterator(nil)
	for {
		typ := it.Next()
		if typ == chunkenc.ValNone {
			break
		}
		switch typ {
		case chunkenc.ValHistogram:
			ts, h := it.AtHistogram(nil)
			hSamples = append(hSamples, histogramSample{
				timestamp: ts,
				fh:        h.ToFloat(nil),
			})
		case chunkenc.ValFloatHistogram:
			ts, fh := it.AtFloatHistogram(nil)
			hSamples = append(hSamples, histogramSample{
				timestamp: ts,
				fh:        fh,
			})
		default:
			// Skip unsupported values
			continue
		}
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("error iterate over chunks: %w", err)
	}

	return hSamples, nil
}

// convertHistograms converts native histogram samples into VictoriaMetrics histogram
// time series in the same way as VictoriaMetrics converts native histograms
// received via Prometheus remote write protocol: every native histogram sample
// is converted into `<name>_count` and `<name>_sum` series plus a set of
// `<name>_bucket` series with `vmrange` labels containing non-cumulative bucket counts.
// The only difference is that for native histograms with custom buckets (NHCB)
// bucket bounds are taken from the custom values, while the remote write protocol
// parser ignores custom values and estimates the bounds with the exponential formula.
// See https://prometheus.io/docs/specs/native_histograms/#data-model
func convertHistograms(hSamples []histogramSample, labels []prompb.Label) []*vm.TimeSeries {
	if len(hSamples) == 0 {
		return nil
	}

	labelPairs := make([]vm.LabelPair, 0, len(labels))
	nameValue := ""
	for _, label := range labels {
		if label.Name == "__name__" {
			nameValue = label.Value
			continue
		}
		labelPairs = append(labelPairs, vm.LabelPair{Name: label.Name, Value: label.Value})
	}
	// the metric has no name, skip it in the same way as VictoriaMetrics does
	// when it receives a native histogram without the metric name via remote write protocol.
	if nameValue == "" {
		return nil
	}

	countSeries := &vm.TimeSeries{
		Name:       nameValue + "_count",
		LabelPairs: labelPairs,
	}
	sumSeries := &vm.TimeSeries{
		Name:       nameValue + "_sum",
		LabelPairs: labelPairs,
	}
	bucketSeries := make(map[string]*vm.TimeSeries)
	// vmranges preserves the order of bucketSeries creation
	// in order to get deterministic results.
	var vmranges []string

	for _, hs := range hSamples {
		fh := hs.fh
		countSeries.Timestamps = append(countSeries.Timestamps, hs.timestamp)
		countSeries.Values = append(countSeries.Values, fh.Count)
		sumSeries.Timestamps = append(sumSeries.Timestamps, hs.timestamp)
		sumSeries.Values = append(sumSeries.Values, fh.Sum)

		it := fh.AllBucketIterator()
		for it.Next() {
			b := it.At()
			if b.Count <= 0 {
				continue
			}
			vmrange := formatVmrange(b.Lower, b.Upper)
			s := bucketSeries[vmrange]
			if s == nil {
				bucketLabelPairs := make([]vm.LabelPair, len(labelPairs), len(labelPairs)+1)
				copy(bucketLabelPairs, labelPairs)
				bucketLabelPairs = append(bucketLabelPairs, vm.LabelPair{Name: "vmrange", Value: vmrange})
				s = &vm.TimeSeries{
					Name:       nameValue + "_bucket",
					LabelPairs: bucketLabelPairs,
				}
				bucketSeries[vmrange] = s
				vmranges = append(vmranges, vmrange)
			}
			s.Timestamps = append(s.Timestamps, hs.timestamp)
			s.Values = append(s.Values, b.Count)
		}
	}

	tss := make([]*vm.TimeSeries, 0, 2+len(vmranges))
	tss = append(tss, countSeries, sumSeries)
	for _, vmrange := range vmranges {
		tss = append(tss, bucketSeries[vmrange])
	}
	return tss
}

// formatVmrange formats the given bucket bounds into `vmrange` label value
// in the same way as VictoriaMetrics does for native histograms
// received via Prometheus remote write protocol.
func formatVmrange(lower, upper float64) string {
	b := make([]byte, 0, 24)
	b = strconv.AppendFloat(b, lower, 'e', 3, 64)
	b = append(b, "..."...)
	b = strconv.AppendFloat(b, upper, 'e', 3, 64)
	return string(b)
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
