package netutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var proxyProtocolTestLoggerID atomic.Uint64

func TestProxyProtocolConnTimeout(t *testing.T) {
	f := func(name string, prefix []byte, wantLog bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			server, client := net.Pipe()
			t.Cleanup(func() {
				_ = server.Close()
				_ = client.Close()
			})

			var logs bytes.Buffer
			logger.SetOutputForTests(&logs)
			t.Cleanup(logger.ResetOutputForTest)
			oldLogger := proxyProtocolReadErrorLogger
			// Each case needs a fresh throttler, including when running with -count.
			loggerName := fmt.Sprintf("proxyProtocolTimeoutTest/%d", proxyProtocolTestLoggerID.Add(1))
			proxyProtocolReadErrorLogger = logger.WithThrottler(loggerName, time.Hour)
			t.Cleanup(func() { proxyProtocolReadErrorLogger = oldLogger })

			writeDone := make(chan error, 1)
			go func() {
				if len(prefix) > 0 {
					if _, err := client.Write(prefix); err != nil {
						writeDone <- err
						_ = client.Close()
						return
					}
				}
				// Expire the deadline only after the prefix has been consumed.
				// This exercises a real timeout without waiting for a timer.
				writeDone <- server.SetReadDeadline(time.Now().Add(-time.Second))
			}()

			conn := newProxyProtocolConn(server)
			buf := make([]byte, 1)
			n, err := conn.Read(buf)
			if writeErr := <-writeDone; writeErr != nil {
				t.Fatalf("cannot prepare timed-out connection: %s", writeErr)
			}
			if n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatalf("expecting no data and a deadline error; got n=%d, err=%v", n, err)
			}
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				t.Fatalf("expecting a network timeout; got %v", err)
			}
			if gotLog := strings.Contains(logs.String(), "cannot read proxy proto conn"); gotLog != wantLog {
				t.Fatalf("unexpected error logging: got %t; want %t; logs: %s", gotLog, wantLog, logs.String())
			}
			if got := conn.RemoteAddr(); got != server.RemoteAddr() {
				t.Fatalf("unexpected remote address after timeout: got %v; want %v", got, server.RemoteAddr())
			}
			if n, nextErr := conn.Read(buf); n != 0 || nextErr != err {
				t.Fatalf("expecting the same error on repeated reads; got n=%d, err=%v", n, nextErr)
			}
		})
	}

	f("empty connection", nil, false)
	f("partial header", []byte(v2Identifier), true)
	// A complete IPv4 header declares a 12-byte address block.
	header := append([]byte(v2Identifier), 0x21, 0x11, 0x00, 0x0c)
	f("missing address block", header, true)
	f("partial address block", append(header, 127, 0, 0, 1), true)
}

func TestParseProxyProtocolSuccess(t *testing.T) {
	f := func(body, wantTail []byte, wantAddr net.Addr) {
		t.Helper()
		r := bytes.NewBuffer(body)
		gotAddr, err := readProxyProto(r)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(gotAddr, wantAddr) {
			t.Fatalf("ip not match, got: %v, want: %v", gotAddr, wantAddr)
		}
		gotTail, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("cannot read tail: %s", err)
		}
		if !bytes.Equal(gotTail, wantTail) {
			t.Fatalf("unexpected tail after parsing proxy protocol\ngot:\n%q\nwant:\n%q", gotTail, wantTail)
		}
	}
	// LOCAL addr
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x20, 0x11, 0x00, 0x0C,
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0}, nil,
		nil)
	// ipv4
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
		// ip data srcid,dstip,srcport,dstport
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0}, nil,
		&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80})
	// ipv4 with payload
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
		// ip data
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0,
		// some payload
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0,
	}, []byte{0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0},
		&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80})
	// ipv6
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x24,
		// src and dst ipv6
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		// ports
		0, 80, 0, 0}, nil,
		&net.TCPAddr{IP: net.ParseIP("::1"), Port: 80})
}

