package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestAlertManager_Addr(t *testing.T) {
	const addr = "http://localhost"
	am, err := NewAlertManager(addr, nil, promauth.HTTPClientConfig{}, nil, 0)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if am.Addr() != addr {
		t.Errorf("expected to have %q; got %q", addr, am.Addr())
	}
}

func TestAlertManager_Send(t *testing.T) {
	const baUser, baPass = "foo", "bar"
	const headerKey, headerValue = "TenantID", "foo"
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
				t.Errorf("expected 0/0 as generatorURL got %s", a[0].GeneratorURL)
			}
			if a[0].StartsAt.IsZero() {
				t.Errorf("expected non-zero start time")
			}
			if a[0].EndAt.IsZero() {
				t.Errorf("expected non-zero end time")
			}
		case 3:
			if r.Header.Get(headerKey) != headerValue {
				t.Errorf("expected header %q to be set to %q; got %q instead",
					headerKey, headerValue, r.Header.Get(headerKey))
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
	}
	am, err := NewAlertManager(srv.URL+alertManagerPath, func(alert Alert) string {
		return strconv.FormatUint(alert.GroupID, 10) + "/" + strconv.FormatUint(alert.ID, 10)
	}, aCfg, nil, 0)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if err := am.Send(context.Background(), []Alert{{}, {}}, nil); err == nil {
		t.Error("expected connection error got nil")
	}
	if err := am.Send(context.Background(), []Alert{}, nil); err == nil {
		t.Error("expected wrong http code error got nil")
	}
	if err := am.Send(context.Background(), []Alert{{
		GroupID:     0,
		Name:        "alert0",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC(),
		Annotations: map[string]string{"a": "b", "c": "d", "e": "f"},
	}}, nil); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if c != 2 {
		t.Errorf("expected 2 calls(count from zero) to server got %d", c)
	}
	if err := am.Send(context.Background(), nil, map[string]string{headerKey: headerValue}); err != nil {
		t.Errorf("unexpected error %s", err)
	}
}
