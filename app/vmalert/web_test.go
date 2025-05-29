package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
)

func TestHandler(t *testing.T) {
	fq := &datasource.FakeQuerier{}
	fq.Add(datasource.Metric{
		Values: []float64{1}, Timestamps: []int64{0},
	})
	g := rule.NewGroup(config.Group{
		Name:        "group",
		File:        "rules.yaml",
		Concurrency: 1,
		Rules: []config.Rule{
			{ID: 0, Alert: "alert"},
			{ID: 1, Record: "record"},
		},
	}, fq, 1*time.Minute, nil)

	g.ExecOnce(context.Background(), func() []notifier.Notifier { return nil }, nil, time.Time{})

	m := &manager{groups: map[uint64]*rule.Group{
		g.CreateID(): g,
	}}
	rh := &requestHandler{m: m}

	getResp := func(t *testing.T, url string, to any, code int) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("unexpected err %s", err)
		}
		if code != resp.StatusCode {
			t.Fatalf("unexpected status code %d want %d", resp.StatusCode, code)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Fatalf("err closing body %s", err)
			}
		}()
		if to != nil {
			if err = json.NewDecoder(resp.Body).Decode(to); err != nil {
				t.Fatalf("unexpected err %s", err)
			}
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rh.handler(w, r) }))
	defer ts.Close()

	t.Run("/api/v1/alerts", func(t *testing.T) {
		lr := listAlertsResponse{}
		getResp(t, ts.URL+"/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 1 {
			t.Fatalf("expected 1 alert got %d", length)
		}
	})

	t.Run("/api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules", &lr, 200)
		if length := len(lr.Data.Groups); length != 1 {
			t.Fatalf("expected 1 group got %d", length)
		}
	})

	t.Run("/api/v1/rules&filters", func(t *testing.T) {
		check := func(url string, expGroups, expRules int) {
			t.Helper()
			lr := listGroupsResponse{}
			getResp(t, ts.URL+url, &lr, 200)
			if length := len(lr.Data.Groups); length != expGroups {
				t.Fatalf("expected %d groups got %d", expGroups, length)
			}
			if len(lr.Data.Groups) < 1 {
				return
			}
			var rulesN int
			for _, gr := range lr.Data.Groups {
				rulesN += len(gr.Rules)
			}
			if rulesN != expRules {
				t.Fatalf("expected %d rules got %d", expRules, rulesN)
			}
		}

		check("/api/v1/rules?type=alert", 1, 1)
		check("/api/v1/rules?type=record", 1, 1)

		// no filtering expected due to bad params
		check("/api/v1/rules?type=badParam", 1, 2)
		check("/api/v1/rules?foo=bar", 1, 2)

		check("/api/v1/rules?rule_group[]=foo&rule_group[]=bar", 0, 0)
		check("/api/v1/rules?rule_group[]=foo&rule_group[]=group&rule_group[]=bar", 1, 2)

		check("/api/v1/rules?rule_group[]=group&file[]=foo", 0, 0)
		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml", 1, 2)

		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml&rule_name[]=foo", 1, 0)
		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml&rule_name[]=alert", 1, 1)
		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml&rule_name[]=alert&rule_name[]=record", 1, 2)
	})
	t.Run("/api/v1/rules&exclude_alerts=true", func(t *testing.T) {
		// check if response returns active alerts by default
		lr := listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules?rule_group[]=group&file[]=rules.yaml", &lr, 200)
		activeAlerts := 0
		for _, gr := range lr.Data.Groups {
			for _, r := range gr.Rules {
				activeAlerts += len(r.Alerts)
			}
		}
		if activeAlerts == 0 {
			t.Fatalf("expected at least 1 active alert in response; got 0")
		}

		// disable returning alerts via param
		lr = listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules?rule_group[]=group&file[]=rules.yaml&exclude_alerts=true", &lr, 200)
		activeAlerts = 0
		for _, gr := range lr.Data.Groups {
			for _, r := range gr.Rules {
				activeAlerts += len(r.Alerts)
			}
		}
		if activeAlerts != 0 {
			t.Fatalf("expected to get 0 active alert in response; got %d", activeAlerts)
		}
	})
}

func TestEmptyResponse(t *testing.T) {
	rhWithNoGroups := &requestHandler{m: &manager{groups: make(map[uint64]*rule.Group)}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rhWithNoGroups.handler(w, r) }))
	defer ts.Close()

	getResp := func(t *testing.T, url string, to any, code int) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("unexpected err %s", err)
		}
		if code != resp.StatusCode {
			t.Fatalf("unexpected status code %d want %d", resp.StatusCode, code)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Fatalf("err closing body %s", err)
			}
		}()
		if to != nil {
			if err = json.NewDecoder(resp.Body).Decode(to); err != nil {
				t.Fatalf("unexpected err %s", err)
			}
		}
	}

	t.Run("no groups /api/v1/alerts", func(t *testing.T) {
		lr := listAlertsResponse{}
		getResp(t, ts.URL+"/api/v1/alerts", &lr, 200)
		if lr.Data.Alerts == nil {
			t.Fatalf("expected /api/v1/alerts response to have non-nil data")
		}
	})

	t.Run("no groups /api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil data")
		}
	})

	rhWithEmptyGroup := &requestHandler{m: &manager{groups: map[uint64]*rule.Group{0: {Name: "test"}}}}
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rhWithEmptyGroup.handler(w, r) })

	t.Run("empty group /api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil data")
		}

		group := lr.Data.Groups[0]
		if group.Rules == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil rules for group")
		}
	})
}