func TestParseProxyProtocolFail(t *testing.T) {
	f := func(body []byte) {
		t.Helper()
		r := bytes.NewBuffer(body)
		gotAddr, err := readProxyProto(r)
		if err == nil {
			t.Fatalf("expected error at input %v", body)
		}
		if gotAddr != nil {
			t.Fatalf("expected ip to be nil, got: %v", gotAddr)
		}
	}
	// too short protocol prefix
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A})
	// broken protocol prefix
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21})
	// invalid header
	f([]byte{0x0D, 0x1A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C})
	// invalid version
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x31, 0x11, 0x00, 0x0C})
	// too long block
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0xff, 0x0C})
	// missing bytes in address
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x0C,
		// ip data srcid,dstip,srcport
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80})
	// too short address length
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x11, 0x00, 0x08,
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0})
	// unsupported family
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x31, 0x00, 0x0C,
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0})
	// unsupported command
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x22, 0x11, 0x00, 0x0C,
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0})
	// mismatch ipv6 and ipv4
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x0C,
		// ip data srcid,dstip,srcport
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0})
	// ipv4 udp isn't supported
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x12, 0x00, 0x0C,
		// ip data srcid,dstip,srcport,dstport
		0x7F, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 80, 0, 0})
	// ipv6 udp isn't supported
	f([]byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x22, 0x00, 0x24,
		// src and dst ipv6
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		// ports
		0, 80, 0, 0})
}

func TestParseProxyProtocolIPv6DoesNotAliasPool(t *testing.T) {
	header := func(last byte) *bytes.Buffer {
		return bytes.NewBuffer([]byte{
			0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, 0x21, 0x21, 0x00, 0x24,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, last,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
			0, 80, 0, 0,
		})
	}
	got, err := readProxyProto(header(1))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := readProxyProto(header(2)); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	want := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 80}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("first addr mutated by pool reuse; got %v, want %v", got, want)
	}
}

func TestProxyProtocolConnReadWriteSuccessful(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ppc := newProxyProtocolConn(server)

	expectedData := []byte("Hello, World!")

	// Send proxy protocol header and test data from client
	go func() {
		proxyHeader := []byte{
			0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A, // signature
			0x21,       // version 2
			0x11,       // family IPv4
			0x00, 0x0C, // length: 12 bytes (IPv4 + ports)
			192, 168, 1, 100, // source IP
			10, 0, 0, 1, // destination IP
			0x1F, 0x90, // source port 8080
			0x00, 0x50, // destination port 80
		}

		// net.Pipe should not produce an error as it is completely in-memory
		_, _ = client.Write(proxyHeader)
		_, _ = client.Write(expectedData)
	}()

	// Read from proxy protocol connection
	actualData := make([]byte, len(expectedData))
	n, err := ppc.Read(actualData)
	if err != nil {
		t.Fatalf("failed to read from proxy protocol connection: %v", err)
	}
	if n != len(expectedData) {
		t.Fatalf("expected to read %d bytes, got %d", len(expectedData), n)
	}
	if !bytes.Equal(actualData, expectedData) {
		t.Fatalf("expected %q, got %q", expectedData, actualData)
	}

	// Verify the remote address is correctly extracted
	expectedAddr := &net.TCPAddr{
		IP:   net.IPv4(192, 168, 1, 100),
		Port: 8080,
	}
	gotAddr := ppc.RemoteAddr()
	if !reflect.DeepEqual(gotAddr, expectedAddr) {
		t.Fatalf("expected remote addr %v, got %v", expectedAddr, gotAddr)
	}
}

func TestProxyProtocolConnReadWriteFailure(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ppc := newProxyProtocolConn(server)

	go func() {
		invalidProxyHeader := []byte("GET / HTTP/1.1\r\n\r\n")

		// net.Pipe should not produce an error as it is completely in-memory
		_, _ = client.Write(invalidProxyHeader)
	}()

	buf := make([]byte, 100)
	_, err := ppc.Read(buf)
	if err == nil {
		t.Fatal("expected error when reading from proxy protocol connection; got none")
	}
	if !strings.HasPrefix(err.Error(), `unexpected proxy protocol header`) {
		t.Fatalf("unexpected proxy protocol header error expected; got: %v", err)
	}

	// Should return original remote address on error
	expectedAddr := server.RemoteAddr()
	gotAddr := ppc.RemoteAddr()
	if !reflect.DeepEqual(gotAddr, expectedAddr) {
		t.Fatalf("expected remote addr %v, got %v", expectedAddr, gotAddr)
	}
}
