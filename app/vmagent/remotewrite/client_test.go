package remotewrite

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestParseRetryAfterHeader(t *testing.T) {
	f := func(retryAfterString string, expectResult time.Duration) {
		t.Helper()

		result := parseRetryAfterHeader(retryAfterString)
		// expect `expectResult == result` when retryAfterString is in seconds or invalid
		// expect the difference between result and expectResult to be lower than 10%
		if !(expectResult == result || math.Abs(float64(expectResult-result))/float64(expectResult) < 0.10) {
			t.Fatalf(
				"incorrect retry after duration, want (ms): %d, got (ms): %d",
				expectResult.Milliseconds(), result.Milliseconds(),
			)
		}
	}

	// retry after header in seconds
	f("10", 10*time.Second)
	// retry after header in date time
	f(time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat), 30*time.Second)
	// retry after header invalid
	f("invalid-retry-after", 0)
	// retry after header not in GMT
	f(time.Now().Add(10*time.Second).Format("Mon, 02 Jan 2006 15:04:05 FAKETZ"), 0)
}

func TestInitSecretFlags(t *testing.T) {
	showRemoteWriteURLOrig := *showRemoteWriteURL
	defer func() {
		*showRemoteWriteURL = showRemoteWriteURLOrig
		flagutil.UnregisterAllSecretFlags()
	}()

	flagutil.UnregisterAllSecretFlags()
	*showRemoteWriteURL = false
	InitSecretFlags()
	if !flagutil.IsSecretFlag("remotewrite.url") {
		t.Fatalf("expecting remoteWrite.url to be secret")
	}
	if !flagutil.IsSecretFlag("remotewrite.headers") {
		t.Fatalf("expecting remoteWrite.headers to be secret")
	}
	if !flagutil.IsSecretFlag("remotewrite.proxyurl") {
		t.Fatalf("expecting remoteWrite.proxyURL to be secret")
	}

	flagutil.UnregisterAllSecretFlags()
	*showRemoteWriteURL = true
	InitSecretFlags()
	if flagutil.IsSecretFlag("remotewrite.url") {
		t.Fatalf("remoteWrite.url must remain visible when -remoteWrite.showURL is set")
	}
	if !flagutil.IsSecretFlag("remotewrite.headers") {
		t.Fatalf("expecting remoteWrite.headers to remain secret")
	}
	if !flagutil.IsSecretFlag("remotewrite.proxyurl") {
		t.Fatalf("expecting remoteWrite.proxyURL to remain secret")
	}
}

func TestRepackBlockFromZstdToSnappy(t *testing.T) {
	expectedPlainBlock := []byte(`foobar`)

	zstdBlock := encoding.CompressZSTDLevel(nil, expectedPlainBlock, 1)
	snappyBlock, err := repackBlockFromZstdToSnappy(zstdBlock)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	actualPlainBlock, err := snappy.Decode(nil, snappyBlock)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if string(actualPlainBlock) != string(expectedPlainBlock) {
		t.Fatalf("unexpected plain block; got %q; want %q", actualPlainBlock, expectedPlainBlock)
	}
}

func TestRepackBlockFromZstdToSnappyInvalidBlock(t *testing.T) {
	snappyBlock, err := repackBlockFromZstdToSnappy([]byte("invalid zstd block"))

	if err == nil {
		t.Fatalf("expected error for invalid zstd block; got nil")
	}
	if len(snappyBlock) != 0 {
		t.Fatalf("expected empty snappy block; got %d bytes", len(snappyBlock))
	}
}

// TestDoRequestEarlyResponse reproduces https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11507
func TestDoRequestEarlyResponse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %s", err)
	}
	defer ln.Close()

	gotCh := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		br := bufio.NewReader(conn)
		contentLength := -1
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
			k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
			if !ok || !strings.EqualFold(k, "Content-Length") {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return
			}
			contentLength = n
		}

		// respond before reading body so client may still be writing it
		if _, err := conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")); err != nil {
			return
		}
		if contentLength < 0 {
			return
		}

		got := make([]byte, contentLength)
		if _, err := io.ReadFull(br, got); err != nil {
			return
		}
		gotCh <- got
	}()

	authCfg, err := (&promauth.Options{}).NewConfig()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	c := &client{
		sanitizedURL: "http://" + ln.Addr().String(),
		authCfg:      authCfg,
		hc:           &http.Client{},
	}

	body := make([]byte, 16*1024*1024)
	for i := range body {
		body[i] = byte(i)
	}
	want := append([]byte(nil), body...)

	resp, err := c.doRequest("http://"+ln.Addr().String()+"/write", body)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer resp.Body.Close()

	// same as runWorker reusing the block right after a successful send
	for i := range body {
		body[i] = 0
	}

	select {
	case got := <-gotCh:
		if !bytes.Equal(got, want) {
			t.Fatalf("server received modified body after reuse; got %d bytes, want %d bytes", len(got), len(want))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for server to drain request body")
	}
}

func TestRequestWriteWaiter(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		w := newRequestWriteWaiter()
		w.gotConn(httptrace.GotConnInfo{})
		w.wroteRequest(httptrace.WroteRequestInfo{})
		w.wait("http://example.com")
	})

	t.Run("err", func(t *testing.T) {
		w := newRequestWriteWaiter()
		w.gotConn(httptrace.GotConnInfo{})
		w.wroteRequest(httptrace.WroteRequestInfo{Err: errors.New("write failed")})
		w.wait("http://example.com")
	})

	t.Run("no conn", func(t *testing.T) {
		w := newRequestWriteWaiter()
		w.wait("http://example.com")
	})

	t.Run("multiple attempts", func(t *testing.T) {
		w := newRequestWriteWaiter()
		w.gotConn(httptrace.GotConnInfo{})
		w.wroteRequest(httptrace.WroteRequestInfo{})
		w.gotConn(httptrace.GotConnInfo{})
		w.wroteRequest(httptrace.WroteRequestInfo{})
		w.wait("http://example.com")
	})

	t.Run("wait until written", func(t *testing.T) {
		w := newRequestWriteWaiter()
		w.gotConn(httptrace.GotConnInfo{})
		done := make(chan struct{})
		go func() {
			w.wait("http://example.com")
			close(done)
		}()
		select {
		case <-done:
			t.Fatal("wait returned before WroteRequest")
		case <-time.After(20 * time.Millisecond):
		}
		w.wroteRequest(httptrace.WroteRequestInfo{})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("wait did not return after WroteRequest")
		}
	})
}
