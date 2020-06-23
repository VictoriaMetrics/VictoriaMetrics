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

func TestGetTLSConfig(t *testing.T) {
	var certFile, keyFile, CAFile, serverName string
	var insecureSkipVerify bool
	serverName = "test"
	insecureSkipVerify = true
	tlsCfg, err := getTLSConfig(&certFile, &keyFile, &CAFile, &serverName, &insecureSkipVerify)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if tlsCfg == nil {
		t.Errorf("expected tlsConfig to be set, got nil")
	}
	if tlsCfg.ServerName != serverName {
		t.Errorf("unexpected ServerName, want %s, got %s", serverName, tlsCfg.ServerName)
	}
	if tlsCfg.InsecureSkipVerify != insecureSkipVerify {
		t.Errorf("unexpected InsecureSkipVerify, want %v, got %v", insecureSkipVerify, tlsCfg.InsecureSkipVerify)
	}
	certFile = "/path/to/nonexisting/cert/file"
	_, err = getTLSConfig(&certFile, &keyFile, &CAFile, &serverName, &insecureSkipVerify)
	if err == nil {
		t.Errorf("expected keypair error, got nil")
	}
	certFile = ""
	CAFile = "/path/to/nonexisting/cert/file"
	_, err = getTLSConfig(&certFile, &keyFile, &CAFile, &serverName, &insecureSkipVerify)
	if err == nil {
		t.Errorf("expected read error, got nil")
	}
}

func TestGetTransport(t *testing.T) {
	var certFile, keyFile, CAFile, serverName string
	var insecureSkipVerify bool
	URL := "http://victoriametrics.com"
	tr, err := getTransport(&URL, &certFile, &keyFile, &CAFile, &serverName, &insecureSkipVerify)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if tr != nil {
		t.Errorf("expected Transport to be nil, got %v", tr)
	}
	URL = "https://victoriametrics.com"
	tr, err = getTransport(&URL, &certFile, &keyFile, &CAFile, &serverName, &insecureSkipVerify)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if tr.TLSClientConfig == nil {
		t.Errorf("expected TLSClientConfig to be set, got nil")
	}
}
