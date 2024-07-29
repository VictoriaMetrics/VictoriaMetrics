package netutil

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
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

func TestGetServerTLSConfig(t *testing.T) {
	f := func(tlsCertFile, tlsKeyFile string, expectErr bool) {
		t.Helper()
		_, err := GetServerTLSConfig(tlsCertFile, tlsKeyFile, "", []string{})
		if !errors.Is(err, nil) != expectErr { // same as: if errors.Is(err, nil) == expectErr {
			t.Fatalf("expect err: %v, get error: %v", expectErr, err)
		}
	}

	mustCreateFile("test.crt", testCRT)
	mustCreateFile("test.key", testPK)
	defer func() {
		_ = os.Remove("test.crt")
		_ = os.Remove("test.key")
	}()

	// check cert file not exist
	f("/a", "./test.key", true)
	// check key file not exist
	f("./test.crt", "/b", true)
	// cert file and key file all exist
	f("./test.crt", "./test.key", false)
}

const (
	testCRT = `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUKm1UQfHNrw+b2T+ARui1PJexOpswDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yNDA3MTAwMzQ1MzVaFw0yNzA3
MTAwMzQ1MzVaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCgdbQ0yIk170LSfhPzrm6LuclwjxGSC31CmmF+BZUH
J19n33BrL++Rfrfz3kMr5JgfVgjeHWZ2PrLp9M9Jf9eXbcpPcPbPKm3rh1VsdNHX
o82BQ0+/pOimpWtA88A1F/XyWqC7b94531oaAOLuQzWWXeUFY4A9WlC6hkNgRj32
EIfuioEXl22pGvQqScgJOJY5nnSFLUvymKUviMTNllIQd8kdNFcz2uKKozoWl9q9
sNHxGZKTa0dfKjeok8X1pybOZK1E7JLUQBi4oPvI6YAKBFV0BTjkqu6ZBMZE1aPe
RxGdGx/fugVc2b3ON6+xfY/gaVjL1eyVG5zI6Z8xB0A3AgMBAAGjUzBRMB0GA1Ud
DgQWBBSetpW1wdKL3jKgHOj0BGX4TKRyHjAfBgNVHSMEGDAWgBSetpW1wdKL3jKg
HOj0BGX4TKRyHjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBU
gyYMzjtDq1/DF48tAnBRFomnzzUsAV2NnuMzPF6mN2kxJunyukFpCLMKYeE4d+Jn
iSDXehLY1pCzKHntTQuqavSOy4uvlboPGFbBuXuR2XYWNqH+wPRd55UtnMntAvBd
Ovec9+1MW5x0RNiiwZqZ3/oRH3CS7c3iRNMq/AXGNWomE1QXT9ujXR+86KuTBS4h
uQB6i6TyKuqzydD9nsqwuviOA7xrAthw6cqrAjgo8KBMJpFavsasnd/cZ+8YQqOW
fTEsNj4PXjYkQP6z6FW/pbNeJLkjuSIwmcs1m5t2bV5oF2kpx+NyqGW0TcYaw3KY
sWswtTQyQldPeQIkIz1p
-----END CERTIFICATE-----`
	testPK = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCgdbQ0yIk170LS
fhPzrm6LuclwjxGSC31CmmF+BZUHJ19n33BrL++Rfrfz3kMr5JgfVgjeHWZ2PrLp
9M9Jf9eXbcpPcPbPKm3rh1VsdNHXo82BQ0+/pOimpWtA88A1F/XyWqC7b94531oa
AOLuQzWWXeUFY4A9WlC6hkNgRj32EIfuioEXl22pGvQqScgJOJY5nnSFLUvymKUv
iMTNllIQd8kdNFcz2uKKozoWl9q9sNHxGZKTa0dfKjeok8X1pybOZK1E7JLUQBi4
oPvI6YAKBFV0BTjkqu6ZBMZE1aPeRxGdGx/fugVc2b3ON6+xfY/gaVjL1eyVG5zI
6Z8xB0A3AgMBAAECggEADG12rWc3YqiAcz9q5ILcMqaLM1RkBtIspkBruvAhaIzu
8Y4Xgw29jyC9Nujo00OgrpNMxjCJE4Zr9/0geC+rxHbZ5kjjAgzmDN8N9C5LZFle
R3EtrN5PsJHQ7+u7tWurflTw7E4lQYk1eBxydwQDPf1RXsH5QtyLB8ScqkkLxSdy
LztMqwu6w9c0TEuzkKMaVgz0xSj2+FLN6yzAQQJUKRCPLff+uKlQx769JA9sji1C
wxcbSJZXdy0EwkGW/4EjGEPxGxZKunSrlem5A5/RPsfFtDo0DWRzZNCt022TVD/X
QesFDt6iZXpo9QUmSPiw25pZn4QD/7bNqePyLxNhLQKBgQDBc+I5WXceSNl5tXOL
UTLkSJaG0R3tBdtQY+Rc0KEQsHi0we968O0BbdFgvdwJw3YWzqm3QtAflgn4GP4X
1pg7CiCy+8cQFY2TShDJpQ//Z0GwDNUeb9I8cevIsONxRsxVphXovRN7/UFc+Hpi
Gnibm/lC9w4F8nXouBCvFzQX3QKBgQDUVv39xUjlBY7qFgTayDY7VfH5HNXgbW+f
sU56EqHtl0VDsDwgI5+91q4uK/X6m0T7FdW1Ua8iKUf+keDzOJR8HeMF01OzpWJR
p/Wqns1P60KGZv/OWn52J7uYMIslU+VyMvCn58QrrRtchJE3QgVKZUcC5kqpWFul
SXax7h6hIwKBgG+OxTlvNzsWpZsDIXOIysFMfsmWFBzYUMXGJS3E/ezi52jNoa2S
/Anj62dPdXGH7zRtzv8on15npq4Us4rJrJX3XC369at30mHKx22RK22MfRvp+oiH
0YQb6e2c3Dw5qKIHmgDR8EeDH0te2yxxuXV6974/PC3/yTD/3FcsGVVdAoGAMeSK
46ECgsWukfRAicO3cnO8WoNbAdPVAZngzbApGjGMFd6IEikstKeH39N2hb8ME09L
GsKpuwYmI3vVdnDZ+tvu5wSDy1dV5cfoYoHTzi6CQCBdhPggdNTbMGRfnZK7+/xa
Lam4n2aaYj/H+0rpAVUQvW6tJmNbjVfYqvA/hC8CgYB8gqUIhP4yz15P2936g8bS
uhNX7Msd6fwLsq/5Hn1j+7oCQ3KfvxOnFUFRDIUQvpLsa38rfn1u06gSXkSoM/3m
WN7PV6auY4J9vhiJcDHLYKWU6IiDPXa2K0EsGarWy1ncJQdpjZPT1Urft2pNF9gP
YwXfJbKUZnJlv9XplwR7Dw==
-----END PRIVATE KEY-----`
)

func mustCreateFile(path, contents string) {
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		panic(fmt.Errorf("cannot create file %q with %d bytes contents: %w", path, len(contents), err))
	}
}
