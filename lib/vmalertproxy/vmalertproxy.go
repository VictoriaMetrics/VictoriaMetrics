package vmalertproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	vmhttputil "github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var proxyTimeout = flag.Duration("vmalert.proxyTimeout", 30*time.Second, "Timeout for requests to vmalert instances when multiple -vmalert.proxyURL urls are set "+
	"and the responses are merged. This prevents a single slow vmalert instance from blocking the merged response. "+
	"A single -vmalert.proxyURL is proxied without this timeout")

// Init initializes proxying requests to the given proxyURLs when calling HandleRequest.
//
// Empty proxyURLs disable the proxying - see Enabled.
//
// Init must be called after flag.Parse(), since it uses command-line flags.
func Init(proxyURLs []string) {
	ts := make([]*proxyTarget, 0, len(proxyURLs))
	for _, proxyURL := range proxyURLs {
		if len(proxyURL) == 0 {
			continue
		}
		pu, err := url.Parse(proxyURL)
		if err != nil {
			logger.Fatalf("cannot parse -vmalert.proxyURL=%q: %s", proxyURL, err)
		}
		ts = append(ts, &proxyTarget{
			u:     pu,
			host:  pu.Host,
			proxy: httputil.NewSingleHostReverseProxy(pu),
		})
	}
	targets = ts
}

// Enabled returns true if at least a single non-empty -vmalert.proxyURL has been passed to Init.
func Enabled() bool {
	return len(targets) > 0
}

// HandleRequest proxies the given request path to vmalert at proxyURLs passed to Init().
//
// If a single proxyURL is set, then all the requests are proxied to it as is.
//
// If multiple proxyURLs are set, then responses to the read-only /api/v1/rules and /api/v1/alerts
// requests are fetched from all the configured vmalert instances and merged into a single response
// by concatenating data.groups and data.alerts lists. Responses from unavailable vmalert instances
// are skipped, so the merged response contains data from the healthy instances only.
// The `group_limit` and `page_num` args of /api/v1/rules are applied to the merged groups list - see pagination.
//
// All the other requests, including vmalert web UI, are proxied to the first proxyURL,
// since their responses cannot be merged.
func HandleRequest(w http.ResponseWriter, r *http.Request, path string) {
	if len(targets) > 1 {
		if fieldName := mergeableAPIPaths[path]; fieldName != "" {
			handleMergeRequest(w, r, path, fieldName)
			return
		}
	}
	targets[0].handleRequest(w, r, path)
}

// mergeableAPIPaths maps read-only vmalert API paths to the name of the list at `data` object,
// which is concatenated across all the configured -vmalert.proxyURL.
//
// Only Prometheus-compatible read-only JSON APIs are mergeable. Responses to all the other paths
// (vmalert web UI, /api/v1/notifiers, /api/v1/group, etc.) cannot be merged, so they are proxied
// to the first -vmalert.proxyURL.
var mergeableAPIPaths = map[string]string{
	"/api/v1/rules":  "groups",
	"/api/v1/alerts": "alerts",
}

type proxyTarget struct {
	u     *url.URL
	host  string
	proxy *httputil.ReverseProxy
}

func (t *proxyTarget) handleRequest(w http.ResponseWriter, r *http.Request, path string) {
	defer func() {
		err := recover()
		if err == nil || err == http.ErrAbortHandler {
			// Suppress http.ErrAbortHandler panic.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1353
			return
		}
		// Forward other panics to the caller.
		panic(err)
	}()
	req := r.Clone(r.Context())
	req.URL.Path = path
	req.Host = t.host

	if isGrafanaRequest(r) {
		q := req.URL.Query()
		q.Set("datasource_type", "prometheus")
		req.URL.RawQuery = q.Encode()
		req.RequestURI = ""
	}

	t.proxy.ServeHTTP(w, req)
}

// isGrafanaRequest returns true if r is sent by Grafana.
//
// Grafana currently supports only Prometheus-style alerts. If other alert types
// (e.g. logs or traces) are returned, it may fail with "Error loading alerts".
//
// Grafana queries the vmalert API directly, bypassing the VictoriaMetrics datasource,
// so query params (such as datasource_type) cannot be enforced on the Grafana side.
//
// To ensure compatibility, we detect Grafana requests via the User-Agent and enforce
// `datasource_type=prometheus`.
//
// See:
// - https://github.com/VictoriaMetrics/victoriametrics-datasource/issues/329#issuecomment-3847585443
// - https://github.com/VictoriaMetrics/victoriametrics-datasource/issues/59
func isGrafanaRequest(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get(`User-Agent`), `Grafana`)
}

