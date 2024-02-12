package netutil

import (
	"crypto/tls"
	"reflect"
	"testing"
)

func TestCipherSuitesFromNamesSucces(t *testing.T) {
	f := func(cipherSuites []string, expectedSuites []uint16) {
		t.Helper()

		suites, err := cipherSuitesFromNames(cipherSuites)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(suites, expectedSuites) {
			t.Fatalf("unexpected ciphersuites; got %d; want %d", suites, expectedSuites)
		}
	}

	// Empty ciphersuites
	f(nil, nil)

	// Supported ciphersuites uppercase
	f([]string{
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
	}, []uint16{
		0xc013,
		0xc014,
	})

	// Supported ciphersuites lowercase
	f([]string{
		"tls_ecdhe_rsa_with_aes_128_cbc_sha",
		"tls_ecdhe_rsa_with_aes_256_cbc_sha",
	}, []uint16{
		0xc013,
		0xc014,
	})

	// Correct ciphersuites via numbers
	f([]string{"0xC013", "0xC014"}, []uint16{0xc013, 0xc014})
	f([]string{"0xc013", "0xc014"}, []uint16{0xc013, 0xc014})
}

func TestCipherSuitesFromNamesFailure(t *testing.T) {
	f := func(cipherSuites []string) {
		t.Helper()
		_, err := cipherSuitesFromNames(cipherSuites)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// wrong ciphersuite
	f([]string{"non-existing-ciphersuite"})
	f([]string{"23432"})
	f([]string{"2343223432423"})

	// insecure ciphersuites
	f([]string{"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA", "TLS_ECDHE_RSA_WITH_RC4_128_SHA"})

	// insecure ciphersuite numbers
	f([]string{"0x0005", "0x000a"})
}

func TestParseTLSVersionSuccess(t *testing.T) {
	f := func(s string, want uint16) {
		t.Helper()
		got, err := ParseTLSVersion(s)
		if err != nil {
			t.Fatalf("unexpected error for ParseTLSVersion(%q): %s", s, err)
		}
		if got != want {
			t.Fatalf("unexpected value got from ParseTLSVersion(%q); got %d; want %d", s, got, want)
		}
	}
	// lowercase tlsName
	f("tls10", tls.VersionTLS10)
	f("tls11", tls.VersionTLS11)
	f("tls12", tls.VersionTLS12)
	f("tls13", tls.VersionTLS13)
	// uppercase tlsName
	f("TLS10", tls.VersionTLS10)
	f("TLS11", tls.VersionTLS11)
	f("TLS12", tls.VersionTLS12)
	f("TLS13", tls.VersionTLS13)
	// empty tlsName
	f("", 0)
}

func TestParseTLSVersionFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		_, err := ParseTLSVersion(s)
		if err == nil {
			t.Fatalf("expecting non-nil error for ParseTLSVersion(%q)", s)
		}
	}
	// incorrect tlsName
	f("123")
	// incorrect tlsName with correct prefix
	f("TLS1")
	// incorrect tls version in tlsName
	f("TLS14")
}
