package notifier

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// Alert the triggered alert
// TODO: Looks like alert name isn't unique
type Alert struct {
	// GroupID contains the ID of the parent rules group
	GroupID uint64
	// Name represents Alert name
	Name string
	// Labels is the list of label-value pairs attached to the Alert
	Labels map[string]string
	// Annotations is the list of annotations generated on Alert evaluation
	Annotations map[string]string
	// State represents the current state of the Alert
	State AlertState
	// Expr contains expression that was executed to generate the Alert
	Expr string
	// ActiveAt defines the moment of time when Alert has become active
	ActiveAt time.Time
	// Start defines the moment of time when Alert has become firing
	Start time.Time
	// End defines the moment of time when Alert supposed to expire
	End time.Time
	// ResolvedAt defines the moment when Alert was switched from Firing to Inactive
	ResolvedAt time.Time
	// LastSent defines the moment when Alert was sent last time
	LastSent time.Time
	// KeepFiringSince defines the moment when StateFiring was kept because of `keep_firing_for` instead of real alert
	KeepFiringSince time.Time
	// Value stores the value returned from evaluating expression from Expr field
	Value float64
	// ID is the unique identifier for the Alert
	ID uint64
	// Restored is true if Alert was restored after restart
	Restored bool
	// For defines for how long Alert needs to be active to become StateFiring
	For time.Duration
}

// AlertState type indicates the Alert state
type AlertState int

const (
	// StateInactive is the state of an alert that is neither firing nor pending.
	StateInactive AlertState = iota
	// StatePending is the state of an alert that has been active for less than
	// the configured threshold duration.
	StatePending
	// StateFiring is the state of an alert that has been active for longer than
	// the configured threshold duration.
	StateFiring
)

// String stringer for AlertState
func (as AlertState) String() string {
	switch as {
	case StateFiring:
		return "firing"
	case StatePending:
		return "pending"
	}
	return "inactive"
}

// ToTplData converts Alert to AlertTplData,
// which only exposes necessary fields for template.
func (a Alert) ToTplData() templates.AlertTplData {
	return templates.AlertTplData{
		Value:    a.Value,
		Labels:   a.Labels,
		Expr:     a.Expr,
		AlertID:  a.ID,
		GroupID:  a.GroupID,
		ActiveAt: a.ActiveAt,
		For:      a.For,
	}
}

func (a Alert) applyRelabelingIfNeeded(relabelCfg *promrelabel.ParsedConfigs) []prompbmarshal.Label {
	var labels []prompbmarshal.Label
	for k, v := range a.Labels {
		labels = append(labels, prompbmarshal.Label{
			Name:  promrelabel.SanitizeMetricName(k),
			Value: v,
		})
	}
	if relabelCfg != nil {
		labels = relabelCfg.Apply(labels, 0)
	}
	promrelabel.SortLabels(labels)
	return labels
}