// handleMergeRequest fetches the given path from all the configured vmalert instances
// and writes the merged response to w.
func handleMergeRequest(w http.ResponseWriter, r *http.Request, path, fieldName string) {
	var pgn *pagination
	if fieldName == "groups" {
		var err error
		pgn, err = parsePagination(r)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if pgn != nil {
			// The merged groups list is paginated by the proxy itself - see the pagination doc comment,
			// so the pagination args must not be forwarded to vmalert instances.
			r = requestWithoutPaginationArgs(r)
		}
	}

	itemsPerTarget := make([][]json.RawMessage, len(targets))
	errsPerTarget := make([]error, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			itemsPerTarget[i], errsPerTarget[i] = t.fetchAPIItems(r, path, fieldName)
		}()
	}
	wg.Wait()

	items := make([]json.RawMessage, 0)
	failedTargets := 0
	for i := range targets {
		if err := errsPerTarget[i]; err != nil {
			// Do not fail the whole request because of a single unavailable vmalert instance -
			// return the merged response from the remaining instances instead.
			//
			// The -vmalert.proxyURL value isn't logged, since it may contain sensitive info such as auth credentials.
			failedTargets++
			logger.Warnf("cannot fetch %s from vmalert at -vmalert.proxyURL #%d: %s", path, i+1, err)
			continue
		}
		items = append(items, itemsPerTarget[i]...)
	}
	if failedTargets == len(targets) {
		errMsg := fmt.Sprintf("all the %d vmalert instances at -vmalert.proxyURL are unavailable; see logs for details", len(targets))
		writeErrorResponse(w, http.StatusServiceUnavailable, errMsg)
		return
	}

	pgnFields := ""
	if pgn != nil {
		var err error
		items, pgnFields, err = pgn.paginate(items)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	var bb bytes.Buffer
	bb.WriteString(`{"status":"success"`)
	if failedTargets > 0 {
		warning := fmt.Sprintf("%d out of %d vmalert instances at -vmalert.proxyURL are unavailable; see logs for details",
			failedTargets, len(targets))
		fmt.Fprintf(&bb, `,"warnings":[%q]`, warning)
	}
	bb.WriteString(pgnFields)
	fmt.Fprintf(&bb, `,"data":{%q:[`, fieldName)
	for i, item := range items {
		if i > 0 {
			bb.WriteByte(',')
		}
		bb.Write(item)
	}
	bb.WriteString(`]}}`)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(bb.Bytes())
}

// fetchAPIItems requests the given path from t and returns the list of items
// stored at the given fieldName of the `data` object in the response.
func (t *proxyTarget) fetchAPIItems(r *http.Request, path, fieldName string) ([]json.RawMessage, error) {
	// Limit the request duration with -vmalert.proxyTimeout, so a single stuck vmalert instance
	// doesn't block the merged response - it is skipped in the same way as an unavailable instance.
	ctx, cancel := context.WithTimeout(r.Context(), *proxyTimeout)
	defer cancel()
	req, err := t.newAPIRequest(ctx, r, path)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: got %d; want %d; response: %s", resp.StatusCode, http.StatusOK, firstBytes(data))
	}
	var ar apiResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, fmt.Errorf("cannot parse response %s: %w", firstBytes(data), err)
	}
	if ar.Status != "success" {
		return nil, fmt.Errorf("unexpected status=%q in the response; errorType=%q, error=%q", ar.Status, ar.ErrorType, ar.Error)
	}
	fieldData, ok := ar.Data[fieldName]
	if !ok {
		// The response doesn't contain the requested list. Treat it as an empty list
		// in the same way as vmalert does for missing entries.
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(fieldData, &items); err != nil {
		return nil, fmt.Errorf("cannot parse data.%s in the response: %w", fieldName, err)
	}
	return items, nil
}

// newAPIRequest returns a GET request to t for the given path with query args copied from r.
func (t *proxyTarget) newAPIRequest(ctx context.Context, r *http.Request, path string) (*http.Request, error) {
	u := *t.u
	u.Path = joinURLPath(t.u.Path, path)
	u.RawPath = ""
	// Preserve the query args at -vmalert.proxyURL and append the request query args to them
	// in the same way as net/http/httputil.NewSingleHostReverseProxy does.
	if t.u.RawQuery == "" || r.URL.RawQuery == "" {
		u.RawQuery = t.u.RawQuery + r.URL.RawQuery
	} else {
		u.RawQuery = t.u.RawQuery + "&" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	copyRequestHeaders(req.Header, r.Header)
	if isGrafanaRequest(r) {
		q := req.URL.Query()
		q.Set("datasource_type", "prometheus")
		req.URL.RawQuery = q.Encode()
	}
	return req, nil
}

// pagination holds the `group_limit` and `page_num` args of the /api/v1/rules request.
//
// When multiple -vmalert.proxyURL urls are set, the pagination is applied to the merged groups list
// by the proxy itself instead of forwarding the args to vmalert instances, since otherwise every
// instance would paginate its own groups independently and the metadata fields such as `total_pages`
// couldn't be calculated for the merged response. The accepted args, the returned metadata fields
// and the returned errors mirror the vmalert behaviour - see listGroups at app/vmalert/web.go.
type pagination struct {
	groupLimit int
	pageNum    int
}

// parsePagination returns the pagination args of r, or nil if r isn't paginated.
func parsePagination(r *http.Request) (*pagination, error) {
	q := r.URL.Query()
	groupLimit := q.Get("group_limit")
	pageNum := q.Get("page_num")
	if groupLimit == "" {
		if pageNum != "" {
			return nil, fmt.Errorf(`"group_limit" needs to be present in order to paginate over the groups`)
		}
		return nil, nil
	}
	var pgn pagination
	v, err := strconv.Atoi(groupLimit)
	if err != nil || v <= 0 {
		return nil, fmt.Errorf(`"group_limit" is expected to be a positive number, found %q`, groupLimit)
	}
	pgn.groupLimit = v
	if pageNum != "" {
		v, err := strconv.Atoi(pageNum)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf(`"page_num" is expected to be a positive number, found %q`, pageNum)
		}
		pgn.pageNum = v
	}
	return &pgn, nil
}

