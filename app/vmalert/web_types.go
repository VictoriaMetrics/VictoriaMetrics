package main

import (
	"time"
)

// APIAlert represents an notifier.AlertingRule state
// for WEB view
type APIAlert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	RuleID      string            `json:"rule_id"`
	GroupID     string            `json:"group_id"`
	Expression  string            `json:"expression"`
	State       string            `json:"state"`
	Value       string            `json:"value"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	ActiveAt    time.Time         `json:"activeAt"`
	SourceLink  string            `json:"source"`
	Restored    bool              `json:"restored"`
}

// APIGroup represents Group for WEB view
type APIGroup struct {
	Name           string             `json:"name"`
	Type           string             `json:"type"`
	ID             string             `json:"id"`
	File           string             `json:"file"`
	Interval       string             `json:"interval"`
	Concurrency    int                `json:"concurrency"`
	Params         []string           `json:"params"`
	Labels         map[string]string  `json:"labels,omitempty"`
	AlertingRules  []APIAlertingRule  `json:"alerting_rules"`
	RecordingRules []APIRecordingRule `json:"recording_rules"`
}

// APIAlertingRule represents AlertingRule for WEB view
type APIAlertingRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	GroupID     string            `json:"group_id"`
	Expression  string            `json:"expression"`
	For         string            `json:"for"`
	LastError   string            `json:"last_error"`
	LastSamples int               `json:"last_samples"`
	LastExec    time.Time         `json:"last_exec"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// APIRecordingRule represents RecordingRule for WEB view
type APIRecordingRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	GroupID     string            `json:"group_id"`
	Expression  string            `json:"expression"`
	LastError   string            `json:"last_error"`
	LastSamples int               `json:"last_samples"`
	LastExec    time.Time         `json:"last_exec"`
	Labels      map[string]string `json:"labels"`
}

// GroupAlerts represents a group of alerts for WEB view
type GroupAlerts struct {
	Group  APIGroup
	Alerts []*APIAlert
}
