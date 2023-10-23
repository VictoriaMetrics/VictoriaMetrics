package rule

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func newTimeSeries(values []float64, timestamps []int64, labels map[string]string) prompbmarshal.TimeSeries {
	ts := prompbmarshal.TimeSeries{
		Samples: make([]prompbmarshal.Sample, len(values)),
	}
	for i := range values {
		ts.Samples[i] = prompbmarshal.Sample{
			Value:     values[i],
			Timestamp: time.Unix(timestamps[i], 0).UnixNano() / 1e6,
		}
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys) // make order deterministic
	for _, key := range keys {
		ts.Labels = append(ts.Labels, prompbmarshal.Label{
			Name:  key,
			Value: labels[key],
		})
	}
	return ts
}

// newTimeSeriesPB creates prompbmarshal.TimeSeries with given
// values, timestamps and labels.
// It expects that labels are already sorted.
func newTimeSeriesPB(values []float64, timestamps []int64, labels []prompbmarshal.Label) prompbmarshal.TimeSeries {
	ts := prompbmarshal.TimeSeries{
		Samples: make([]prompbmarshal.Sample, len(values)),
	}
	for i := range values {
		ts.Samples[i] = prompbmarshal.Sample{
			Value:     values[i],
			Timestamp: time.Unix(timestamps[i], 0).UnixNano() / 1e6,
		}
	}
	ts.Labels = labels
	return ts
}

type curlWriter struct {
	b strings.Builder
}

func (cw *curlWriter) string() string {
	res := "curl " + cw.b.String()
	cw.b.Reset()
	return strings.TrimSpace(res)
}

func (cw *curlWriter) addWithEsc(str string) {
	escStr := `'` + strings.Replace(str, `'`, `'\''`, -1) + `'`
	cw.add(escStr)
}

func (cw *curlWriter) add(str string) {
	cw.b.WriteString(str)
	cw.b.WriteString(" ")
}

func requestToCurl(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}

	cw := &curlWriter{}

	schema := req.URL.Scheme
	requestURL := req.URL.String()
	if !datasource.ShowDatasourceURL() {
		requestURL = req.URL.Redacted()
	}
	if schema == "" {
		schema = "http"
		if req.TLS != nil {
			schema = "https"
		}
		requestURL = schema + "://" + req.Host + requestURL
	}

	if schema == "https" {
		cw.add("-k")
	}

	cw.add("-X")
	cw.add(req.Method)

	var keys []string
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		cw.add("-H")
		if !datasource.ShowDatasourceURL() && isSecreteHeader(k) {
			cw.addWithEsc(fmt.Sprintf("%s: <secret>", k))
			continue
		}
		cw.addWithEsc(fmt.Sprintf("%s: %s", k, strings.Join(req.Header[k], " ")))
	}

	cw.addWithEsc(requestURL)
	return cw.string()
}

var secretWords = []string{"auth", "pass", "key", "secret", "token"}

func isSecreteHeader(str string) bool {
	s := strings.ToLower(str)
	for _, secret := range secretWords {
		if strings.Contains(s, secret) {
			return true
		}
	}
	return false
}
