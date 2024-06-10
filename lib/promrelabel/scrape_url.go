package promrelabel

import (
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// GetScrapeURL makes scrape url and __address_ labels for the given labels and extraParams.
func GetScrapeURL(labels *promutils.Labels, extraParams map[string][]string) (string, string) {
	// See https://www.robustperception.io/life-of-a-label
	scheme := labels.Get("__scheme__")
	if len(scheme) == 0 {
		scheme = "http"
	}
	metricsPath := labels.Get("__metrics_path__")
	if len(metricsPath) == 0 {
		metricsPath = "/metrics"
	}
	address := labels.Get("__address__")
	if len(address) == 0 {
		return "", ""
	}
	// Usability extension to Prometheus behavior: extract optional scheme and metricsPath from __address__.
	// Prometheus silently drops targets with __address__ containing scheme or metricsPath
	// according to https://www.robustperception.io/life-of-a-label/ .
	if strings.HasPrefix(address, "http://") {
		scheme = "http"
		address = address[len("http://"):]
	} else if strings.HasPrefix(address, "https://") {
		scheme = "https"
		address = address[len("https://"):]
	}
	if n := strings.IndexByte(address, '/'); n >= 0 {
		metricsPath = address[n:]
		address = address[:n]
	}
	address = addMissingPort(address, scheme == "https")

	if !strings.HasPrefix(metricsPath, "/") {
		metricsPath = "/" + metricsPath
	}
	params := getParamsFromLabels(labels, extraParams)
	optionalQuestion := ""
	if len(params) > 0 {
		optionalQuestion = "?"
		if strings.Contains(metricsPath, "?") {
			optionalQuestion = "&"
		}
	}
	paramsStr := url.Values(params).Encode()
	scrapeURL := buildScrapeURL(scheme, address, metricsPath, optionalQuestion, paramsStr)
	return scrapeURL, address
}

func getParamsFromLabels(labels *promutils.Labels, extraParams map[string][]string) map[string][]string {
	// See https://www.robustperception.io/life-of-a-label
	var m map[string][]string
	for _, label := range labels.GetLabels() {
		if !strings.HasPrefix(label.Name, "__param_") {
			continue
		}
		name := label.Name[len("__param_"):]
		values := []string{label.Value}
		if p := extraParams[name]; len(p) > 1 {
			values = append(values, p[1:]...)
		}
		if m == nil {
			m = make(map[string][]string)
		}
		m[name] = values
	}
	return m
}

func buildScrapeURL(scheme, address, metricsPath, optionalQuestion, paramsStr string) string {
	bb := bbPool.Get()
	b := bb.B[:0]
	b = append(b, scheme...)
	b = append(b, "://"...)
	b = append(b, address...)
	b = append(b, metricsPath...)
	b = append(b, optionalQuestion...)
	b = append(b, paramsStr...)
	s := bytesutil.InternBytes(b)
	bb.B = b
	bbPool.Put(bb)
	return s
}

func addMissingPort(addr string, isTLS bool) string {
	if strings.Contains(addr, ":") {
		return addr
	}
	if isTLS {
		return concatTwoStrings(addr, ":443")
	}
	return concatTwoStrings(addr, ":80")
}

func concatTwoStrings(x, y string) string {
	bb := bbPool.Get()
	b := bb.B[:0]
	b = append(b, x...)
	b = append(b, y...)
	s := bytesutil.InternBytes(b)
	bb.B = b
	bbPool.Put(bb)
	return s
}

var bbPool bytesutil.ByteBufferPool
