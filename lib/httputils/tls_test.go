package httputils

import "testing"

func TestTLSConfig(t *testing.T) {
	var certFile, keyFile, CAFile, serverName string
	var insecureSkipVerify bool
	serverName = "test"
	insecureSkipVerify = true
	tlsCfg, err := TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if tlsCfg == nil {
		t.Fatalf("expected tlsConfig to be set, got nil")
	}
	if tlsCfg.ServerName != serverName {
		t.Fatalf("unexpected ServerName, want %s, got %s", serverName, tlsCfg.ServerName)
	}
	if tlsCfg.InsecureSkipVerify != insecureSkipVerify {
		t.Fatalf("unexpected InsecureSkipVerify, want %v, got %v", insecureSkipVerify, tlsCfg.InsecureSkipVerify)
	}
	certFile = "/path/to/nonexisting/cert/file"
	_, err = TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err == nil {
		t.Fatalf("expected keypair error, got nil")
	}
	certFile = ""
	CAFile = "/path/to/nonexisting/cert/file"
	_, err = TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err == nil {
		t.Fatalf("expected read error, got nil")
	}
}

func TestTransport(t *testing.T) {
	var certFile, keyFile, CAFile, serverName string
	var insecureSkipVerify bool
	URL := "http://victoriametrics.com"
	_, err := Transport(URL, certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	URL = "https://victoriametrics.com"
	tr, err := Transport(URL, certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if tr.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig to be set, got nil")
	}

	noSchemaURL := "127.0.0.1:8880"
	_, err = Transport(noSchemaURL, certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err == nil {
		t.Fatalf("expected to have parse error for URL without specified schema; got nil instead")
	}
}
