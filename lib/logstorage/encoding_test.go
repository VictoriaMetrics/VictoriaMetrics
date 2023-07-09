package logstorage

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func isAlmostEqual(a, b int) bool {
	fa := float64(a)
	fb := float64(b)
	return math.Abs(fa-fb) <= math.Abs(fa+fb)*0.17
}

func TestMarshalUnmarshalStringsBlock(t *testing.T) {
	f := func(logs string, blockLenExpected int) {
		t.Helper()
		var a []string
		if logs != "" {
			a = strings.Split(logs, "\n")
		}
		data := marshalStringsBlock(nil, a)
		if !isAlmostEqual(len(data), blockLenExpected) {
			t.Fatalf("unexpected block length; got %d; want %d; block=%q", len(data), blockLenExpected, data)
		}
		sbu := getStringsBlockUnmarshaler()
		values, err := sbu.unmarshal(nil, data, uint64(len(a)))
		if err != nil {
			t.Fatalf("cannot unmarshal strings block: %s", err)
		}
		if !reflect.DeepEqual(values, a) {
			t.Fatalf("unexpected strings after unmarshaling;\ngot\n%q\nwant\n%q", values, a)
		}
		putStringsBlockUnmarshaler(sbu)
	}
	f("", 5)
	f("foo", 9)
	f(`foo
bar
baz
`, 18)
	f(`
Apr 28 13:39:06 localhost systemd[1]: Started Network Manager Script Dispatcher Service.
Apr 28 13:39:06 localhost nm-dispatcher: req:1 'connectivity-change': new request (2 scripts)
Apr 28 13:39:06 localhost nm-dispatcher: req:1 'connectivity-change': start running ordered scripts...
Apr 28 13:40:05 localhost kernel: [35544.823503] wlp4s0: AP c8:ea:f8:00:6a:31 changed bandwidth, new config is 2437 MHz, width 1 (2437/0 MHz)
Apr 28 13:40:15 localhost kernel: [35554.295612] wlp4s0: AP c8:ea:f8:00:6a:31 changed bandwidth, new config is 2437 MHz, width 2 (2447/0 MHz)
Apr 28 13:43:37 localhost NetworkManager[1516]: <info>  [1651142617.3668] manager: NetworkManager state is now CONNECTED_GLOBAL
Apr 28 13:43:37 localhost dbus-daemon[1475]: [system] Activating via systemd: service name='org.freedesktop.nm_dispatcher' unit='dbus-org.freedesktop.nm-dispatcher.service' requested by ':1.13' (uid=0 pid=1516 comm="/usr/sbin/NetworkManager --no-daemon " label="unconfined")
Apr 28 13:43:37 localhost systemd[1]: Starting Network Manager Script Dispatcher Service...
Apr 28 13:43:37 localhost whoopsie[2812]: [13:43:37] The default IPv4 route is: /org/freedesktop/NetworkManager/ActiveConnection/10
Apr 28 13:43:37 localhost whoopsie[2812]: [13:43:37] Not a paid data plan: /org/freedesktop/NetworkManager/ActiveConnection/10
Apr 28 13:43:37 localhost whoopsie[2812]: [13:43:37] Found usable connection: /org/freedesktop/NetworkManager/ActiveConnection/10
Apr 28 13:43:37 localhost dbus-daemon[1475]: [system] Successfully activated service 'org.freedesktop.nm_dispatcher'
Apr 28 13:43:37 localhost systemd[1]: Started Network Manager Script Dispatcher Service.
Apr 28 13:43:37 localhost nm-dispatcher: req:1 'connectivity-change': new request (2 scripts)
Apr 28 13:43:37 localhost nm-dispatcher: req:1 'connectivity-change': start running ordered scripts...
Apr 28 13:43:38 localhost whoopsie[2812]: [13:43:38] online
Apr 28 13:45:01 localhost CRON[12181]: (root) CMD (command -v debian-sa1 > /dev/null && debian-sa1 1 1)
Apr 28 13:48:01 localhost kernel: [36020.497806] CPU0: Core temperature above threshold, cpu clock throttled (total events = 22034)
Apr 28 13:48:01 localhost kernel: [36020.497807] CPU2: Core temperature above threshold, cpu clock throttled (total events = 22034)
Apr 28 13:48:01 localhost kernel: [36020.497809] CPU1: Package temperature above threshold, cpu clock throttled (total events = 27400)
Apr 28 13:48:01 localhost kernel: [36020.497810] CPU3: Package temperature above threshold, cpu clock throttled (total events = 27400)
Apr 28 13:48:01 localhost kernel: [36020.497810] CPU2: Package temperature above threshold, cpu clock throttled (total events = 27400)
Apr 28 13:48:01 localhost kernel: [36020.497812] CPU0: Package temperature above threshold, cpu clock throttled (total events = 27400)
Apr 28 13:48:01 localhost kernel: [36020.499855] CPU2: Core temperature/speed normal
Apr 28 13:48:01 localhost kernel: [36020.499855] CPU0: Core temperature/speed normal
Apr 28 13:48:01 localhost kernel: [36020.499856] CPU1: Package temperature/speed normal
Apr 28 13:48:01 localhost kernel: [36020.499857] CPU3: Package temperature/speed normal
Apr 28 13:48:01 localhost kernel: [36020.499858] CPU0: Package temperature/speed normal
Apr 28 13:48:01 localhost kernel: [36020.499859] CPU2: Package temperature/speed normal
`, 951)

	// Generate a string longer than 1<<16 bytes
	s := "foo"
	for len(s) < (1 << 16) {
		s += s
	}
	s += "\n"
	lines := s
	f(lines, 36)
	lines += s
	f(lines, 52)

	// Generate more than 256 strings
	lines = ""
	for i := 0; i < 1000; i++ {
		lines += fmt.Sprintf("line %d\n", i)
	}
	f(lines, 766)
}
