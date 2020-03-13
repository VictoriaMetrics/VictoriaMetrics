package provider

import (
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
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

// AlertsFromMetrics converts metrics to alerts by alert Rule
func AlertsFromMetrics(metrics []datasource.Metric, group string, rule config.Rule) []Alert {
	alerts := make([]Alert, 0, len(metrics))
	for i, m := range metrics {
		a := Alert{
			Group:  group,
			Name:   rule.Name,
			Labels: metrics[i].Labels,
			// todo eval template in annotations
			Annotations: rule.Annotations,
			Start:       time.Unix(m.Timestamp, 0),
		}
		for k, v := range rule.Labels {
			a.Labels = append(a.Labels, datasource.Label{
				Name:  k,
				Value: v,
			})
		}
		a.Labels = removeDuplicated(a.Labels)
		alerts = append(alerts, a)
	}
	return alerts
}

func removeDuplicated(l []datasource.Label) []datasource.Label {
	sort.Slice(l, func(i, j int) bool {
		return l[i].Name < l[j].Name
	})
	j := 0
	for i := 1; i < len(l); i++ {
		if l[j] == l[i] {
			continue
		}
		j++
		l[j] = l[i]
	}
	return l[:j+1]
}
