package promauth

import (
	"testing"
)

func TestNewTLSConfig(t *testing.T) {
	var certFile, keyFile, caFile, serverName string
	var insecureSkipVerify bool

	// empty certFile, keyFile and caFile
	serverName = "test"
	insecureSkipVerify = true
	tlsCfg, err := NewTLSConfig(certFile, keyFile, caFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if tlsCfg == nil {
		t.Fatalf("expected tlsConfig to be set, got nil")
	}
	if tlsCfg.ServerName != serverName {
		t.Fatalf("unexpected ServerName; got %q; want %q", tlsCfg.ServerName, serverName)
	}
	if tlsCfg.InsecureSkipVerify != insecureSkipVerify {
		t.Fatalf("unexpected InsecureSkipVerify; got %v; want %v", tlsCfg.InsecureSkipVerify, insecureSkipVerify)
	}

	// non-existing CA file
	caFile = "/path/to/nonexisting/cert/file"
	_, err = NewTLSConfig(certFile, keyFile, caFile, serverName, insecureSkipVerify)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}

	// non-existing cert file
	caFile = ""
	certFile = "/path/to/nonexisting/cert/file"
	tlsCfg, err = NewTLSConfig(certFile, keyFile, caFile, serverName, insecureSkipVerify)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	_, err = tlsCfg.GetClientCertificate(nil)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}

}

func TestNewTLSTransport(t *testing.T) {
	var certFile, keyFile, caFile, serverName string
	var insecureSkipVerify bool

	tr, err := NewTLSTransport(certFile, keyFile, caFile, serverName, insecureSkipVerify, "test_client")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if tr.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig to be set, got nil")
	}
}
