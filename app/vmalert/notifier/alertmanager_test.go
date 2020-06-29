package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestAlertManager_Send(t *testing.T) {
	const baUser, baPass = "foo", "bar"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("should not be called")
	})
	c := -1
	mux.HandleFunc(alertManagerPath, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Errorf("unauthorized request")
		}
		if user != baUser || pass != baPass {
			t.Errorf("wrong creds %q:%q; expected %q:%q",
				user, pass, baUser, baPass)
		}
		c++
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method got %s", r.Method)
		}
		switch c {
		case 0:
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		case 1:
			w.WriteHeader(500)
		case 2:
			var a []struct {
				Labels       map[string]string `json:"labels"`
				StartsAt     time.Time         `json:"startsAt"`
				EndAt        time.Time         `json:"endsAt"`
				Annotations  map[string]string `json:"annotations"`
				GeneratorURL string            `json:"generatorURL"`
			}
			if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
				t.Errorf("can not unmarshal data into alert %s", err)
				t.FailNow()
			}
			if len(a) != 1 {
				t.Errorf("expected 1 alert in array got %d", len(a))
			}
			if a[0].GeneratorURL != "0/0" {
				t.Errorf("exptected 0/0 as generatorURL got %s", a[0].GeneratorURL)
			}
			if a[0].Labels["alertname"] != "alert0" {
				t.Errorf("exptected alert0 as alert name got %s", a[0].Labels["alertname"])
			}
			if a[0].StartsAt.IsZero() {
				t.Errorf("exptected non-zero start time")
			}
			if a[0].EndAt.IsZero() {
				t.Errorf("exptected non-zero end time")
			}
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	am := NewAlertManager(srv.URL, baUser, baPass, func(alert Alert) string {
		return strconv.FormatUint(alert.GroupID, 10) + "/" + strconv.FormatUint(alert.ID, 10)
	}, srv.Client())
	if err := am.Send(context.Background(), []Alert{{}, {}}); err == nil {
		t.Error("expected connection error got nil")
	}
	if err := am.Send(context.Background(), []Alert{}); err == nil {
		t.Error("expected wrong http code error got nil")
	}
	if err := am.Send(context.Background(), []Alert{{
		GroupID:     0,
		Name:        "alert0",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC(),
		Annotations: map[string]string{"a": "b", "c": "d", "e": "f"},
	}}); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if c != 2 {
		t.Errorf("expected 2 calls(count from zero) to server got %d", c)
	}
}
