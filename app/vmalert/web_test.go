package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestHandler(t *testing.T) {
	ar := &AlertingRule{
		Name: "alert",
		alerts: map[uint64]*notifier.Alert{
			0: {State: notifier.StateFiring},
		},
		state: newRuleState(10),
	}
	ar.state.add(ruleStateEntry{
		time:    time.Now(),
		at:      time.Now(),
		samples: 10,
	})
	rr := &RecordingRule{
		Name:  "record",
		state: newRuleState(10),
	}
	g := &Group{
		Name:  "group",
		Rules: []Rule{ar, rr},
	}
	m := &manager{groups: make(map[uint64]*Group)}
	m.groups[0] = g
	rh := &requestHandler{m: m}

	getResp := func(url string, to interface{}, code int) {
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

	t.Run("/", func(_ *testing.T) {
		getResp(ts.URL, nil, 200)
		getResp(ts.URL+"/vmalert", nil, 200)
		getResp(ts.URL+"/vmalert/alerts", nil, 200)
		getResp(ts.URL+"/vmalert/groups", nil, 200)
		getResp(ts.URL+"/vmalert/notifiers", nil, 200)
		getResp(ts.URL+"/rules", nil, 200)
	})

	t.Run("/vmalert/rule", func(_ *testing.T) {
		a := ar.ToAPI()
		getResp(ts.URL+"/vmalert/"+a.WebLink(), nil, 200)
		r := rr.ToAPI()
		getResp(ts.URL+"/vmalert/"+r.WebLink(), nil, 200)
	})
	t.Run("/vmalert/alert", func(_ *testing.T) {
		alerts := ar.AlertsToAPI()
		for _, a := range alerts {
			getResp(ts.URL+"/vmalert/"+a.WebLink(), nil, 200)
		}
	})
	t.Run("/vmalert/rule?badParam", func(_ *testing.T) {
		params := fmt.Sprintf("?%s=0&%s=1", paramGroupID, paramRuleID)
		getResp(ts.URL+"/vmalert/rule"+params, nil, 404)

		params = fmt.Sprintf("?%s=1&%s=0", paramGroupID, paramRuleID)
		getResp(ts.URL+"/vmalert/rule"+params, nil, 404)
	})

	t.Run("/api/v1/alerts", func(t *testing.T) {
		lr := listAlertsResponse{}
		getResp(ts.URL+"/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 1 {
			t.Errorf("expected 1 alert got %d", length)
		}

		lr = listAlertsResponse{}
		getResp(ts.URL+"/vmalert/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 1 {
			t.Errorf("expected 1 alert got %d", length)
		}
	})
	t.Run("/api/v1/alert?alertID&groupID", func(t *testing.T) {
		expAlert := ar.newAlertAPI(*ar.alerts[0])
		alert := &APIAlert{}
		getResp(ts.URL+"/"+expAlert.APILink(), alert, 200)
		if !reflect.DeepEqual(alert, expAlert) {
			t.Errorf("expected %v is equal to %v", alert, expAlert)
		}

		alert = &APIAlert{}
		getResp(ts.URL+"/vmalert/"+expAlert.APILink(), alert, 200)
		if !reflect.DeepEqual(alert, expAlert) {
			t.Errorf("expected %v is equal to %v", alert, expAlert)
		}
	})

	t.Run("/api/v1/alert?badParams", func(_ *testing.T) {
		params := fmt.Sprintf("?%s=0&%s=1", paramGroupID, paramAlertID)
		getResp(ts.URL+"/api/v1/alert"+params, nil, 404)
		getResp(ts.URL+"/vmalert/api/v1/alert"+params, nil, 404)

		params = fmt.Sprintf("?%s=1&%s=0", paramGroupID, paramAlertID)
		getResp(ts.URL+"/api/v1/alert"+params, nil, 404)
		getResp(ts.URL+"/vmalert/api/v1/alert"+params, nil, 404)

		// bad request, alertID is missing
		params = fmt.Sprintf("?%s=1", paramGroupID)
		getResp(ts.URL+"/api/v1/alert"+params, nil, 400)
		getResp(ts.URL+"/vmalert/api/v1/alert"+params, nil, 400)
	})

	t.Run("/api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(ts.URL+"/api/v1/rules", &lr, 200)
		if length := len(lr.Data.Groups); length != 1 {
			t.Errorf("expected 1 group got %d", length)
		}

		lr = listGroupsResponse{}
		getResp(ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if length := len(lr.Data.Groups); length != 1 {
			t.Errorf("expected 1 group got %d", length)
		}
	})
}

func TestEmptyResponse(t *testing.T) {
	rhWithNoGroups := &requestHandler{m: &manager{groups: make(map[uint64]*Group)}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rhWithNoGroups.handler(w, r) }))
	defer ts.Close()

	getResp := func(url string, to interface{}, code int) {
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
		getResp(ts.URL+"/api/v1/alerts", &lr, 200)
		if lr.Data.Alerts == nil {
			t.Errorf("expected /api/v1/alerts response to have non-nil data")
		}

		lr = listAlertsResponse{}
		getResp(ts.URL+"/vmalert/api/v1/alerts", &lr, 200)
		if lr.Data.Alerts == nil {
			t.Errorf("expected /api/v1/alerts response to have non-nil data")
		}
	})

	t.Run("no groups /api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(ts.URL+"/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Errorf("expected /api/v1/rules response to have non-nil data")
		}

		lr = listGroupsResponse{}
		getResp(ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Errorf("expected /api/v1/rules response to have non-nil data")
		}
	})

	rhWithEmptyGroup := &requestHandler{m: &manager{groups: map[uint64]*Group{0: {Name: "test"}}}}
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rhWithEmptyGroup.handler(w, r) })

	t.Run("empty group /api/v1/rules", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(ts.URL+"/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil data")
		}

		lr = listGroupsResponse{}
		getResp(ts.URL+"/vmalert/api/v1/rules", &lr, 200)
		if lr.Data.Groups == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil data")
		}

		group := lr.Data.Groups[0]
		if group.Rules == nil {
			t.Fatalf("expected /api/v1/rules response to have non-nil rules for group")
		}
	})
}
