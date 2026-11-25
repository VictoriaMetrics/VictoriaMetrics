package vmstorage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseLogNewSeriesSeconds(t *testing.T) {
	got, err := parseLogNewSeriesSeconds("")
	if err != nil || got != defaultLogNewSeriesSeconds {
		t.Fatalf("empty: got %d, %v", got, err)
	}
	got, err = parseLogNewSeriesSeconds("120")
	if err != nil || got != 120 {
		t.Fatalf("120: got %d, %v", got, err)
	}
	got, err = parseLogNewSeriesSeconds("86400")
	if err != nil || got != 86400 {
		t.Fatalf("86400: got %d, %v", got, err)
	}
	if _, err = parseLogNewSeriesSeconds("abc"); err == nil {
		t.Fatal("expected non-numeric seconds to be rejected")
	}
	if _, err = parseLogNewSeriesSeconds("86401"); err == nil {
		t.Fatal("expected seconds above 24 hours to be rejected")
	}
	if _, err = parseLogNewSeriesSeconds("-1"); err == nil {
		t.Fatal("expected negative seconds to be rejected")
	}
	if _, err = parseLogNewSeriesSeconds("0"); err == nil {
		t.Fatal("expected zero seconds to be rejected")
	}
	if _, err = parseLogNewSeriesSeconds("999999999"); err == nil {
		t.Fatal("expected huge seconds to be rejected")
	}
}

func TestLogNewSeriesInvalidSeconds(t *testing.T) {
	for _, seconds := range []string{"0", "-1", "86401", "abc"} {
		t.Run(seconds, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/internal/log_new_series?seconds="+seconds, nil)
			w := httptest.NewRecorder()
			// No storage is attached: invalid input must return before enabling logging.
			vms := &VMStorage{}
			if !vms.requestHandler(w, r) {
				t.Fatal("request was not handled")
			}
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}
