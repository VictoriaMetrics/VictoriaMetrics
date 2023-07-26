package unittest

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// AlertTestCase holds alert_rule_test cases defined in test file
type AlertTestCase struct {
	EvalTime  *promutils.Duration `yaml:"eval_time"`
	GroupName string              `yaml:"groupname"`
	Alertname string              `yaml:"alertname"`
	ExpAlerts []ExpAlert          `yaml:"exp_alerts"`
}

// ExpAlert holds exp_alerts defined in test file
type ExpAlert struct {
	ExpLabels      map[string]string `yaml:"exp_labels"`
	ExpAnnotations map[string]string `yaml:"exp_annotations"`
}
