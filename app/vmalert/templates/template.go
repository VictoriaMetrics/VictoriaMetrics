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

package templates

import (
	"errors"
	"fmt"
	htmlTpl "html/template"
	"io"
	"math"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	textTpl "text/template"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/formatutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// go template execution fails when it's tree is empty
const defaultTemplate = `{{- define "default.template" -}}{{- end -}}`

var tplMu sync.RWMutex

type textTemplate struct {
	current     *textTpl.Template
	replacement *textTpl.Template
}

var masterTmpl textTemplate

func newTemplate() *textTpl.Template {
	tmpl := textTpl.New("").Option("missingkey=zero").Funcs(templateFuncs())
	return textTpl.Must(tmpl.Parse(defaultTemplate))
}

// Load func loads templates from multiple globs specified in pathPatterns and either
// sets them directly to current template if it's undefined or with overwrite=true
// or sets replacement templates and adds templates with new names to a current
func Load(pathPatterns []string, overwrite bool) error {
	var err error
	tmpl := newTemplate()
	for _, tp := range pathPatterns {
		p, err := doublestar.FilepathGlob(tp)
		if err != nil {
			return fmt.Errorf("failed to retrieve a template glob %q: %w", tp, err)
		}
		if len(p) > 0 {
			tmpl, err = tmpl.ParseFiles(p...)
			if err != nil {
				return fmt.Errorf("failed to parse template glob %q: %w", tp, err)
			}
		}
	}
	if len(tmpl.Templates()) > 0 {
		err := tmpl.Execute(io.Discard, nil)
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
	}
	tplMu.Lock()
	defer tplMu.Unlock()
	if masterTmpl.current == nil || overwrite {
		masterTmpl.replacement = nil
		masterTmpl.current = newTemplate()
	} else {
		masterTmpl.replacement = newTemplate()
		if err = copyTemplates(tmpl, masterTmpl.replacement, overwrite); err != nil {
			return err
		}
	}
	return copyTemplates(tmpl, masterTmpl.current, overwrite)
}

func copyTemplates(from *textTpl.Template, to *textTpl.Template, overwrite bool) error {
	if from == nil {
		return nil
	}
	if to == nil {
		to = newTemplate()
	}
	tmpl, err := from.Clone()
	if err != nil {
		return err
	}
	for _, t := range tmpl.Templates() {
		if to.Lookup(t.Name()) == nil || overwrite {
			to, err = to.AddParseTree(t.Name(), t.Tree)
			if err != nil {
				return fmt.Errorf("failed to add template %q: %w", t.Name(), err)
			}
		}
	}
	return nil
}

// Reload func replaces current template with a replacement template
// which was set by Load with override=false
func Reload() {
	tplMu.Lock()
	defer tplMu.Unlock()
	if masterTmpl.replacement != nil {
		masterTmpl.current = masterTmpl.replacement
		masterTmpl.replacement = nil
	}
}

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

// UpdateWithFuncs updates existing or sets a new function map for a template
func UpdateWithFuncs(funcs textTpl.FuncMap) {
	tplMu.Lock()
	defer tplMu.Unlock()
	masterTmpl.current = masterTmpl.current.Funcs(funcs)
}

// GetWithFuncs returns a copy of current template with additional FuncMap
// provided with funcs argument
func GetWithFuncs(funcs textTpl.FuncMap) (*textTpl.Template, error) {
	tplMu.RLock()
	defer tplMu.RUnlock()
	tmpl, err := masterTmpl.current.Clone()
	if err != nil {
		return nil, err
	}
	return tmpl.Funcs(funcs), nil
}

// Get returns a copy of a template
func Get() (*textTpl.Template, error) {
	tplMu.RLock()
	defer tplMu.RUnlock()
	return masterTmpl.current.Clone()
}

// FuncsWithQuery returns a function map that depends on metric data
func FuncsWithQuery(query QueryFn) textTpl.FuncMap {
	return textTpl.FuncMap{
		"query": func(q string) ([]metric, error) {
			if query == nil {
				return nil, fmt.Errorf("cannot execute query %q: query is not available in this context", q)
			}

			result, err := query(q)
			if err != nil {
				return nil, err
			}
			return datasourceMetricsToTemplateMetrics(result), nil
		},
	}
}

// FuncsWithExternalURL returns a function map that depends on externalURL value
func FuncsWithExternalURL(externalURL *url.URL) textTpl.FuncMap {
	return textTpl.FuncMap{
		"externalURL": func() string {
			return externalURL.String()
		},

		"pathPrefix": func() string {
			return externalURL.Path
		},
	}
}

// templateFuncs initiates template helper functions
func templateFuncs() textTpl.FuncMap {
	// See https://prometheus.io/docs/prometheus/latest/configuration/template_reference/
	// and https://github.com/prometheus/prometheus/blob/fa6e05903fd3ce52e374a6e1bf4eb98c9f1f45a7/template/template.go#L150
	return textTpl.FuncMap{
		/* Strings */

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

		// crlfEscape replaces '\n' and '\r' chars with `\\n` and `\\r`.
		// This function is deprecated.
		//
		// It is better to use quotesEscape, jsonEscape, queryEscape or pathEscape instead -
		// these functions properly escape `\n` and `\r` chars according to their purpose.
		"crlfEscape": func(q string) string {
			q = strings.Replace(q, "\n", `\n`, -1)
			return strings.Replace(q, "\r", `\r`, -1)
		},

		// quotesEscape escapes the string, so it can be safely put inside JSON string.
		//
		// See also jsonEscape.
		"quotesEscape": quotesEscape,

		// jsonEscape converts the string to properly encoded JSON string.
		//
		// See also quotesEscape.
		"jsonEscape": jsonEscape,

		// htmlEscape applies html-escaping to q, so it can be safely embedded as plaintext into html.
		//
		// See also safeHtml.
		"htmlEscape": htmlEscape,

		// stripPort splits string into host and port, then returns only host.
		"stripPort": func(hostPort string) string {
			host, _, err := net.SplitHostPort(hostPort)
			if err != nil {
				return hostPort
			}
			return host
		},

		// stripDomain removes the domain part of a FQDN. Leaves port untouched.
		"stripDomain": func(hostPort string) string {
			host, port, err := net.SplitHostPort(hostPort)
			if err != nil {
				host = hostPort
			}
			ip := net.ParseIP(host)
			if ip != nil {
				return hostPort
			}
			host = strings.Split(host, ".")[0]
			if port != "" {
				return net.JoinHostPort(host, port)
			}
			return host
		},

		// match reports whether the string s
		// contains any match of the regular expression pattern.
		// alias for https://golang.org/pkg/regexp/#MatchString
		"match": regexp.MatchString,

		// reReplaceAll ReplaceAllString returns a copy of src, replacing matches of the Regexp with
		// the replacement string repl. Inside repl, $ signs are interpreted as in Expand,
		// so for instance $1 represents the text of the first submatch.
		// alias for https://golang.org/pkg/regexp/#Regexp.ReplaceAllString
		"reReplaceAll": func(pattern, repl, text string) string {
			re := regexp.MustCompile(pattern)
			return re.ReplaceAllString(text, repl)
		},

		// parseDuration parses a duration string such as "1h" into the number of seconds it represents
		"parseDuration": func(s string) (float64, error) {
			d, err := promutils.ParseDuration(s)
			if err != nil {
				return 0, err
			}
			return d.Seconds(), nil
		},

		// same with parseDuration but returns a time.Duration
		"parseDurationTime": func(s string) (time.Duration, error) {
			d, err := promutils.ParseDuration(s)
			if err != nil {
				return 0, err
			}
			return d, nil
		},

		/* Numbers */

		// humanize converts given number to a human readable format
		// by adding metric prefixes https://en.wikipedia.org/wiki/Metric_prefix
		"humanize": func(i interface{}) (string, error) {
			v, err := toFloat64(i)
			if err != nil {
				return "", err
			}
			if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
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
				return fmt.Sprintf("%.4g%s", v, prefix), nil
			}
			prefix := ""
			for _, p := range []string{"m", "u", "n", "p", "f", "a", "z", "y"} {
				if math.Abs(v) >= 1 {
					break
				}
				prefix = p
				v *= 1000
			}
			return fmt.Sprintf("%.4g%s", v, prefix), nil
		},

		// humanize1024 converts given number to a human readable format with 1024 as base
		"humanize1024": func(i interface{}) (string, error) {
			v, err := toFloat64(i)
			if err != nil {
				return "", err
			}
			if math.Abs(v) <= 1 || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			return formatutil.HumanizeBytes(v), nil
		},

		// humanizeDuration converts given seconds to a human-readable duration
		"humanizeDuration": func(i interface{}) (string, error) {
			v, err := toFloat64(i)
			if err != nil {
				return "", err
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			if v == 0 {
				return fmt.Sprintf("%.4gs", v), nil
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
					return fmt.Sprintf("%s%dd %dh %dm %ds", sign, days, hours, minutes, seconds), nil
				}
				if hours != 0 {
					return fmt.Sprintf("%s%dh %dm %ds", sign, hours, minutes, seconds), nil
				}
				if minutes != 0 {
					return fmt.Sprintf("%s%dm %ds", sign, minutes, seconds), nil
				}
				// For seconds, we display 4 significant digits.
				return fmt.Sprintf("%s%.4gs", sign, v), nil
			}
			prefix := ""
			for _, p := range []string{"m", "u", "n", "p", "f", "a", "z", "y"} {
				if math.Abs(v) >= 1 {
					break
				}
				prefix = p
				v *= 1000
			}
			return fmt.Sprintf("%.4g%ss", v, prefix), nil
		},

		// humanizePercentage converts given ratio value to a fraction of 100
		"humanizePercentage": func(i interface{}) (string, error) {
			v, err := toFloat64(i)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%.4g%%", v*100), nil
		},

		// humanizeTimestamp converts given timestamp to a human readable time equivalent
		"humanizeTimestamp": func(i interface{}) (string, error) {
			v, err := toFloat64(i)
			if err != nil {
				return "", err
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			t := timeFromUnixTimestamp(v).Time().UTC()
			return fmt.Sprint(t), nil
		},

		// toTime converts given timestamp to a time.Time.
		"toTime": func(i interface{}) (time.Time, error) {
			v, err := toFloat64(i)
			if err != nil {
				return time.Time{}, err
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return time.Time{}, fmt.Errorf("cannot convert %v to time.Time", v)
			}
			t := timeFromUnixTimestamp(v).Time().UTC()
			return t, nil
		},

		/* URLs */

		// externalURL returns value of `external.url` flag
		"externalURL": func() string {
			// externalURL function supposed to be substituted at FuncsWithExteralURL().
			// it is present here only for validation purposes, when there is no
			// provided datasource.
			//
			// return non-empty slice to pass validation with chained functions in template
			return ""
		},

		// pathPrefix returns a Path segment from the URL value in `external.url` flag
		"pathPrefix": func() string {
			// pathPrefix function supposed to be substituted at FuncsWithExteralURL().
			// it is present here only for validation purposes, when there is no
			// provided datasource.
			//
			// return non-empty slice to pass validation with chained functions in template
			return ""
		},

		// pathEscape escapes the string so it can be safely placed inside a URL path segment.
		//
		// See also queryEscape.
		"pathEscape": url.PathEscape,

		// queryEscape escapes the string so it can be safely placed inside a query arg in URL.
		//
		// See also queryEscape.
		"queryEscape": url.QueryEscape,

		// query executes the MetricsQL/PromQL query against
		// configured `datasource.url` address.
		// For example, {{ query "foo" | first | value }} will
		// execute "/api/v1/query?query=foo" request and will return
		// the first value in response.
		"query": func(_ string) ([]metric, error) {
			// query function supposed to be substituted at FuncsWithQuery().
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

		// strvalue returns metric name.
		"strvalue": func(m metric) string {
			return m.Labels["__name__"]
		},

		// sortByLabel sorts the given metrics by provided label key
		"sortByLabel": func(label string, metrics []metric) []metric {
			sort.SliceStable(metrics, func(i, j int) bool {
				return metrics[i].Labels[label] < metrics[j].Labels[label]
			})
			return metrics
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
		//
		// See also htmlEscape.
		"safeHtml": func(text string) htmlTpl.HTML {
			return htmlTpl.HTML(text)
		},
	}
}

// Time is the number of milliseconds since the epoch
// (1970-01-01 00:00 UTC) excluding leap seconds.
type Time int64

// timeFromUnixTimestamp returns the Time equivalent to t in unix timestamp.
func timeFromUnixTimestamp(t float64) Time {
	return Time(t * 1e3)
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

func toFloat64(v interface{}) (float64, error) {
	switch i := v.(type) {
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		return strconv.ParseFloat(i, 64)
	default:
		return 0, fmt.Errorf("unexpected value type %v", i)
	}
}
