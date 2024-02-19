package main

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
)

const (
	// ParamGroupID is group id key in url parameter
	paramGroupID = "group_id"
	// ParamAlertID is alert id key in url parameter
	paramAlertID = "alert_id"
	// ParamRuleID is rule id key in url parameter
	paramRuleID = "rule_id"
)

// apiAlert represents a notifier.AlertingRule state
// for WEB view
// https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type apiAlert struct {
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
	// Stabilizing shows when firing state is kept because of
	// `keep_firing_for` instead of real alert
	Stabilizing bool `json:"stabilizing"`
}

// WebLink returns a link to the alert which can be used in UI.
func (aa *apiAlert) WebLink() string {
	return fmt.Sprintf("alert?%s=%s&%s=%s",
		paramGroupID, aa.GroupID, paramAlertID, aa.ID)
}

// APILink returns a link to the alert's JSON representation.
func (aa *apiAlert) APILink() string {
	return fmt.Sprintf("api/v1/alert?%s=%s&%s=%s",
		paramGroupID, aa.GroupID, paramAlertID, aa.ID)
}

// apiGroup represents Group for web view
// https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type apiGroup struct {
	// Name is the group name as present in the config
	Name string `json:"name"`
	// Rules contains both recording and alerting rules
	Rules []apiRule `json:"rules"`
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
	// Headers contains HTTP headers added to each Rule's request
	Headers []string `json:"headers,omitempty"`
	// NotifierHeaders contains HTTP headers added to each alert request which will send to notifier
	NotifierHeaders []string `json:"notifier_headers,omitempty"`
	// Labels is a set of label value pairs, that will be added to every rule.
	Labels map[string]string `json:"labels,omitempty"`
	// EvalOffset Group will be evaluated at the exact time offset on the range of [0...evaluationInterval]
	EvalOffset float64 `json:"eval_offset,omitempty"`
	// EvalDelay will adjust the `time` parameter of rule evaluation requests to compensate intentional query delay from datasource.
	EvalDelay float64 `json:"eval_delay,omitempty"`
}

// groupAlerts represents a group of alerts for WEB view
type groupAlerts struct {
	Group  apiGroup
	Alerts []*apiAlert
}

