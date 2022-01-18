// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package notifier

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	htmlTpl "html/template"
	textTpl "text/template"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/metricsql"
)

// metric is private copy of datasource.Metric,
// it is used for templating annotations,
// Labels as map simplifies templates evaluation.
type metric struct {
	Labels    map[string]string
	Timestamp int64
	Value     float64
}

// datasourceMetricsToTemplateMetrics converts Metrics from datasource package to private copy for templating.
func datasourceMetricsToTemplateMetrics(ms []datasource.Metric) []metric {
	mss := make([]metric, 0, len(ms))
	for _, m := range ms {
		labelsMap := make(map[string]string, len(m.Labels))
		for _, labelValue := range m.Labels {
			labelsMap[labelValue.Name] = labelValue.Value
		}
		mss = append(mss, metric{
			Labels:    labelsMap,
			Timestamp: m.Timestamps[0],
			Value:     m.Values[0]})
	}
	return mss
}

// QueryFn is used to wrap a call to datasource into simple-to-use function
// for templating functions.
type QueryFn func(query string) ([]datasource.Metric, error)

var tmplFunc textTpl.FuncMap

// InitTemplateFunc initiates template helper functions
func InitTemplateFunc(externalURL *url.URL) {
	// See https://prometheus.io/docs/prometheus/latest/configuration/template_reference/
	tmplFunc = textTpl.FuncMap{
		/* Strings */

		// reReplaceAll ReplaceAllString returns a copy of src, replacing matches of the Regexp with
		// the replacement string repl. Inside repl, $ signs are interpreted as in Expand,
		// so for instance $1 represents the text of the first submatch.
		// alias for https://golang.org/pkg/regexp/#Regexp.ReplaceAllString
		"reReplaceAll": func(pattern, repl, text string) string {
			re := regexp.MustCompile(pattern)
			return re.ReplaceAllString(text, repl)
		},

		// match reports whether the string s
		// contains any match of the regular expression pattern.
		// alias for https://golang.org/pkg/regexp/#MatchString
		"match": regexp.MatchString,

		// title returns a copy of the string s with all Unicode letters
		// that begin words mapped to their Unicode title case.
		// alias for https://golang.org/pkg/strings/#Title
		"title": strings.Title,

		// toUpper returns s with all Unicode letters mapped to their upper case.
		// alias for https://golang.org/pkg/strings/#ToUpper
		"toUpper": strings.ToUpper,

		// toLower returns s with all Unicode letters mapped to their lower case.
		// alias for https://golang.org/pkg/strings/#ToLower
		"toLower": strings.ToLower,

		// stripPort splits string into host and port, then returns only host.
		"stripPort": func(hostPort string) string {
			host, _, err := net.SplitHostPort(hostPort)
			if err != nil {
				return hostPort
			}
			return host
		},

		// parseDuration parses a duration string such as "1h" into the number of seconds it represents
		"parseDuration": func(d string) (float64, error) {
			ms, err := metricsql.DurationValue(d, 0)
			if err != nil {
				return 0, err
			}
			return float64(ms) / 1000, nil
		},

		/* Numbers */

		// humanize converts given number to a human readable format
		// by adding metric prefixes https://en.wikipedia.org/wiki/Metric_prefix
		"humanize": func(v float64) string {
			if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v)
			}
			if math.Abs(v) >= 1 {
				prefix := ""
				for _, p := range []string{"k", "M", "G", "T", "P", "E", "Z", "Y"} {
					if math.Abs(v) < 1000 {
						break
					}
					prefix = p
					v /= 1000
				}
				return fmt.Sprintf("%.4g%s", v, prefix)
			}
			prefix := ""
			for _, p := range []string{"m", "u", "n", "p", "f", "a", "z", "y"} {
				if math.Abs(v) >= 1 {
					break
				}
				prefix = p
				v *= 1000
			}
			return fmt.Sprintf("%.4g%s", v, prefix)
		},

		// humanize1024 converts given number to a human readable format with 1024 as base
		"humanize1024": func(v float64) string {
			if math.Abs(v) <= 1 || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v)
			}
			prefix := ""
			for _, p := range []string{"ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"} {
				if math.Abs(v) < 1024 {
					break
				}
				prefix = p
				v /= 1024
			}
			return fmt.Sprintf("%.4g%s", v, prefix)
		},

		// humanizeDuration converts given seconds to a human readable duration
		"humanizeDuration": func(v float64) string {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v)
			}
			if v == 0 {
				return fmt.Sprintf("%.4gs", v)
			}
			if math.Abs(v) >= 1 {
				sign := ""
				if v < 0 {
					sign = "-"
					v = -v
				}
				seconds := int64(v) % 60
				minutes := (int64(v) / 60) % 60
				hours := (int64(v) / 60 / 60) % 24
				days := int64(v) / 60 / 60 / 24
				// For days to minutes, we display seconds as an integer.
				if days != 0 {
					return fmt.Sprintf("%s%dd %dh %dm %ds", sign, days, hours, minutes, seconds)
				}
				if hours != 0 {
					return fmt.Sprintf("%s%dh %dm %ds", sign, hours, minutes, seconds)
				}
				if minutes != 0 {
					return fmt.Sprintf("%s%dm %ds", sign, minutes, seconds)
				}
				// For seconds, we display 4 significant digits.
				return fmt.Sprintf("%s%.4gs", sign, v)
			}
			prefix := ""
			for _, p := range []string{"m", "u", "n", "p", "f", "a", "z", "y"} {
				if math.Abs(v) >= 1 {
					break
				}
				prefix = p
				v *= 1000
			}
			return fmt.Sprintf("%.4g%ss", v, prefix)
		},

		// humanizePercentage converts given ratio value to a fraction of 100
		"humanizePercentage": func(v float64) string {
			return fmt.Sprintf("%.4g%%", v*100)
		},

		// humanizeTimestamp converts given timestamp to a human readable time equivalent
		"humanizeTimestamp": func(v float64) string {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v)
			}
			t := TimeFromUnixNano(int64(v * 1e9)).Time().UTC()
			return fmt.Sprint(t)
		},

		/* URLs */

		// externalURL returns value of `external.url` flag
		"externalURL": func() string {
			return externalURL.String()
		},

		// pathPrefix returns a Path segment from the URL value in `external.url` flag
		"pathPrefix": func() string {
			return externalURL.Path
		},

		// pathEscape escapes the string so it can be safely placed inside a URL path segment,
		// replacing special characters (including /) with %XX sequences as needed.
		// alias for https://golang.org/pkg/net/url/#PathEscape
		"pathEscape": func(u string) string {
			return url.PathEscape(u)
		},

		// queryEscape escapes the string so it can be safely placed
		// inside a URL query.
		// alias for https://golang.org/pkg/net/url/#QueryEscape
		"queryEscape": func(q string) string {
			return url.QueryEscape(q)
		},

		// crlfEscape replaces new line chars to skip URL encoding.
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/890
		"crlfEscape": func(q string) string {
			q = strings.Replace(q, "\n", `\n`, -1)
			return strings.Replace(q, "\r", `\r`, -1)
		},

		// quotesEscape escapes quote char
		"quotesEscape": func(q string) string {
			return strings.Replace(q, `"`, `\"`, -1)
		},

		// query executes the MetricsQL/PromQL query against
		// configured `datasource.url` address.
		// For example, {{ query "foo" | first | value }} will
		// execute "/api/v1/query?query=foo" request and will return
		// the first value in response.
		"query": func(q string) ([]metric, error) {
			// query function supposed to be substituted at funcsWithQuery().
			// it is present here only for validation purposes, when there is no
			// provided datasource.
			//
			// return non-empty slice to pass validation with chained functions in template
			// see issue #989 for details
			return []metric{{}}, nil
		},

		// first returns the first by order element from the given metrics list.
		// usually used alongside with `query` template function.
		"first": func(metrics []metric) (metric, error) {
			if len(metrics) > 0 {
				return metrics[0], nil
			}
			return metric{}, errors.New("first() called on vector with no elements")
		},

		// label returns the value of the given label name for the given metric.
		// usually used alongside with `query` template function.
		"label": func(label string, m metric) string {
			return m.Labels[label]
		},

		// value returns the value of the given metric.
		// usually used alongside with `query` template function.
		"value": func(m metric) float64 {
			return m.Value
		},

		/* Helpers */

		// Converts a list of objects to a map with keys arg0, arg1 etc.
		// This is intended to allow multiple arguments to be passed to templates.
		"args": func(args ...interface{}) map[string]interface{} {
			result := make(map[string]interface{})
			for i, a := range args {
				result[fmt.Sprintf("arg%d", i)] = a
			}
			return result
		},

		// safeHtml marks string as HTML not requiring auto-escaping.
		"safeHtml": func(text string) htmlTpl.HTML {
			return htmlTpl.HTML(text)
		},
	}
}

func funcsWithQuery(query QueryFn) textTpl.FuncMap {
	fm := make(textTpl.FuncMap)
	for k, fn := range tmplFunc {
		fm[k] = fn
	}
	fm["query"] = func(q string) ([]metric, error) {
		result, err := query(q)
		if err != nil {
			return nil, err
		}
		return datasourceMetricsToTemplateMetrics(result), nil
	}
	return fm
}

// Time is the number of milliseconds since the epoch
// (1970-01-01 00:00 UTC) excluding leap seconds.
type Time int64

// TimeFromUnixNano returns the Time equivalent to the Unix Time
// t provided in nanoseconds.
func TimeFromUnixNano(t int64) Time {
	return Time(t / nanosPerTick)
}

// The number of nanoseconds per minimum tick.
const nanosPerTick = int64(minimumTick / time.Nanosecond)

// MinimumTick is the minimum supported time resolution. This has to be
// at least time.Second in order for the code below to work.
const minimumTick = time.Millisecond

// second is the Time duration equivalent to one second.
const second = int64(time.Second / minimumTick)

// Time returns the time.Time representation of t.
func (t Time) Time() time.Time {
	return time.Unix(int64(t)/second, (int64(t)%second)*nanosPerTick)
}
