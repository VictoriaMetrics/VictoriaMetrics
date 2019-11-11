package test

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var (
	parseTimeExpRegex = regexp.MustCompile(`"?{TIME[^}]*}"?`)
	extractRegex      = regexp.MustCompile(`"?{([^}]*)}"?`)
)

// PopulateTimeTplString substitutes {TIME_*} with t in s and returns the result.
func PopulateTimeTplString(s string, t time.Time) string {
	return string(PopulateTimeTpl([]byte(s), t))
}

// PopulateTimeTpl substitutes {TIME_*} with tGlobal in b and returns the result.
func PopulateTimeTpl(b []byte, tGlobal time.Time) []byte {
	return parseTimeExpRegex.ReplaceAllFunc(b, func(repl []byte) []byte {
		t := tGlobal
		repl = extractRegex.FindSubmatch(repl)[1]
		parts := strings.SplitN(string(repl), "-", 2)
		if len(parts) == 2 {
			duration, err := time.ParseDuration(strings.TrimSpace(parts[1]))
			if err != nil {
				log.Fatalf("error %s parsing duration %s in %s", err, parts[1], repl)
			}
			t = t.Add(-duration)
		}
		switch strings.TrimSpace(parts[0]) {
		case `TIME_S`:
			return []byte(fmt.Sprintf("%d", t.Unix()))
		case `TIME_MSZ`:
			return []byte(fmt.Sprintf("%d", t.Unix()*1e3))
		case `TIME_MS`:
			return []byte(fmt.Sprintf("%d", timeToMillis(t)))
		case `TIME_NS`:
			return []byte(fmt.Sprintf("%d", t.UnixNano()))
		default:
			log.Fatalf("unknown time pattern %s in %s", parts[0], repl)
		}
		return repl
	})
}

func timeToMillis(t time.Time) int64 {
	return t.UnixNano() / 1e6
}
