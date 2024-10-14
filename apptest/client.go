package apptest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// client is used for interacting with the apps over the network.
//
// At the moment it only supports HTTP protocol but may be exptended to support
// RPCs, etc.
type client struct {
	httpCli *http.Client
}

// newClient creates a new client.
func newClient() *client {
	return &client{
		httpCli: &http.Client{
			Transport: &http.Transport{},
		},
	}
}

// closeConnections closes client connections.
func (c *client) closeConnections() {
	c.httpCli.CloseIdleConnections()
}

// apiV1Series retrieves the value of a metric that is made available by an app
// at /metrics URL.
func (c *client) getMetric(t *testing.T, metricsURL, metricName string) float64 {
	t.Helper()

	metrics := c.get(t, metricsURL, http.StatusOK)
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

// apiV1SeriesResponse is an inmemory representation of the /api/v1/series
// response.
type apiV1SeriesResponse struct {
	Status    string
	IsPartial bool
	Data      []map[string]string
}

// apiV1Series sends a query to a /api/v1/series endpoint and returns the list
// of time series that match the query.
func (c *client) apiV1Series(t *testing.T, seriesURL, matchQuery string) *apiV1SeriesResponse {
	t.Helper()

	values := url.Values{}
	values.Add("match[]", matchQuery)
	jsonRes := c.postForm(t, seriesURL, values, http.StatusOK)
	var res apiV1SeriesResponse
	if err := json.Unmarshal([]byte(jsonRes), &res); err != nil {
		t.Fatalf("could not unmarshal /api/v1/series response: %v", err)
	}
	return &res
}

// postForm sends a HTTP GET request. Once the function receives a response, it
// checks whether the response status code matches the expected one and returns
// the response body to the caller.
func (c *client) get(t *testing.T, url string, wantStatusCode int) string {
	t.Helper()
	return c.do(t, http.MethodGet, url, "", "", wantStatusCode)
}

// postForm sends a HTTP POST request. Once the function receives a response, it
// checks whether the response status code matches the expected one and returns
// the response body to the caller.
func (c *client) post(t *testing.T, url, contentType, data string, wantStatusCode int) string {
	t.Helper()
	return c.do(t, http.MethodPost, url, contentType, data, wantStatusCode)
}

// postForm sends a HTTP POST request containing the POST-form data. Once the
// function receives a response, it checks whether the response status code
// matches the expected one and returns the response body to the caller.
func (c *client) postForm(t *testing.T, url string, data url.Values, wantStatusCode int) string {
	t.Helper()
	return c.post(t, url, "application/x-www-form-urlencoded", data.Encode(), wantStatusCode)
}

// do prepares a HTTP request, sends it to the server, receives the response
// from the server, ensures then response code matches the expected one, reads
// the rentire response body and returns it to the caller.
func (c *client) do(t *testing.T, method, url, contentType, data string, wantStatusCode int) string {
	t.Helper()

	req, err := http.NewRequest(method, url, strings.NewReader(data))
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
