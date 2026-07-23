package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
)

func TestGroupsPaginationWithEvaluationTimeSort(t *testing.T) {
	fqWithDelay := func(delay time.Duration) *datasource.FakeQuerierWithDelay {
		fqd := &datasource.FakeQuerierWithDelay{Delay: delay}
		fqd.Add(datasource.Metric{
			Values:     []float64{1},
			Timestamps: []int64{0},
		})
		return fqd
	}

	newGroup := func(name string, delay time.Duration) *rule.Group {
		g := rule.NewGroup(config.Group{
			Name:        name,
			File:        fmt.Sprintf("%s.yaml", name),
			Type:        config.NewRawType("prometheus"),
			Concurrency: 1,
			Rules: []config.Rule{
				{
					ID:     0,
					Record: fmt.Sprintf("record-%s", name),
					Expr:   "up",
				},
			},
		}, fqWithDelay(delay), 1*time.Minute, nil)

		ch := g.ExecOnce(context.Background(), nil, time.Time{})
		if ch != nil {
			for err := range ch {
				if err != nil {
					t.Fatalf("unexpected exec error for group %q: %s", name, err)
				}
			}
		}
		return g
	}

	// Intentionally choose group names in ascending order, but rule evaluation delays in opposite order.
	// Old (buggy) implementation paginated before sorting, so page membership followed name order.
	gA := newGroup("a", 10*time.Millisecond) // fast
	gB := newGroup("b", 40*time.Millisecond) // slowest
	gC := newGroup("c", 25*time.Millisecond) // middle

	m := &manager{groups: map[uint64]*rule.Group{}}
	m.groups[gA.CreateID()] = gA
	m.groups[gB.CreateID()] = gB
	m.groups[gC.CreateID()] = gC
	rh := &requestHandler{m: m}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rh.handler(w, r) }))
	defer ts.Close()

	get := func(url string) listGroupsResponse {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("unexpected http err: %s", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status code %d for %q", resp.StatusCode, url)
		}
		var lr listGroupsResponse
		if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
			t.Fatalf("failed to decode response: %s", err)
		}
		return lr
	}

	lr := get(ts.URL + "/api/v1/rules?group_limit=1&page_num=1&sort=evaluation_time")
	if lr.Page != 1 {
		t.Fatalf("unexpected page in response: got %d; want %d", lr.Page, 1)
	}
	if lr.TotalPages != 3 {
		t.Fatalf("unexpected total pages in response: got %d; want %d", lr.TotalPages, 3)
	}
	if len(lr.Data.Groups) != 1 {
		t.Fatalf("unexpected groups count in response: got %d; want %d", len(lr.Data.Groups), 1)
	}
	if got, want := lr.Data.Groups[0].Name, "b"; got != want {
		t.Fatalf("unexpected group on page 1: got %q; want %q", got, want)
	}

	lr = get(ts.URL + "/api/v1/rules?group_limit=1&page_num=2&sort=evaluation_time")
	if len(lr.Data.Groups) != 1 {
		t.Fatalf("unexpected groups count in response: got %d; want %d", len(lr.Data.Groups), 1)
	}
	if got, want := lr.Data.Groups[0].Name, "c"; got != want {
		t.Fatalf("unexpected group on page 2: got %q; want %q", got, want)
	}
}
