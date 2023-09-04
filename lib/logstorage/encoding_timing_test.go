package logstorage

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkMarshalStringsBlock(b *testing.B) {
	block := strings.Split(benchLogs, "\n")

	b.SetBytes(int64(len(benchLogs)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			buf = marshalStringsBlock(buf[:0], block)
		}
	})
}

func BenchmarkStringsBlockUnmarshaler_Unmarshal(b *testing.B) {
	block := strings.Split(benchLogs, "\n")
	data := marshalStringsBlock(nil, block)

	b.SetBytes(int64(len(benchLogs)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		sbu := getStringsBlockUnmarshaler()
		var values []string
		for pb.Next() {
			var err error
			values, err = sbu.unmarshal(values[:0], data, uint64(len(block)))
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			sbu.reset()
		}
		putStringsBlockUnmarshaler(sbu)
	})
}

const benchLogs = `
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
`
