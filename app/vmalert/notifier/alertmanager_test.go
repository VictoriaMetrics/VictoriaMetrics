package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func TestAlertManager_Addr(t *testing.T) {
	const addr = "http://localhost"
	am, err := NewAlertManager(addr, nil, promauth.HTTPClientConfig{}, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if am.Addr() != addr {
		t.Fatalf("expected to have %q; got %q", addr, am.Addr())
	}
}

func TestAlertManager_Send(t *testing.T) {
	const baUser, baPass = "foo", "bar"
	const headerKey, headerValue = "TenantID", "foo"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("should not be called")
	})
	c := -1
	mux.HandleFunc(alertManagerPath, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Fatalf("unauthorized request")
		}
		if user != baUser || pass != baPass {
			t.Fatalf("wrong creds %q:%q; expected %q:%q", user, pass, baUser, baPass)
		}
		c++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method got %s", r.Method)
		}
		switch c {
		case 0:
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		case 1:
			if r.Header.Get(headerKey) != headerValue {
				t.Fatalf("expected header %q to be set to %q; got %q instead", headerKey, headerValue, r.Header.Get(headerKey))
			}
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
				t.Fatalf("can not unmarshal data into alert %s", err)
			}
			if len(a) != 1 {
				t.Fatalf("expected 1 alert in array got %d", len(a))
			}
			if a[0].GeneratorURL != "0/0" {
				t.Fatalf("expected 0/0 as generatorURL got %s", a[0].GeneratorURL)
			}
			if a[0].StartsAt.IsZero() {
				t.Fatalf("expected non-zero start time")
			}
			if a[0].EndAt.IsZero() {
				t.Fatalf("expected non-zero end time")
			}
			if len(a[0].Labels) != 1 {
				t.Fatalf("expected 1 labels got %d", len(a[0].Labels))
			}
			if len(a[0].Annotations) != 2 {
				t.Fatalf("expected 2 annotations got %d", len(a[0].Annotations))
			}
			if r.Header.Get(headerKey) != "bar" {
				t.Fatalf("expected header %q to be set to %q; got %q instead", headerKey, headerValue, r.Header.Get(headerKey))
			}
		case 3:
			var a []struct {
				Labels map[string]string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
				t.Fatalf("can not unmarshal data into alert %s", err)
			}
			if len(a) != 1 {
				t.Fatalf("expected 1 alert in array got %d", len(a))
			}
			if len(a[0].Labels) != 3 {
				t.Fatalf("expected 3 labels got %d", len(a[0].Labels))
			}
			if a[0].Labels["env"] != "prod" {
				t.Fatalf("expected env label to be prod during relabeling, got %s", a[0].Labels["env"])
			}
			if r.Header.Get(headerKey) != "bar" {
				t.Fatalf("expected header %q to be set to %q; got %q instead", headerKey, "bar", r.Header.Get(headerKey))
			}
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	aCfg := promauth.HTTPClientConfig{
		BasicAuth: &promauth.BasicAuthConfig{
			Username: baUser,
			Password: promauth.NewSecret(baPass),
		},
		Headers: []string{fmt.Sprintf("%s:%s", headerKey, headerValue)},
	}
	parsedConfigs, err := promrelabel.ParseRelabelConfigsData([]byte(`
- action: drop
  if: '{tenant="0"}'
  regex: ".*"
- target_label: "env"
  replacement: "prod"
  if: '{tenant="1"}'
`))
	if err != nil {
		t.Fatalf("unexpected error when parse relabeling config: %s", err)
	}
	am, err := NewAlertManager(srv.URL+alertManagerPath, func(alert Alert) string {
		return strconv.FormatUint(alert.GroupID, 10) + "/" + strconv.FormatUint(alert.ID, 10)
	}, aCfg, parsedConfigs, 0)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := am.Send(context.Background(), []Alert{{Labels: map[string]string{"a": "b"}}}, nil); err == nil {
		t.Fatalf("expected connection error got nil")
	}

	if err := am.Send(context.Background(), []Alert{{Labels: map[string]string{"a": "b"}}}, nil); err == nil {
		t.Fatalf("expected wrong http code error got nil")
	}

	if err := am.Send(context.Background(), []Alert{{
		GroupID:     0,
		Name:        "alert0",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC(),
		Labels:      map[string]string{"alertname": "alert0"},
		Annotations: map[string]string{"a": "b", "c": "d"},
	}}, map[string]string{headerKey: "bar"}); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	if err := am.Send(context.Background(), []Alert{
		// drop tenant0 alert message during relabeling
		{
			Name:   "alert1",
			Labels: map[string]string{"rule": "test", "tenant": "0"},
		},
		{
			Name:   "alert2",
			Labels: map[string]string{"rule": "test", "tenant": "1"},
		},
	}, map[string]string{headerKey: "bar"}); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	if c != 3 {
		t.Fatalf("expected 3 calls(count from zero) to server got %d", c)
	}
}
