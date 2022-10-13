package remoteread

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/prompb"
)

const defaultWriteTimeout = 30 * time.Second

// Client is an HTTP client for reading
// timeseries via remote read protocol.
type Client struct {
	addr    string
	c       *http.Client
	authCfg *promauth.Config
}

// Config is config for remote read.
type Config struct {
	// Addr of remote storage
	Addr    string
	AuthCfg *promauth.Config
	// WriteTimeout defines timeout for HTTP write request
	// to remote storage
	WriteTimeout time.Duration
	// Transport will be used by the underlying http.Client
	Transport *http.Transport
}

// Filter is used for request remote read data by filter
type Filter struct {
	Min, Max   int64
	Label      string
	LabelValue string
}

func (f Filter) inRange(min, max int64) bool {
	fmin, fmax := f.Min, f.Max
	if min == 0 {
		fmin = min
	}
	if fmax == 0 {
		fmax = max
	}
	return min <= fmax && fmin <= max
}

// NewClient returns asynchronous client for
// reading timeseries via remote write protocol.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("config.Addr can't be empty")
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.Transport == nil {
		cfg.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}

	c := &Client{
		c: &http.Client{
			Timeout:   cfg.WriteTimeout,
			Transport: cfg.Transport,
		},
		addr:    strings.TrimSuffix(cfg.Addr, "/"),
		authCfg: cfg.AuthCfg,
	}
	return c, nil
}

// Read fetch data from remote write source
func (c *Client) Read(ctx context.Context, filter *Filter) ([]*prompb.TimeSeries, error) {
	query, err := c.query(filter)
	if err != nil {
		return nil, err
	}
	req := &prompb.ReadRequest{
		Queries: []*prompb.Query{
			query,
		},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal read request: %w", err)
	}

	const attempts = 5
	b := snappy.Encode(nil, data)
	for i := 0; i < attempts; i++ {
		qr, err := c.fetch(ctx, b)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, fmt.Errorf("process stoped")
			}
			logger.Errorf("attempt %d to fetch data from remote storage: %s", i+1, err)
			// sleeping to avoid remote db hammering
			time.Sleep(time.Second)
			continue
		}
		return qr.Timeseries, nil
	}
	return nil, nil
}

func (c *Client) fetch(ctx context.Context, data []byte) (*prompb.QueryResult, error) {
	r := bytes.NewReader(data)
	req, err := http.NewRequest("POST", c.addr, r)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Add("Accept-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Read-Version", "0.1.0")

	if c.authCfg != nil {
		c.authCfg.SetHeaders(req, true)
	}

	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error while sending request to %s: %w; Data len %d(%d)",
			req.URL.Redacted(), err, len(data), r.Size())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL.Redacted(), body)
	}
	defer func() { _ = resp.Body.Close() }()

	d, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	uncompressed, err := snappy.Decode(nil, d)
	if err != nil {
		return nil, fmt.Errorf("error decode response: %w", err)
	}

	var readResp prompb.ReadResponse
	err = proto.Unmarshal(uncompressed, &readResp)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	return readResp.Results[0], nil
}

func (c *Client) query(filter *Filter) (*prompb.Query, error) {
	var ms *labels.Matcher
	if filter.Label == "" && filter.LabelValue == "" {
		ms = labels.MustNewMatcher(labels.MatchRegexp, labels.MetricName, ".+")
	} else {
		ms = labels.MustNewMatcher(labels.MatchRegexp, filter.Label, filter.LabelValue)
	}
	m, err := toLabelMatchers(ms)
	if err != nil {
		return nil, err
	}
	return &prompb.Query{
		StartTimestampMs: filter.Min,
		EndTimestampMs:   filter.Max - 1,
		Matchers:         []*prompb.LabelMatcher{m},
	}, nil
}

func toLabelMatchers(matcher *labels.Matcher) (*prompb.LabelMatcher, error) {
	var mType prompb.LabelMatcher_Type
	switch matcher.Type {
	case labels.MatchEqual:
		mType = prompb.LabelMatcher_EQ
	case labels.MatchNotEqual:
		mType = prompb.LabelMatcher_NEQ
	case labels.MatchRegexp:
		mType = prompb.LabelMatcher_RE
	case labels.MatchNotRegexp:
		mType = prompb.LabelMatcher_NRE
	default:
		return nil, fmt.Errorf("invalid matcher type")
	}
	return &prompb.LabelMatcher{
		Type:  mType,
		Name:  matcher.Name,
		Value: matcher.Value,
	}, nil
}
