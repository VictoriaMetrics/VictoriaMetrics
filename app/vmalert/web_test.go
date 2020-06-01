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
	ar := &AlertingRule{
		Name: "alert",
		alerts: map[uint64]*notifier.Alert{
			0: {},
		},
	}
	g := &Group{
		Name:  "group",
		Rules: []Rule{ar},
	}
	m := &manager{groups: make(map[uint64]*Group)}
	m.groups[0] = g
	rh := &requestHandler{m: m}

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
	t.Run("/api/v1/groups", func(t *testing.T) {
		lr := listGroupsResponse{}
		getResp(ts.URL+"/api/v1/groups", &lr, 200)
		if length := len(lr.Data.Groups); length != 1 {
			t.Errorf("expected 1 group got %d", length)
		}
	})
	t.Run("/api/v1/0/0/status", func(t *testing.T) {
		alert := &APIAlert{}
		getResp(ts.URL+"/api/v1/0/0/status", alert, 200)
		expAlert := ar.newAlertAPI(*ar.alerts[0])
		if !reflect.DeepEqual(alert, expAlert) {
			t.Errorf("expected %v is equal to %v", alert, expAlert)
		}
	})
	t.Run("/api/v1/0/1/status", func(t *testing.T) {
		getResp(ts.URL+"/api/v1/0/1/status", nil, 404)
	})
	t.Run("/api/v1/1/0/status", func(t *testing.T) {
		getResp(ts.URL+"/api/v1/1/0/status", nil, 404)
	})
	t.Run("/", func(t *testing.T) {
		getResp(ts.URL, nil, 200)
	})
}
