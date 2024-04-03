package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	g := &rule.Group{
		Name:        "group",
		File:        "rules.yaml",
		Concurrency: 1,
	}
	ar := rule.NewAlertingRule(fq, g, config.Rule{ID: 0, Alert: "alert"})
	rr := rule.NewRecordingRule(fq, g, config.Rule{ID: 1, Record: "record"})
	g.Rules = []rule.Rule{ar, rr}
	g.ExecOnce(context.Background(), func() []notifier.Notifier { return nil }, nil, time.Time{})

	m := &manager{groups: map[uint64]*rule.Group{
		g.ID(): g,
	}}
	rh := &requestHandler{m: m}

	getResp := func(t *testing.T, url string, to interface{}, code int) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("unexpected err %s", err)
		}
		if code != resp.StatusCode {
			t.Errorf("unexpected status code %d want %d", resp.StatusCode, code)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf("err closing body %s", err)
			}
		}()
		if to != nil {
			if err = json.NewDecoder(resp.Body).Decode(to); err != nil {
				t.Errorf("unexpected err %s", err)
			}
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rh.handler(w, r) }))
	defer ts.Close()

	t.Run("/", func(t *testing.T) {
		getResp(t, ts.URL, nil, 200)
		getResp(t, ts.URL+"/vmalert", nil, 200)
		getResp(t, ts.URL+"/vmalert/alerts", nil, 200)
		getResp(t, ts.URL+"/vmalert/groups", nil, 200)
		getResp(t, ts.URL+"/vmalert/notifiers", nil, 200)
		getResp(t, ts.URL+"/rules", nil, 200)
	})

	t.Run("/vmalert/rule", func(t *testing.T) {
		a := ruleToAPI(ar)
		getResp(t, ts.URL+"/vmalert/"+a.WebLink(), nil, 200)
		r := ruleToAPI(rr)
		getResp(t, ts.URL+"/vmalert/"+r.WebLink(), nil, 200)
	})
	t.Run("/vmalert/alert", func(t *testing.T) {
		alerts := ruleToAPIAlert(ar)
		for _, a := range alerts {
			getResp(t, ts.URL+"/vmalert/"+a.WebLink(), nil, 200)
		}
	})
	t.Run("/vmalert/rule?badParam", func(t *testing.T) {
		params := fmt.Sprintf("?%s=0&%s=1", paramGroupID, paramRuleID)
		getResp(t, ts.URL+"/vmalert/rule"+params, nil, 404)

		params = fmt.Sprintf("?%s=1&%s=0", paramGroupID, paramRuleID)
		getResp(t, ts.URL+"/vmalert/rule"+params, nil, 404)
	})

	t.Run("/api/v1/alerts", func(t *testing.T) {
		lr := listAlertsResponse{}
		getResp(t, ts.URL+"/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 1 {
			t.Errorf("expected 1 alert got %d", length)
		}

		lr = listAlertsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 1 {
			t.Errorf("expected 1 alert got %d", length)
		}
	})
	t.Run("/api/v1/alert?alertID&groupID", func(t *testing.T) {
		expAlert := newAlertAPI(ar, ar.GetAlerts()[0])
		alert := &apiAlert{}
		getResp(t, ts.URL+"/"+expAlert.APILink(), alert, 200)
		if !reflect.DeepEqual(alert, expAlert) {
			t.Errorf("expected %v is equal to %v", alert, expAlert)
		}

		alert = &apiAlert{}
		getResp(t, ts.URL+"/vmalert/"+expAlert.APILink(), alert, 200)
		if !reflect.DeepEqual(alert, expAlert) {
			t.Errorf("expected %v is equal to %v", alert, expAlert)
		}
	})

	t.Run("/api/v1/alert?badParams", func(t *testing.T) {
		params := fmt.Sprintf("?%s=0&%s=1", paramGroupID, paramAlertID)
		getResp(t, ts.URL+"/api/v1/alert"+params, nil, 404)
		getResp(t, ts.URL+"/vmalert/api/v1/alert"+params, nil, 404)

		params = fmt.Sprintf("?%s=1&%s=0", paramGroupID, paramAlertID)
		getResp(t, ts.URL+"/api/v1/alert"+params, nil, 404)
		getResp(t, ts.URL+"/vmalert/api/v1/alert"+params, nil, 404)

		// bad request, alertID is missing
		params = fmt.Sprintf("?%s=1", paramGroupID)
		getResp(t, ts.URL+"/api/v1/alert"+params, nil, 400)
		getResp(t, ts.URL+"/vmalert/api/v1/alert"+params, nil, 400)
	})

	t.Run("/api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules", &lr, 200)
		if length := len(lr.Data.Groups); length != 1 {
			t.Errorf("expected 1 group got %d", length)
		}

		lr = listGroupsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if length := len(lr.Data.Groups); length != 1 {
			t.Errorf("expected 1 group got %d", length)
		}
	})
	t.Run("/api/v1/rule?ruleID&groupID", func(t *testing.T) {
		expRule := ruleToAPI(ar)
		gotRule := apiRule{}
		getResp(t, ts.URL+"/"+expRule.APILink(), &gotRule, 200)

		if expRule.ID != gotRule.ID {
			t.Errorf("expected to get Rule %q; got %q instead", expRule.ID, gotRule.ID)
		}

		gotRule = apiRule{}
		getResp(t, ts.URL+"/vmalert/"+expRule.APILink(), &gotRule, 200)

		if expRule.ID != gotRule.ID {
			t.Errorf("expected to get Rule %q; got %q instead", expRule.ID, gotRule.ID)
		}

		gotRuleWithUpdates := apiRuleWithUpdates{}
		getResp(t, ts.URL+"/"+expRule.APILink(), &gotRuleWithUpdates, 200)
		if gotRuleWithUpdates.StateUpdates == nil || len(gotRuleWithUpdates.StateUpdates) < 1 {
			t.Fatalf("expected %+v to have state updates field not empty", gotRuleWithUpdates.StateUpdates)
		}
	})

	t.Run("/api/v1/rules&filters", func(t *testing.T) {
		check := func(url string, expGroups, expRules int) {
			t.Helper()
			lr := listGroupsResponse{}
			getResp(t, ts.URL+url, &lr, 200)
			if length := len(lr.Data.Groups); length != expGroups {
				t.Errorf("expected %d groups got %d", expGroups, length)
			}
			if len(lr.Data.Groups) < 1 {
				return
			}
			var rulesN int
			for _, gr := range lr.Data.Groups {
				rulesN += len(gr.Rules)
			}
			if rulesN != expRules {
				t.Errorf("expected %d rules got %d", expRules, rulesN)
			}
		}

		check("/api/v1/rules?type=alert", 1, 1)
		check("/api/v1/rules?type=record", 1, 1)

		check("/vmalert/api/v1/rules?type=alert", 1, 1)
		check("/vmalert/api/v1/rules?type=record", 1, 1)

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

	getResp := func(t *testing.T, url string, to interface{}, code int) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("unexpected err %s", err)
		}
		if code != resp.StatusCode {
			t.Errorf("unexpected status code %d want %d", resp.StatusCode, code)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf("err closing body %s", err)
			}
		}()
		if to != nil {
			if err = json.NewDecoder(resp.Body).Decode(to); err != nil {
				t.Errorf("unexpected err %s", err)
			}
		}
	}

	t.Run("no groups /api/v1/alerts", func(t *testing.T) {
		lr := listAlertsResponse{}
		getResp(t, ts.URL+"/api/v1/alerts", &lr, 200)
		if lr.Data.Alerts == nil {
			t.Errorf("expected /api/v1/alerts response to have non-nil data")
		}

		lr = listAlertsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/alerts", &lr, 200)
		if lr.Data.Alerts == nil {
			t.Errorf("expected /api/v1/alerts response to have non-nil data")
		}
	})

	t.Run("no groups /api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(t, ts.URL+"/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Errorf("expected /api/v1/rules response to have non-nil data")
		}

		lr = listGroupsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Errorf("expected /api/v1/rules response to have non-nil data")
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

		lr = listGroupsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil data")
		}

		group := lr.Data.Groups[0]
		if group.Rules == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil rules for group")
		}
	})
}