// apiRule represents a Rule for web view
// see https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type apiRule struct {
	// State must be one of these under following scenarios
	//  "pending": at least 1 alert in the rule in pending state and no other alert in firing ruleState.
	//  "firing": at least 1 alert in the rule in firing state.
	//  "inactive": no alert in the rule in firing or pending state.
	State string `json:"state"`
	Name  string `json:"name"`
	// Query represents Rule's `expression` field
	Query string `json:"query"`
	// Duration represents Rule's `for` field
	Duration float64 `json:"duration"`
	// Alert will continue firing for this long even when the alerting expression no longer has results.
	KeepFiringFor float64           `json:"keep_firing_for"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	// LastError contains the error faced while executing the rule.
	LastError string `json:"lastError"`
	// EvaluationTime is the time taken to completely evaluate the rule in float seconds.
	EvaluationTime float64 `json:"evaluationTime"`
	// LastEvaluation is the timestamp of the last time the rule was executed
	LastEvaluation time.Time `json:"lastEvaluation"`
	// Alerts  is the list of all the alerts in this rule that are currently pending or firing
	Alerts []*apiAlert `json:"alerts,omitempty"`
	// Health is the health of rule evaluation.
	// It MUST be one of "ok", "err", "unknown"
	Health string `json:"health"`
	// Type of the rule: recording or alerting
	Type string `json:"type"`

	// Additional fields

	// DatasourceType of the rule: prometheus or graphite
	DatasourceType string `json:"datasourceType"`
	// LastSamples stores the amount of data samples received on last evaluation
	LastSamples int `json:"lastSamples"`
	// LastSeriesFetched stores the amount of time series fetched by datasource
	// during the last evaluation
	LastSeriesFetched *int `json:"lastSeriesFetched,omitempty"`

	// ID is a unique Alert's ID within a group
	ID string `json:"id"`
	// GroupID is an unique Group's ID
	GroupID string `json:"group_id"`
	// GroupName is Group name rule belong to
	GroupName string `json:"group_name"`
	// File is file name where rule is defined
	File string `json:"file"`
	// Debug shows whether debug mode is enabled
	Debug bool `json:"debug"`

	// MaxUpdates is the max number of recorded ruleStateEntry objects
	MaxUpdates int `json:"max_updates_entries"`
	// Updates contains the ordered list of recorded ruleStateEntry objects
	Updates []rule.StateEntry `json:"-"`
}

// apiRuleWithUpdates represents apiRule but with extra fields for marshalling
type apiRuleWithUpdates struct {
	apiRule
	// Updates contains the ordered list of recorded ruleStateEntry objects
	StateUpdates []rule.StateEntry `json:"updates,omitempty"`
}

// APILink returns a link to the rule's JSON representation.
func (ar apiRule) APILink() string {
	return fmt.Sprintf("api/v1/rule?%s=%s&%s=%s",
		paramGroupID, ar.GroupID, paramRuleID, ar.ID)
}

// WebLink returns a link to the alert which can be used in UI.
func (ar apiRule) WebLink() string {
	return fmt.Sprintf("rule?%s=%s&%s=%s",
		paramGroupID, ar.GroupID, paramRuleID, ar.ID)
}

func ruleToAPI(r interface{}) apiRule {
	if ar, ok := r.(*rule.AlertingRule); ok {
		return alertingToAPI(ar)
	}
	if rr, ok := r.(*rule.RecordingRule); ok {
		return recordingToAPI(rr)
	}
	return apiRule{}
}

const (
	ruleTypeRecording = "recording"
	ruleTypeAlerting  = "alerting"
)

func recordingToAPI(rr *rule.RecordingRule) apiRule {
	lastState := rule.GetLastEntry(rr)
	r := apiRule{
		Type:              ruleTypeRecording,
		DatasourceType:    rr.Type.String(),
		Name:              rr.Name,
		Query:             rr.Expr,
		Labels:            rr.Labels,
		LastEvaluation:    lastState.Time,
		EvaluationTime:    lastState.Duration.Seconds(),
		Health:            "ok",
		LastSamples:       lastState.Samples,
		LastSeriesFetched: lastState.SeriesFetched,
		MaxUpdates:        rule.GetRuleStateSize(rr),
		Updates:           rule.GetAllRuleState(rr),

		// encode as strings to avoid rounding
		ID:      fmt.Sprintf("%d", rr.ID()),
		GroupID: fmt.Sprintf("%d", rr.GroupID),
	}
	if lastState.Err != nil {
		r.LastError = lastState.Err.Error()
		r.Health = "err"
	}
	return r
}

// alertingToAPI returns Rule representation in form of apiRule
func alertingToAPI(ar *rule.AlertingRule) apiRule {
	lastState := rule.GetLastEntry(ar)
	r := apiRule{
		Type:              ruleTypeAlerting,
		DatasourceType:    ar.Type.String(),
		Name:              ar.Name,
		Query:             ar.Expr,
		Duration:          ar.For.Seconds(),
		KeepFiringFor:     ar.KeepFiringFor.Seconds(),
		Labels:            ar.Labels,
		Annotations:       ar.Annotations,
		LastEvaluation:    lastState.Time,
		EvaluationTime:    lastState.Duration.Seconds(),
		Health:            "ok",
		State:             "inactive",
		Alerts:            ruleToAPIAlert(ar),
		LastSamples:       lastState.Samples,
		LastSeriesFetched: lastState.SeriesFetched,
		MaxUpdates:        rule.GetRuleStateSize(ar),
		Updates:           rule.GetAllRuleState(ar),
		Debug:             ar.Debug,

		// encode as strings to avoid rounding in JSON
		ID:        fmt.Sprintf("%d", ar.ID()),
		GroupID:   fmt.Sprintf("%d", ar.GroupID),
		GroupName: ar.GroupName,
		File:      ar.File,
	}
	if lastState.Err != nil {
		r.LastError = lastState.Err.Error()
		r.Health = "err"
	}
	// satisfy apiRule.State logic
	if len(r.Alerts) > 0 {
		r.State = notifier.StatePending.String()
		stateFiring := notifier.StateFiring.String()
		for _, a := range r.Alerts {
			if a.State == stateFiring {
				r.State = stateFiring
				break
			}
		}
	}
	return r
}

// ruleToAPIAlert generates list of apiAlert objects from existing alerts
func ruleToAPIAlert(ar *rule.AlertingRule) []*apiAlert {
	var alerts []*apiAlert
	for _, a := range ar.GetAlerts() {
		if a.State == notifier.StateInactive {
			continue
		}
		alerts = append(alerts, newAlertAPI(ar, a))
	}
	return alerts
}

// alertToAPI generates apiAlert object from alert by its id(hash)
func alertToAPI(ar *rule.AlertingRule, id uint64) *apiAlert {
	a := ar.GetAlert(id)
	if a == nil {
		return nil
	}
	return newAlertAPI(ar, a)
}

// NewAlertAPI creates apiAlert for notifier.Alert
func newAlertAPI(ar *rule.AlertingRule, a *notifier.Alert) *apiAlert {
	aa := &apiAlert{
		// encode as strings to avoid rounding
		ID:      fmt.Sprintf("%d", a.ID),
		GroupID: fmt.Sprintf("%d", a.GroupID),
		RuleID:  fmt.Sprintf("%d", ar.RuleID),

		Name:        a.Name,
		Expression:  ar.Expr,
		Labels:      a.Labels,
		Annotations: a.Annotations,
		State:       a.State.String(),
		ActiveAt:    a.ActiveAt,
		Restored:    a.Restored,
		Value:       strconv.FormatFloat(a.Value, 'f', -1, 32),
	}
	if alertURLGeneratorFn != nil {
		aa.SourceLink = alertURLGeneratorFn(*a)
	}
	if a.State == notifier.StateFiring && !a.KeepFiringSince.IsZero() {
		aa.Stabilizing = true
	}
	return aa
}

func groupToAPI(g *rule.Group) apiGroup {
	g = g.DeepCopy()
	ag := apiGroup{
		// encode as string to avoid rounding
		ID: fmt.Sprintf("%d", g.ID()),

		Name:            g.Name,
		Type:            g.Type.String(),
		File:            g.File,
		Interval:        g.Interval.Seconds(),
		LastEvaluation:  g.LastEvaluation,
		Concurrency:     g.Concurrency,
		Params:          urlValuesToStrings(g.Params),
		Headers:         headersToStrings(g.Headers),
		NotifierHeaders: headersToStrings(g.NotifierHeaders),

		Labels: g.Labels,
	}
	if g.EvalOffset != nil {
		ag.EvalOffset = g.EvalOffset.Seconds()
	}
	if g.EvalDelay != nil {
		ag.EvalDelay = g.EvalDelay.Seconds()
	}
	ag.Rules = make([]apiRule, 0)
	for _, r := range g.Rules {
		ag.Rules = append(ag.Rules, ruleToAPI(r))
	}
	return ag
}

func urlValuesToStrings(values url.Values) []string {
	if len(values) < 1 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var res []string
	for _, k := range keys {
		params := values[k]
		for _, v := range params {
			res = append(res, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return res
}

func headersToStrings(headers map[string]string) []string {
	if len(headers) < 1 {
		return nil
	}

	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var res []string
	for _, k := range keys {
		v := headers[k]
		res = append(res, fmt.Sprintf("%s: %s", k, v))
	}

	return res
}
