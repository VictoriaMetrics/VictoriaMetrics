package native

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/auth"
)

const (
	nativeTenantsAddr     = "admin/tenants"
	nativeMetricNamesAddr = "api/v1/label/__name__/values"
)

// Client is an HTTP client for exporting and importing
// time series via native protocol.
type Client struct {
	AuthCfg     *auth.Config
	Addr        string
	ExtraLabels []string
	HTTPClient  *http.Client
}

// LabelValues represents series from api/v1/series response
type LabelValues map[string]string

// Response represents response from api/v1/label/__name__/values
type Response struct {
	Status      string   `json:"status"`
	MetricNames []string `json:"data"`
}

// Explore finds metric names by provided filter from api/v1/label/__name__/values
func (c *Client) Explore(ctx context.Context, f Filter, tenantID string, start, end time.Time) ([]string, error) {
	url := fmt.Sprintf("%s/%s", c.Addr, nativeMetricNamesAddr)
	if tenantID != "" {
		url = fmt.Sprintf("%s/select/%s/prometheus/%s", c.Addr, tenantID, nativeMetricNamesAddr)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", url, err)
	}

	params := req.URL.Query()
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", end.Format(time.RFC3339))
	params.Set("match[]", f.Match)
	req.URL.RawQuery = params.Encode()

	resp, err := c.do(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("series request failed: %s", err)
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("cannot decode series response: %s", err)
	}
	return response.MetricNames, resp.Body.Close()
}

// ImportPipe uses pipe reader in request to process data
func (c *Client) ImportPipe(ctx context.Context, dstURL string, pr *io.PipeReader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dstURL, pr)
	if err != nil {
		return fmt.Errorf("cannot create import request to %q: %s", c.Addr, err)
	}

	importResp, err := c.do(req, http.StatusNoContent)
	if err != nil {
		return fmt.Errorf("import request failed: %s", err)
	}
	if err := importResp.Body.Close(); err != nil {
		return fmt.Errorf("cannot close import response body: %s", err)
	}
	return nil
}

// ExportPipe makes request by provided filter and return io.ReadCloser which can be used to get data
func (c *Client) ExportPipe(ctx context.Context, url string, f Filter) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", c.Addr, err)
	}

	params := req.URL.Query()
	params.Set("match[]", f.Match)
	if f.TimeStart != "" {
		params.Set("start", f.TimeStart)
	}
	if f.TimeEnd != "" {
		params.Set("end", f.TimeEnd)
	}
	req.URL.RawQuery = params.Encode()

	// disable compression since it is meaningless for native format
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := c.do(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("export request failed: %w", err)
	}
	return resp.Body, nil
}

// GetSourceTenants discovers tenants by provided filter
func (c *Client) GetSourceTenants(ctx context.Context, f Filter) ([]string, error) {
	u := fmt.Sprintf("%s/%s", c.Addr, nativeTenantsAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", u, err)
	}

	params := req.URL.Query()
	if f.TimeStart != "" {
		params.Set("start", f.TimeStart)
	}
	if f.TimeEnd != "" {
		params.Set("end", f.TimeEnd)
	}
	req.URL.RawQuery = params.Encode()

	resp, err := c.do(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("tenants request failed: %s", err)
	}

	var r struct {
		Tenants []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("cannot decode tenants response: %s", err)
	}

	if err := resp.Body.Close(); err != nil {
		return nil, fmt.Errorf("cannot close tenants response body: %s", err)
	}

	return r.Tenants, nil
}

func (c *Client) do(req *http.Request, expSC int) (*http.Response, error) {
	if c.AuthCfg != nil {
		c.AuthCfg.SetHeaders(req, true)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unexpected error when performing request: %w", err)
	}

	if resp.StatusCode != expSC {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body for status code %d: %s", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected response code %d: %s", resp.StatusCode, string(body))
	}
	return resp, err
}
