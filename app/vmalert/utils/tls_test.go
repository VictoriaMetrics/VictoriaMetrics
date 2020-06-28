package utils

import "testing"

func TestTLSConfig(t *testing.T) {
	var certFile, keyFile, CAFile, serverName string
	var insecureSkipVerify bool
	serverName = "test"
	insecureSkipVerify = true
	tlsCfg, err := TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
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
	_, err = TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err == nil {
		t.Errorf("expected keypair error, got nil")
	}
	certFile = ""
	CAFile = "/path/to/nonexisting/cert/file"
	_, err = TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err == nil {
		t.Errorf("expected read error, got nil")
	}
}

func TestTransport(t *testing.T) {
	var certFile, keyFile, CAFile, serverName string
	var insecureSkipVerify bool
	URL := "http://victoriametrics.com"
	_, err := Transport(URL, certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	URL = "https://victoriametrics.com"
	tr, err := Transport(URL, certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if tr.TLSClientConfig == nil {
		t.Errorf("expected TLSClientConfig to be set, got nil")
	}
}
