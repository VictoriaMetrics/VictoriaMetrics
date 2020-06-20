package main

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestGetExternalURL(t *testing.T) {
	expURL := "https://vicotriametrics.com/path"
	u, err := getExternalURL(expURL, "", false)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Errorf("unexpected url want %s, got %s", expURL, u.String())
	}
	h, _ := os.Hostname()
	expURL = fmt.Sprintf("https://%s:4242", h)
	u, err = getExternalURL("", "0.0.0.0:4242", true)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Errorf("unexpected url want %s, got %s", expURL, u.String())
	}
}

func TestGetAlertURLGenerator(t *testing.T) {
	testAlert := notifier.Alert{GroupID: 42, ID: 2, Value: 4}
	u, _ := url.Parse("https://victoriametrics.com/path")
	fn, err := getAlertURLGenerator(u, "", false)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if exp := "https://victoriametrics.com/path/api/v1/42/2/status"; exp != fn(testAlert) {
		t.Errorf("unexpected url want %s, got %s", exp, fn(testAlert))
	}
	_, err = getAlertURLGenerator(nil, "foo?{{invalid}}", true)
	if err == nil {
		t.Errorf("exptected tempalte validation error got nil")
	}
	fn, err = getAlertURLGenerator(u, "foo?query={{$value}}", true)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if exp := "https://victoriametrics.com/path/foo?query=4"; exp != fn(testAlert) {
		t.Errorf("unexpected url want %s, got %s", exp, fn(testAlert))
	}
}
