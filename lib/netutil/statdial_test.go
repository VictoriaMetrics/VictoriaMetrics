package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"testing/synctest"
	"time"
)

func TestIsUnixSocketDialError(t *testing.T) {
	f := func(err error, want bool) {
		t.Helper()

		got := isUnixSocketDialError(err)
		if got != want {
			t.Fatalf("unexpected result; got %v; want %v", got, want)
		}
	}

	errDial := errors.New("dial error")
	f(nil, false)
	f(errDial, false)
	f(&net.OpError{Op: "dial", Net: "unix", Err: errDial}, false)
	f(&net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{}, Err: errDial}, false)

	errUnix := &net.OpError{
		Op:   "dial",
		Net:  "unix",
		Addr: &net.UnixAddr{Name: "exporter.sock", Net: "unix"},
		Err:  errDial,
	}
	f(errUnix, true)
	f(fmt.Errorf("wrapped error: %w", errUnix), true)
}

func TestIsTCPv4Addr(t *testing.T) {
	f := func(addr string, resultExpected bool) {
		t.Helper()

		result := isTCPv4Addr(addr)
		if result != resultExpected {
			t.Fatalf("unexpected result for isIPv4Addr(%q); got %v; want %v", addr, result, resultExpected)
		}
	}

	// empty addr
	f("", false)

	// too small number of octets
	f("foobar", false)
	f("1", false)
	f("1.2", false)
	f("1.2.3", false)
	f("1.2.3.", false)

	// non-numeric octets
	f("foo.bar.baz.aaa", false)

	// non-numeric last value
	f("1.2.3.foo", false)

	// negative value
	f("1.2.3.-4", false)

	// missing port
	f("1.2.3.4", false)

	// invalid port
	f("1.2.3.4:foo", false)

	// too big octet
	f("1.2.3.444:5", false)

	// too big port
	f("1.2.3.4:152344", false)

	// IPv6 addresses
	f("[::1]:80", false)
	f("[::]:443", false)
	f("[fe80::1]:8080", false)
	f("[fe80::1]:8080", false)
	f("[2001:db8::1]:9090", false)
	f("2606:4700:4700::1111:80", false)
	f("[fd00::1:2:3]:8428", false)

	// domain names
	f("localhost:80", false)
	f("example.com:443", false)
	f("victoriametrics.com", false)
	f("zombo.com", false)
	f("theonion.com", false)
	f("vmstorage-0:8401", false)
	f("vmstorage-0.svc.cluster.local.:8401", false)
	f("pee-poo.internal:9090", false)

	// full URL
	f("https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@300;400;500;700&family=Space+Grotesk:wght@400;600;700&display=swap", false)

	// random strings
	f("abc", false)
	f("my dog got autism", false)
	f("IDDQD", false)
	f("Galactus", false)
	f("hello:world", false)
	f("12:34:56", false)

	// normal TCPv4 addr
	f("1.2.3.4:5", true)
	f("0.0.0.0:80", true)
	f("1.2.3.4:65535", true)
}

func TestNewStatDialFuncWithTimeoutDeadline(t *testing.T) {
	originalResolver := Resolver
	Resolver = &statDialTestResolver{
		resolver: originalResolver,
		lookupSRV: func(ctx context.Context, _, _, _ string) (string, []*net.SRV, error) {
			<-ctx.Done()
			return "", nil, ctx.Err()
		},
	}
	t.Cleanup(func() { Resolver = originalResolver })

	for _, tc := range []struct {
		name          string
		timeout       time.Duration
		parentTimeout time.Duration
		want          time.Duration
	}{
		{"short", 5 * time.Second, 2 * time.Minute, 5 * time.Second},
		{"above_default", time.Minute, 2 * time.Minute, time.Minute},
		{"zero", 0, time.Minute, time.Minute},
		{"negative", -time.Second, time.Minute, time.Minute},
		{"earlier_parent", time.Minute, time.Second, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), tc.parentTimeout)
				defer cancel()
				dial := NewStatDialFuncWithTimeout("test_dial_deadline", tc.timeout)
				start := time.Now()
				_, err := dial(ctx, "tcp", "srv+kubernetes.test:443")
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("unexpected error; got %v; want deadline exceeded", err)
				}
				if elapsed := time.Since(start); elapsed != tc.want {
					t.Fatalf("unexpected timeout; got %s; want %s", elapsed, tc.want)
				}
			})
		})
	}
}

type statDialTestResolver struct {
	resolver
	lookupSRV func(context.Context, string, string, string) (string, []*net.SRV, error)
}

func (r *statDialTestResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	return r.lookupSRV(ctx, service, proto, name)
}
