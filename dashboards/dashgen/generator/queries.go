package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/dashboards/dashgen/parser"
)

// componentVersionPatterns maps component names to regex patterns for vm_app_version filtering.
// Note: Prometheus label matchers already anchor at start, so we don't use ^.
var componentVersionPatterns = map[string]string{
	"cluster":   "(vminsert|vmselect|vmstorage)-.*",
	"single":    "victoria-metrics-.*",
	"vmagent":   "vmagent-.*",
	"vmalert":   "vmalert-.*",
	"vmauth":    "vmauth-.*",
	"vmanomaly": "vmanomaly-.*",
	"unknown":   ".*", // Unknown alerts apply to all components
}

// svcNameRegex extracts service name from version label (e.g., "vmagent-20251204-..." -> "vmagent").
const svcNameRegex = `^(.+)-\\d{8}-.*`

// queryTemplate generates a PromQL query that returns the MINIMUM (worst) percentage
// of healthy instances over the selected time range.
// Returns:
//   - 100 when no instances fired the alert during the range (all healthy)
//   - 0-99 when some instances fired (shows worst percentage)
//   - No data when the alert is not applicable to the component
//
// Logic:
// 1. Count total instances per svc_name (from vm_app_version with version filter)
// 2. Count firing instances per svc_name (alert expr joined with vm_app_version)
// 3. Calculate: 100 * (total - firing) / total
// 4. Take min_over_time to show worst state in selected range
const queryTemplate = `min_over_time(
  (
    WITH (
      vm_svc = label_replace(
        vm_app_version{version=~"%s", version!~"(victoria-(logs|traces)|vl|vt).*", job=~"$job", instance=~"$instance"},
        "svc_name",
        "$1",
        "version",
        "%s"
      ),
      total = count by (svc_name) (vm_svc),
      firing_pod = count by (svc_name) (
        ((%s) > 0) * on(pod, instance, job) group_left(svc_name) vm_svc
      ),
      firing_inst = count by (svc_name) (
        ((%s) > 0) * on(instance, job) group_left(svc_name) vm_svc
      ),
      firing = (firing_pod or firing_inst or total * 0)
    )
    clamp_min(100 * (total - firing) / total, 0)
  )[$__range:]
)`

// NormalizeAlertQuery transforms an alert expression into a dashboard query
// that returns the health percentage per service name.
// Returns 100 when all instances are healthy, <100 when some are firing,
// or no data when not applicable to the component.
func NormalizeAlertQuery(rule parser.AlertRule) string {
	expr := strings.TrimSpace(rule.Expr)

	versionFilter := componentVersionPatterns[rule.Component]
	if versionFilter == "" {
		versionFilter = ".*" // Default to all if component not mapped
	}

	return fmt.Sprintf(queryTemplate, versionFilter, svcNameRegex, expr, expr)
}

// refIDReplacer removes characters that are invalid in Grafana refIds.
var refIDReplacer = strings.NewReplacer(
	" ", "",
	"-", "",
	"_", "",
	":", "",
	".", "",
)

// startsWithDigit checks if a string starts with a digit.
var startsWithDigit = regexp.MustCompile(`^\d`)

// GenerateRefID creates a valid Grafana refId from an alert name.
// Grafana refIds must be alphanumeric and start with a letter.
func GenerateRefID(alertName string) string {
	refID := refIDReplacer.Replace(alertName)

	if len(refID) == 0 || startsWithDigit.MatchString(refID) {
		refID = "Q" + refID
	}

	return refID
}
