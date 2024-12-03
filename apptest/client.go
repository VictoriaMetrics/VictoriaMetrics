package apptest

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// Client is used for interacting with the apps over the network.
//
// At the moment it only supports HTTP protocol but may be exptended to support
// RPCs, etc.
type Client struct {
	httpCli *http.Client
}

// NewClient creates a new client.
func NewClient() *Client {
	return &Client{
		httpCli: &http.Client{
			Transport: &http.Transport{},
		},
	}
}

// CloseConnections closes client connections.
func (c *Client) CloseConnections() {
	c.httpCli.CloseIdleConnections()
}

// Get sends a HTTP GET request. Once the function receives a response, it
// checks whether the response status code matches the expected one and returns
// the response body to the caller.
func (c *Client) Get(t *testing.T, url string, wantStatusCode int) string {
	t.Helper()
	return c.do(t, http.MethodGet, url, "", nil, wantStatusCode)
}

// Post sends a HTTP POST request. Once the function receives a response, it
// checks whether the response status code matches the expected one and returns
// the response body to the caller.
func (c *Client) Post(t *testing.T, url, contentType string, data []byte, wantStatusCode int) string {
	t.Helper()
	return c.do(t, http.MethodPost, url, contentType, data, wantStatusCode)
}

// PostForm sends a HTTP POST request containing the POST-form data. Once the
// function receives a response, it checks whether the response status code
// matches the expected one and returns the response body to the caller.
func (c *Client) PostForm(t *testing.T, url string, data url.Values, wantStatusCode int) string {
	t.Helper()
	return c.Post(t, url, "application/x-www-form-urlencoded", []byte(data.Encode()), wantStatusCode)
}

// do prepares a HTTP request, sends it to the server, receives the response
// from the server, ensures then response code matches the expected one, reads
// the rentire response body and returns it to the caller.
func (c *Client) do(t *testing.T, method, url, contentType string, data []byte, wantStatusCode int) string {
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

	if got, want := res.StatusCode, wantStatusCode; got != want {
		t.Fatalf("unexpected response code: got %d, want %d (body: %s)", got, want, body)
	}

	return body
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

// ServesMetrics is used to retrive the app's metrics.
//
// This type is expected to be embdded by the apps that serve metrics.
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

	metrics := app.cli.Get(t, app.metricsURL, http.StatusOK)
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
	t.Fatalf("metic not found: %s", metricName)
	return 0
}

// GetMetricsByPrefix retrieves the values of all metrics that start with given
// prefix.
func (app *ServesMetrics) GetMetricsByPrefix(t *testing.T, prefix string) []float64 {
	t.Helper()

	values := []float64{}

	metrics := app.cli.Get(t, app.metricsURL, http.StatusOK)
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
