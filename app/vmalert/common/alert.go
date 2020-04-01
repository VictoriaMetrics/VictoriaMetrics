package common

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Alert the triggered alert
type Alert struct {
	Group       string
	Name        string
	Labels      []datasource.Label
	Annotations map[string]string

	Start time.Time
	End   time.Time
	Value float64
}

type alertTplData struct {
	Labels         map[string]string
	ExternalLabels map[string]string
	Value          float64
}

const tplHeader = `{{ $value := .Value }}{{ $labels := .Labels }}{{ $externalLabels := .ExternalLabels }}`

// AlertsFromMetrics converts metrics to alerts by alert Rule
func AlertsFromMetrics(metrics []datasource.Metric, group string, rule Rule, start, end time.Time) []Alert {
	alerts := make([]Alert, 0, len(metrics))
	var err error
	for i, m := range metrics {
		a := Alert{
			Group: group,
			Name:  rule.Name,
			Start: start,
			End:   end,
			Value: m.Value,
		}
		tplData := alertTplData{Value: m.Value, ExternalLabels: make(map[string]string)}
		tplData.Labels, a.Labels = mergeLabels(metrics[i].Labels, rule.Labels)
		a.Annotations, err = templateAnnotations(rule.Annotations, tplHeader, tplData)
		if err != nil {
			logger.Errorf("%s", err)
		}
		alerts = append(alerts, a)
	}
	return alerts
}

func mergeLabels(ml []datasource.Label, rl map[string]string) (map[string]string, []datasource.Label) {
	set := make(map[string]string, len(ml)+len(rl))
	sl := append([]datasource.Label(nil), ml...)
	for _, i := range ml {
		set[i.Name] = i.Value
	}
	for name, value := range rl {
		if _, ok := set[name]; ok {
			continue
		}
		set[name] = value
		sl = append(sl, datasource.Label{
			Name:  name,
			Value: value,
		})
	}
	return set, sl
}

func templateAnnotations(annotations map[string]string, header string, data alertTplData) (map[string]string, error) {
	var builder strings.Builder
	var buf bytes.Buffer
	eg := errGroup{}
	r := make(map[string]string, len(annotations))
	for key, text := range annotations {
		r[key] = text
		buf.Reset()
		builder.Reset()
		builder.Grow(len(header) + len(text))
		builder.WriteString(header)
		builder.WriteString(text)
		if err := templateAnnotation(&buf, builder.String(), data); err != nil {
			eg.errs = append(eg.errs, fmt.Sprintf("key %s, template %s:%s", key, text, err))
			continue
		}
		r[key] = buf.String()
	}
	return r, eg.err()
}

// ValidateAnnotations validate annotations for possible template error, uses empty data for template population
func ValidateAnnotations(annotations map[string]string) error {
	_, err := templateAnnotations(annotations, tplHeader, alertTplData{
		Labels:         map[string]string{},
		ExternalLabels: map[string]string{},
		Value:          0,
	})
	return err
}

func templateAnnotation(dst io.Writer, text string, data alertTplData) error {
	tpl, err := template.New("").Funcs(tmplFunc).Option("missingkey=zero").Parse(text)
	if err != nil {
		return fmt.Errorf("error parsing annotation:%w", err)
	}
	if err = tpl.Execute(dst, data); err != nil {
		return fmt.Errorf("error evaluating annotation template:%w", err)
	}
	return nil
}

type errGroup struct {
	errs []string
}

func (eg *errGroup) err() error {
	if eg == nil || len(eg.errs) == 0 {
		return nil
	}
	return eg
}

func (eg *errGroup) Error() string {
	return fmt.Sprintf("errors:%s", strings.Join(eg.errs, "\n"))
}
