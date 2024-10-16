package apptest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"testing"
)

type vmselect struct {
	*app
	*servesMetrics
	httpListenAddr          string
	clusternativeListenAddr string
	cli                     *client
}

func mustStartVmselect(t *testing.T, instance string, flags []string, cli *client) *vmselect {
	t.Helper()

	app, err := startVmselect(instance, flags, cli)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
}

func startVmselect(instance string, flags []string, cli *client) (*vmselect, error) {
	app, stderrExtracts, err := startApp(instance, "../bin/vmselect", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-clusternativeListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
			vmselectAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &vmselect{
		app: app,
		servesMetrics: &servesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr:          stderrExtracts[0],
		clusternativeListenAddr: stderrExtracts[1],
		cli:                     cli,
	}, nil
}

// prometheusAPIV1SeriesResponse is an inmemory representation of the
// /prometheus/api/v1/series response.
type prometheusAPIV1SeriesResponse struct {
	Status    string
	IsPartial bool
	Data      []map[string]string
}

// apiV1Series sends a query to a /prometheus/api/v1/series endpoint and returns
// the list of time series that match the query.
func (app *vmselect) prometheusAPIV1Series(t *testing.T, tenant, matchQuery string) *prometheusAPIV1SeriesResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/series", app.httpListenAddr, tenant)
	values := url.Values{}
	values.Add("match[]", matchQuery)
	jsonRes := app.cli.postForm(t, seriesURL, values, http.StatusOK)
	var res prometheusAPIV1SeriesResponse
	if err := json.Unmarshal([]byte(jsonRes), &res); err != nil {
		t.Fatalf("could not unmarshal /api/v1/series response: %v", err)
	}
	return &res
}

func (app *vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
