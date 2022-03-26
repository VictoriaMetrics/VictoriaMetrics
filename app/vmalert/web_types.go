package main

import (
	"time"
)

// APIAlert represents a notifier.AlertingRule state
// for WEB view
// https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type APIAlert struct {
	State       string            `json:"state"`
	Name        string            `json:"name"`
	Value       string            `json:"value"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations"`
	ActiveAt    time.Time         `json:"activeAt"`

	// Additional fields

	// ID is an unique Alert's ID within a group
	ID string `json:"id"`
	// RuleID is an unique Rule's ID within a group
	RuleID string `json:"rule_id"`
	// GroupID is an unique Group's ID
	GroupID string `json:"group_id"`
	// Expression contains the PromQL/MetricsQL expression
	// for Rule's evaluation
	Expression string `json:"expression"`
	// SourceLink contains a link to a system which should show
	// why Alert was generated
	SourceLink string `json:"source"`
	// Restored shows whether Alert's state was restored on restart
	Restored bool `json:"restored"`
}

// APIGroup represents Group for WEB view
// https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type APIGroup struct {
	// Name is the group name as present in the config
	Name string `json:"name"`
	// Rules contains both recording and alerting rules
	Rules []APIRule `json:"rules"`
	// Interval is the Group's evaluation interval in float seconds as present in the file.
	Interval float64 `json:"interval"`
	// LastEvaluation is the timestamp of the last time the Group was executed
	LastEvaluation time.Time `json:"lastEvaluation"`

	// Additional fields

	// Type shows the datasource type (prometheus or graphite) of the Group
	Type string `json:"type"`
	// ID is a unique Group ID
	ID string `json:"id"`
	// File contains a path to the file with Group's config
	File string `json:"file"`
	// Concurrency shows how many rules may be evaluated simultaneously
	Concurrency int `json:"concurrency"`
	// Params contains HTTP URL parameters added to each Rule's request
	Params []string `json:"params,omitempty"`
	// Labels is a set of label value pairs, that will be added to every rule.
	Labels map[string]string `json:"labels,omitempty"`
}

// GroupAlerts represents a group of alerts for WEB view
type GroupAlerts struct {
	Group  APIGroup
	Alerts []*APIAlert
}

// APIRule represents a Rule for WEB view
// see https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type APIRule struct {
	// State must be one of these under following scenarios
	//  "pending": at least 1 alert in the rule in pending state and no other alert in firing state.
	//  "firing": at least 1 alert in the rule in firing state.
	//  "inactive": no alert in the rule in firing or pending state.
	State string `json:"state"`
	Name  string `json:"name"`
	// Query represents Rule's `expression` field
	Query string `json:"query"`
	// Duration represents Rule's `for` field
	Duration    float64           `json:"duration"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	// LastError contains the error faced while executing the rule.
	LastError string `json:"lastError"`
	// EvaluationTime is the time taken to completely evaluate the rule in float seconds.
	EvaluationTime float64 `json:"evaluationTime"`
	// LastEvaluation is the timestamp of the last time the rule was executed
	LastEvaluation time.Time `json:"lastEvaluation"`
	// Alerts  is the list of all the alerts in this rule that are currently pending or firing
	Alerts []*APIAlert `json:"alerts,omitempty"`
	// Health is the health of rule evaluation.
	// It MUST be one of "ok", "err", "unknown"
	Health string `json:"health"`
	// Type of the rule: recording or alerting
	Type string `json:"type"`

	// Additional fields

	// Type of the rule: recording or alerting
	DatasourceType string `json:"datasourceType"`
	LastSamples    int    `json:"lastSamples"`
	// ID is an unique Alert's ID within a group
	ID string `json:"id"`
	// GroupID is an unique Group's ID
	GroupID string `json:"group_id"`
}
