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

func ruleLink(ar *rule.AlertingRule) string {
	return fmt.Sprintf("api/v1/rule?%s=%d&%s=%d", paramGroupID, ar.GroupID, paramRuleID, ar.ID())
}

func alertLink(ar *rule.AlertingRule, aa *apiAlert) string {
	return fmt.Sprintf("api/v1/alert?%s=%d&%s=%s", paramGroupID, ar.GroupID, paramAlertID, aa.ID)
}

func TestHandler(t *testing.T) {
	fq := &datasource.FakeQuerier{}
	fq.Add(datasource.Metric{
		Values:     []float64{1},
		Timestamps: []int64{0},
	})
	m := &manager{groups: map[uint64]*rule.Group{}}
	var ar *rule.AlertingRule
	for _, dsType := range []string{"prometheus", "", "graphite"} {
		g := rule.NewGroup(config.Group{
			Name:        "group",
			File:        "rules.yaml",
			Type:        config.NewRawType(dsType),
			Concurrency: 1,
			Rules: []config.Rule{
				{
					ID:    0,
					Alert: "alert",
				},
				{
					ID:     1,
					Record: "record",
				},
			},
		}, fq, 1*time.Minute, nil)
		ar = g.Rules[0].(*rule.AlertingRule)
		g.ExecOnce(context.Background(), func() []notifier.Notifier { return nil }, nil, time.Time{})
		m.groups[g.CreateID()] = g
	}
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
		if to != nil && code < 300 {
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
		if length := len(lr.Data.Alerts); length != 3 {
			t.Fatalf("expected 3 alert got %d", length)
		}

		lr = listAlertsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 3 {
			t.Fatalf("expected 3 alert got %d", length)
		}

		lr = listAlertsResponse{}
		getResp(t, ts.URL+"/api/v1/alerts?datasource_type=test", &lr, 400)

		lr = listAlertsResponse{}
		getResp(t, ts.URL+"/api/v1/alerts?datasource_type=prometheus", &lr, 200)
		if length := len(lr.Data.Alerts); length != 2 {
			t.Fatalf("expected 2 alert got %d", length)
		}
	})
	t.Run("/api/v1/alert?alertID&groupID", func(t *testing.T) {
		expAlert := newAlertAPI(ar, ar.GetAlerts()[0])
		alert := &apiAlert{}
		getResp(t, ts.URL+"/"+alertLink(ar, expAlert), alert, 200)
		if !reflect.DeepEqual(alert, expAlert) {
			t.Fatalf("expected %v is equal to %v", alert, expAlert)
		}

		alert = &apiAlert{}
		getResp(t, ts.URL+"/vmalert/"+alertLink(ar, expAlert), alert, 200)
		if !reflect.DeepEqual(alert, expAlert) {
			t.Fatalf("expected %v is equal to %v", alert, expAlert)
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
		if length := len(lr.Data.Groups); length != 3 {
			t.Fatalf("expected 3 group got %d", length)
		}

		lr = listGroupsResponse{}
		getResp(t, ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if length := len(lr.Data.Groups); length != 3 {
			t.Fatalf("expected 3 group got %d", length)
		}
	})
	t.Run("/api/v1/rule?ruleID&groupID", func(t *testing.T) {
		expRule := ruleToAPI(ar)
		gotRule := apiRule{}
		getResp(t, ts.URL+"/"+ruleLink(ar), &gotRule, 200)

		if expRule.ID != gotRule.ID {
			t.Fatalf("expected to get Rule %q; got %q instead", expRule.ID, gotRule.ID)
		}

		gotRule = apiRule{}
		getResp(t, ts.URL+"/vmalert/"+ruleLink(ar), &gotRule, 200)

		if expRule.ID != gotRule.ID {
			t.Fatalf("expected to get Rule %q; got %q instead", expRule.ID, gotRule.ID)
		}

		gotRuleWithUpdates := apiRuleWithUpdates{}
		getResp(t, ts.URL+"/"+ruleLink(ar), &gotRuleWithUpdates, 200)
		if len(gotRuleWithUpdates.StateUpdates) < 1 {
			t.Fatalf("expected %+v to have state updates field not empty", gotRuleWithUpdates.StateUpdates)
		}
	})

	t.Run("/api/v1/rules&filters", func(t *testing.T) {
		check := func(url string, statusCode, expGroups, expRules int) {
			t.Helper()
			lr := listGroupsResponse{}
			getResp(t, ts.URL+url, &lr, statusCode)
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

		check("/api/v1/rules?type=alert", 200, 3, 3)
		check("/api/v1/rules?type=record", 200, 3, 3)
		check("/api/v1/rules?type=records", 400, 0, 0)

		check("/vmalert/api/v1/rules?type=alert", 200, 3, 3)
		check("/vmalert/api/v1/rules?type=record", 200, 3, 3)
		check("/vmalert/api/v1/rules?type=recording", 400, 0, 0)

		check("/vmalert/api/v1/rules?datasource_type=prometheus", 200, 2, 4)
		check("/vmalert/api/v1/rules?datasource_type=graphite", 200, 1, 2)
		check("/vmalert/api/v1/rules?datasource_type=graphiti", 400, 0, 0)

		// no filtering expected due to bad params
		check("/api/v1/rules?type=badParam", 400, 0, 0)
		check("/api/v1/rules?foo=bar", 200, 3, 6)

		check("/api/v1/rules?rule_group[]=foo&rule_group[]=bar", 200, 0, 0)
		check("/api/v1/rules?rule_group[]=foo&rule_group[]=group&rule_group[]=bar", 200, 3, 6)

		check("/api/v1/rules?rule_group[]=group&file[]=foo", 200, 0, 0)
		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml", 200, 3, 6)

		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml&rule_name[]=foo", 200, 3, 0)
		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml&rule_name[]=alert", 200, 3, 3)
		check("/api/v1/rules?rule_group[]=group&file[]=rules.yaml&rule_name[]=alert&rule_name[]=record", 200, 3, 6)
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
		if to != nil && code < 300 {
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
