package unittest

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// alertTestCase holds alert_rule_test cases defined in test file
type alertTestCase struct {
	EvalTime  *promutils.Duration `yaml:"eval_time"`
	GroupName string              `yaml:"groupname"`
	Alertname string              `yaml:"alertname"`
	ExpAlerts []expAlert          `yaml:"exp_alerts"`
}

// expAlert holds exp_alerts defined in test file
type expAlert struct {
	ExpLabels      map[string]string `yaml:"exp_labels"`
	ExpAnnotations map[string]string `yaml:"exp_annotations"`
}
