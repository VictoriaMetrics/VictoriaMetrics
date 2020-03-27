package provider

import (
	"bytes"
	"strings"
	"text/template"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// AlertProvider is common interface for alert manager provider
type AlertProvider interface {
	Send(alerts []Alert) error
}

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
func AlertsFromMetrics(metrics []datasource.Metric, group string, rule config.Rule, start, end time.Time) []Alert {
	alerts := make([]Alert, 0, len(metrics))
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
		a.Annotations = templateAnnotations(rule.Annotations, tplHeader, tplData)
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

func templateAnnotations(annotations map[string]string, header string, data alertTplData) map[string]string {
	var builder strings.Builder
	var buf bytes.Buffer
	r := make(map[string]string, len(annotations))
	for key, text := range annotations {
		r[key] = text
		buf.Reset()
		builder.Reset()
		builder.Grow(len(header) + len(text))
		builder.WriteString(header)
		builder.WriteString(text)
		// todo add template helper func from Prometheus
		tpl, err := template.New("").Option("missingkey=zero").Parse(builder.String())
		if err != nil {
			logger.Errorf("error parsing annotation template %q for %q:%s", text, key, err)
			continue
		}
		if err = tpl.Execute(&buf, data); err != nil {
			logger.Errorf("error evaluating annotation template %s for %s:%s", text, key, err)
			continue
		}
		r[key] = buf.String()
	}
	return r
}