// requestWithoutPaginationArgs returns a copy of r without the pagination query args.
func requestWithoutPaginationArgs(r *http.Request) *http.Request {
	r = r.Clone(r.Context())
	q := r.URL.Query()
	q.Del("group_limit")
	q.Del("page_num")
	r.URL.RawQuery = q.Encode()
	return r
}

// paginate returns the page of groups requested by pgn together with the JSON-encoded
// pagination metadata fields for the merged response.
//
// Zero metadata fields are skipped in the same way as the `omitempty` fields
// of listGroupsResponse at app/vmalert/web.go.
func (pgn *pagination) paginate(groups []json.RawMessage) ([]json.RawMessage, string, error) {
	totalGroups := len(groups)
	totalRules := 0
	for _, g := range groups {
		var v struct {
			Rules []json.RawMessage `json:"rules"`
		}
		// Parse errors are ignored, since g has been already validated as a JSON value
		// and the groups without the `rules` list contain no rules to count.
		_ = json.Unmarshal(g, &v)
		totalRules += len(v.Rules)
	}
	totalPages := max((totalGroups+pgn.groupLimit-1)/pgn.groupLimit, 1)
	if pgn.pageNum > totalPages {
		return nil, "", fmt.Errorf("page_num=%d exceeds total amount of pages in result=%d", pgn.pageNum, totalPages)
	}
	start := 0
	if pgn.pageNum > 0 {
		start = (pgn.pageNum - 1) * pgn.groupLimit
	}
	end := min(start+pgn.groupLimit, totalGroups)

	var sb strings.Builder
	if pgn.pageNum > 0 {
		fmt.Fprintf(&sb, `,"page":%d`, pgn.pageNum)
	}
	fmt.Fprintf(&sb, `,"total_pages":%d`, totalPages)
	if totalGroups > 0 {
		fmt.Fprintf(&sb, `,"total_groups":%d`, totalGroups)
	}
	if totalRules > 0 {
		fmt.Fprintf(&sb, `,"total_rules":%d`, totalRules)
	}
	return groups[start:end], sb.String(), nil
}

// writeErrorResponse writes a Prometheus-compatible JSON error to w.
func writeErrorResponse(w http.ResponseWriter, statusCode int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"status":"error","errorType":%q,"error":%q}`, strconv.Itoa(statusCode), errMsg)
}

// apiResponse is a Prometheus-compatible API response returned by vmalert.
//
// The `data` contents are kept as-is, so the merged response contains the original items
// without the need to keep their schema in sync with vmalert.
type apiResponse struct {
	Status    string                     `json:"status"`
	Data      map[string]json.RawMessage `json:"data"`
	ErrorType string                     `json:"errorType"`
	Error     string                     `json:"error"`
}

// joinURLPath joins a and b in the same way as net/http/httputil.NewSingleHostReverseProxy does,
// so requests to the merged APIs use the same target urls as the plain reverse proxy.
func joinURLPath(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func copyRequestHeaders(dst, src http.Header) {
	for k, vs := range src {
		if skipRequestHeaders[k] {
			continue
		}
		dst[k] = append([]string(nil), vs...)
	}
}

// skipRequestHeaders contains hop-by-hop headers, which must not be copied to the outgoing requests.
//
// Accept-Encoding is skipped too, so net/http could transparently decode gzipped responses.
var skipRequestHeaders = map[string]bool{
	"Accept-Encoding":     true,
	"Connection":          true,
	"Content-Length":      true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Proxy-Connection":    true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func firstBytes(data []byte) []byte {
	if len(data) > 256 {
		return data[:256]
	}
	return data
}

var apiClient = &http.Client{
	Transport: vmhttputil.NewTransport(false, "vmalert_proxy"),
}

var targets []*proxyTarget
