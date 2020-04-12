package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestHandler(t *testing.T) {
	rule := &Rule{
		Name: "alert",
		alerts: map[uint64]*notifier.Alert{
			0: {},
		},
	}
	rh := &requestHandler{
		groups: []Group{{
			Name:  "group",
			Rules: []*Rule{rule},
		}},
	}
	getResp := func(url string, to interface{}, code int) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Errorf("unexpected err %s", err)
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
	t.Run("/api/v1/alerts", func(t *testing.T) {
		lr := listAlertsResponse{}
		getResp(ts.URL+"/api/v1/alerts", &lr, 200)
		if length := len(lr.Data.Alerts); length != 1 {
			t.Errorf("expected 1 alert got %d", length)
		}
	})
	t.Run("/api/v1/group/0/status", func(t *testing.T) {
		alert := &apiAlert{}
		getResp(ts.URL+"/api/v1/group/0/status", alert, 200)
		expAlert := rule.newAlertAPI(*rule.alerts[0])
		if !reflect.DeepEqual(alert, expAlert) {
			t.Errorf("expected %v is equal to %v", alert, expAlert)
		}
	})
	t.Run("/api/v1/group/1/status", func(t *testing.T) {
		getResp(ts.URL+"/api/v1/group/1/status", nil, 404)
	})
	t.Run("/api/v1/unknown-group/0/status", func(t *testing.T) {
		getResp(ts.URL+"/api/v1/unknown-group/0/status", nil, 404)
	})
	t.Run("/", func(t *testing.T) {
		getResp(ts.URL, nil, 200)
	})
}
