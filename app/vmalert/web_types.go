package main

import (
	"time"
)

// APIAlert represents an notifier.AlertingRule state
// for WEB view
type APIAlert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	GroupID     string            `json:"group_id"`
	Expression  string            `json:"expression"`
	State       string            `json:"state"`
	Value       string            `json:"value"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	ActiveAt    time.Time         `json:"activeAt"`
}

// APIGroup represents Group for WEB view
type APIGroup struct {
	Name           string             `json:"name"`
	ID             string             `json:"id"`
	File           string             `json:"file"`
	Interval       string             `json:"interval"`
	Concurrency    int                `json:"concurrency"`
	AlertingRules  []APIAlertingRule  `json:"alerting_rules"`
	RecordingRules []APIRecordingRule `json:"recording_rules"`
}

// APIAlertingRule represents AlertingRule for WEB view
type APIAlertingRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	GroupID     string            `json:"group_id"`
	Expression  string            `json:"expression"`
	For         string            `json:"for"`
	LastError   string            `json:"last_error"`
	LastExec    time.Time         `json:"last_exec"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// APIRecordingRule represents RecordingRule for WEB view
type APIRecordingRule struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	GroupID    string            `json:"group_id"`
	Expression string            `json:"expression"`
	LastError  string            `json:"last_error"`
	LastExec   time.Time         `json:"last_exec"`
	Labels     map[string]string `json:"labels"`
}
