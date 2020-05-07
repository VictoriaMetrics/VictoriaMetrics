package main

import (
	"context"
	"encoding/json"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"
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
		mu: sync.RWMutex{},
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
		alert := &APIAlert{}
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

func Test_requestHandler_runConfigUpdater(t *testing.T) {
	type fields struct {
		groups []Group
	}
	type args struct {
		updateChan     chan os.Signal
		w              *watchdog
		wg             *sync.WaitGroup
		initRulePath   []string
		updateRulePath string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []Group
	}{
		{
			name: "update good rules",
			args: args{
				w:              &watchdog{},
				wg:             &sync.WaitGroup{},
				updateChan:     make(chan os.Signal),
				initRulePath:   []string{"testdata/rules0-good.rules"},
				updateRulePath: "testdata/dir/rules1-good.rules",
			},
			fields: fields{
				groups: []Group{},
			},
			want: []Group{{Name: "duplicatedGroupDiffFiles", Rules: []*Rule{newTestRule("VMRows", time.Second*10)}}},
		},
		{
			name: "update with one bad rule file",
			args: args{
				w:              &watchdog{},
				wg:             &sync.WaitGroup{},
				updateChan:     make(chan os.Signal),
				initRulePath:   []string{"testdata/rules0-good.rules"},
				updateRulePath: "testdata/dir/rules2-bad.rules",
			},
			fields: fields{
				groups: []Group{},
			},
			want: []Group{
				{
					Name: "duplicatedGroupDiffFiles", Rules: []*Rule{
						newTestRule("VMRows", time.Second*10),
					}},
				{
					Name: "TestGroup", Rules: []*Rule{
						newTestRule("Conns", time.Duration(0)),
						newTestRule("ExampleAlertAlwaysFiring", time.Duration(0)),
					}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			grp, err := Parse(tt.args.initRulePath, *validateTemplates)
			if err != nil {
				t.Errorf("cannot setup test: %v", err)
				cancel()
				return
			}
			groupUpdateStorage := startInitGroups(ctx, tt.args.w, nil, grp, tt.args.wg)
			rh := &requestHandler{
				groups: grp,
				mu:     sync.RWMutex{},
			}
			tt.args.wg.Add(1)
			go func() {
				//possible side effect with global var modification
				err = rulePath.Set(tt.args.updateRulePath)
				if err != nil {
					t.Errorf("cannot update rule")
					panic(err)
				}
				//need some delay
				time.Sleep(time.Millisecond * 300)
				tt.args.updateChan <- syscall.SIGHUP
				cancel()

			}()
			rh.runConfigUpdater(ctx, tt.args.updateChan, groupUpdateStorage, tt.args.w, tt.args.wg)
			tt.args.wg.Wait()
			if len(tt.want) != len(rh.groups) {
				t.Errorf("want: %v,\ngot :%v ", tt.want, rh.groups)
			}
		})
	}
}
