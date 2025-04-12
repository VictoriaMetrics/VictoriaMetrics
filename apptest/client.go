package apptest

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
)

// Client is used for interacting with the apps over the network.
type Client struct {
	httpCli *http.Client
}

// NewClient creates a new client.
func NewClient() *Client {
	return &Client{
		httpCli: &http.Client{
			Transport: httputil.NewTransport(false, "apptest_client"),
		},
	}
}

// CloseConnections closes client connections.
func (c *Client) CloseConnections() {
	c.httpCli.CloseIdleConnections()
}

// Get sends a HTTP GET request, returns
// the response body and status code to the caller.
func (c *Client) Get(t *testing.T, url string) (string, int) {
	t.Helper()
	return c.do(t, http.MethodGet, url, "", nil)
}

// Post sends a HTTP POST request, returns
// the response body and status code to the caller.
func (c *Client) Post(t *testing.T, url, contentType string, data []byte) (string, int) {
	t.Helper()
	return c.do(t, http.MethodPost, url, contentType, data)
}

// PostForm sends a HTTP POST request containing the POST-form data, returns
// the response body and status code to the caller.
func (c *Client) PostForm(t *testing.T, url string, data url.Values) (string, int) {
	t.Helper()
	return c.Post(t, url, "application/x-www-form-urlencoded", []byte(data.Encode()))
}

// Delete sends a HTTP DELETE request and returns the response body and status code
// to the caller.
func (c *Client) Delete(t *testing.T, url string) (string, int) {
	t.Helper()
	return c.do(t, http.MethodDelete, url, "", nil)
}

// do prepares a HTTP request, sends it to the server, receives the response
// from the server, returns the response body and status code to the caller.
func (c *Client) do(t *testing.T, method, url, contentType string, data []byte) (string, int) {
	t.Helper()

	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("could not create a HTTP request: %v", err)
	}

	if len(contentType) > 0 {
		req.Header.Add("Content-Type", contentType)
	}
	res, err := c.httpCli.Do(req)
	if err != nil {
		t.Fatalf("could not send HTTP request: %v", err)
	}

	body := readAllAndClose(t, res.Body)

	return body, res.StatusCode
}

func (c *Client) Write(t *testing.T, address string, data []string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("cannot dial %s: %s", address, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	d := []byte(strings.Join(data, "\n"))
	n, err := conn.Write(d)
	if err != nil {
		t.Fatalf("cannot write %d bytes to %s: %s", len(d), address, err)
	}
	if n != len(d) {
		t.Fatalf("BUG: conn.Write() returned unexpected number of written bytes to %s; got %d; want %d", address, n, len(d))
	}
}

// readAllAndClose reads everything from the response body and then closes it.
func readAllAndClose(t *testing.T, responseBody io.ReadCloser) string {
	t.Helper()

	defer responseBody.Close()
	b, err := io.ReadAll(responseBody)
	if err != nil {
		t.Fatalf("could not read response body: %d", err)
	}
	return string(b)
}

// ServesMetrics is used to retrieve the app's metrics.
//
// This type is expected to be embedded by the apps that serve metrics.
type ServesMetrics struct {
	metricsURL string
	cli        *Client
}

// GetIntMetric retrieves the value of a metric served by an app at /metrics URL.
// The value is then converted to int.
func (app *ServesMetrics) GetIntMetric(t *testing.T, metricName string) int {
	t.Helper()

	return int(app.GetMetric(t, metricName))
}

// GetMetric retrieves the value of a metric served by an app at /metrics URL.
func (app *ServesMetrics) GetMetric(t *testing.T, metricName string) float64 {
	t.Helper()

	metrics, statusCode := app.cli.Get(t, app.metricsURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
	for _, metric := range strings.Split(metrics, "\n") {
		value, found := strings.CutPrefix(metric, metricName)
		if found {
			value = strings.Trim(value, " ")
			res, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatalf("could not parse metric value %s: %v", metric, err)
			}
			return res
		}
	}
	t.Fatalf("metric not found: %s", metricName)
	return 0
}

// GetMetricsByPrefix retrieves the values of all metrics that start with given
// prefix.
func (app *ServesMetrics) GetMetricsByPrefix(t *testing.T, prefix string) []float64 {
	t.Helper()

	values := []float64{}

	metrics, statusCode := app.cli.Get(t, app.metricsURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
	for _, metric := range strings.Split(metrics, "\n") {
		if !strings.HasPrefix(metric, prefix) {
			continue
		}

		parts := strings.Split(metric, " ")
		if len(parts) < 2 {
			t.Fatalf("unexpected record format: got %q, want metric name and value separated by a space", metric)
		}

		value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			t.Fatalf("could not parse metric value %s: %v", metric, err)
		}

		values = append(values, value)
	}
	return values
}
