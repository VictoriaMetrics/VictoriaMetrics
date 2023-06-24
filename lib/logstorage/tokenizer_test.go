package logstorage

import (
	"reflect"
	"strings"
	"testing"
)

func TestTokenizeStrings(t *testing.T) {
	f := func(a, tokensExpected []string) {
		t.Helper()
		tokens := tokenizeStrings(nil, a)
		if !reflect.DeepEqual(tokens, tokensExpected) {
			t.Fatalf("unexpected tokens;\ngot\n%q\nwant\n%q", tokens, tokensExpected)
		}
	}
	f(nil, nil)
	f([]string{""}, nil)
	f([]string{"foo"}, []string{"foo"})
	f([]string{"foo bar---.!!([baz]!!! %$# TaSte"}, []string{"TaSte", "bar", "baz", "foo"})
	f([]string{"теСТ 1234 f12.34", "34 f12 AS"}, []string{"1234", "34", "AS", "f12", "теСТ"})
	f(strings.Split(`
Apr 28 13:43:38 localhost whoopsie[2812]: [13:43:38] online
Apr 28 13:45:01 localhost CRON[12181]: (root) CMD (command -v debian-sa1 > /dev/null && debian-sa1 1 1)
Apr 28 13:48:01 localhost kernel: [36020.497806] CPU0: Core temperature above threshold, cpu clock throttled (total events = 22034)
`, "\n"), []string{"01", "1", "12181", "13", "22034", "28", "2812", "36020", "38", "43", "45", "48", "497806", "Apr", "CMD", "CPU0", "CRON",
		"Core", "above", "clock", "command", "cpu", "debian", "dev", "events", "kernel", "localhost", "null", "online", "root",
		"sa1", "temperature", "threshold", "throttled", "total", "v", "whoopsie"})
}
