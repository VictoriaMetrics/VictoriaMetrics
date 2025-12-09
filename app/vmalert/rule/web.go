package rule

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

const (
	// ParamGroupID is group id key in url parameter
	ParamGroupID = "group_id"
	// ParamAlertID is alert id key in url parameter
	ParamAlertID = "alert_id"
	// ParamRuleID is rule id key in url parameter
	ParamRuleID = "rule_id"

	// TypeRecording is a RecordingRule type
	TypeRecording = "recording"
	// TypeAlerting is an AlertingRule type
	TypeAlerting = "alerting"
)

// ApiGroup represents a Group for web view
type ApiGroup struct {
	// Name is the group name as present in the config
	Name string `json:"name"`
	// Rules contains both recording and alerting rules
	Rules []ApiRule `json:"rules"`
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
	// States represents counts per each rule state
	States map[string]int `json:"states"`
}

// APILink returns a link to the group's JSON representation.
func (ag *ApiGroup) APILink() string {
	return fmt.Sprintf("api/v1/group?%s=%s", ParamGroupID, ag.ID)
}

// GroupAlerts represents a Group with its Alerts for web view
type GroupAlerts struct {
	Group  *ApiGroup
	Alerts []*ApiAlert
}

// ApiRule represents a Rule for web view
// see https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type ApiRule struct {
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
	Alerts []*ApiAlert `json:"alerts,omitempty"`
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
	Updates []StateEntry `json:"-"`
}

func (r *ApiRule) isNoMatch() bool {
	return r.LastSamples == 0 && r.LastSeriesFetched != nil && *r.LastSeriesFetched == 0
}

// ApiAlert represents a notifier.AlertingRule state
// for WEB view
// https://github.com/prometheus/compliance/blob/main/alert_generator/specification.md#get-apiv1rules
type ApiAlert struct {
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
func (aa *ApiAlert) WebLink() string {
	return fmt.Sprintf("alert?%s=%s&%s=%s",
		ParamGroupID, aa.GroupID, ParamAlertID, aa.ID)
}

// APILink returns a link to the alert's JSON representation.
func (aa *ApiAlert) APILink() string {
	return fmt.Sprintf("api/v1/alert?%s=%s&%s=%s",
		ParamGroupID, aa.GroupID, ParamAlertID, aa.ID)
}

// ApiRuleWithUpdates represents ApiRule but with extra fields for marshalling
type ApiRuleWithUpdates struct {
	ApiRule
	// Updates contains the ordered list of recorded ruleStateEntry objects
	StateUpdates []StateEntry `json:"updates,omitempty"`
}

// APILink returns a link to the rule's JSON representation.
func (ar ApiRule) APILink() string {
	return fmt.Sprintf("api/v1/rule?%s=%s&%s=%s",
		ParamGroupID, ar.GroupID, ParamRuleID, ar.ID)
}

// WebLink returns a link to the alert which can be used in UI.
func (ar ApiRule) WebLink() string {
	return fmt.Sprintf("rule?%s=%s&%s=%s",
		ParamGroupID, ar.GroupID, ParamRuleID, ar.ID)
}

// AlertsToAPI returns list of ApiAlert objects from existing alerts
func (ar *AlertingRule) AlertsToAPI() []*ApiAlert {
	var alerts []*ApiAlert
	for _, a := range ar.GetAlerts() {
		if a.State == notifier.StateInactive {
			continue
		}
		alerts = append(alerts, NewAlertAPI(ar, a))
	}
	return alerts
}

// NewAlertAPI creates apiAlert for notifier.Alert
func NewAlertAPI(ar *AlertingRule, a *notifier.Alert) *ApiAlert {
	aa := &ApiAlert{
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
	if notifier.AlertURLGeneratorFn != nil {
		aa.SourceLink = notifier.AlertURLGeneratorFn(*a)
	}
	if a.State == notifier.StateFiring && !a.KeepFiringSince.IsZero() {
		aa.Stabilizing = true
	}
	return aa
}

func (r *ApiRule) ExtendState() {
	if len(r.Alerts) > 0 {
		return
	}
	if r.State == "" {
		r.State = "ok"
	}
	if r.Health != "ok" {
		r.State = "unhealthy"
	} else if r.isNoMatch() {
		r.State = "nomatch"
	}
}

// ToAPI returns ApiGroup representation of g
func (g *Group) ToAPI() *ApiGroup {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ag := ApiGroup{
		// encode as string to avoid rounding
		ID:              strconv.FormatUint(g.GetID(), 10),
		Name:            g.Name,
		Type:            g.Type.String(),
		File:            g.File,
		Interval:        g.Interval.Seconds(),
		LastEvaluation:  g.LastEvaluation,
		Concurrency:     g.Concurrency,
		Params:          urlValuesToStrings(g.Params),
		Headers:         headersToStrings(g.Headers),
		NotifierHeaders: headersToStrings(g.NotifierHeaders),
		Labels:          g.Labels,
		States:          make(map[string]int),
	}
	if g.EvalOffset != nil {
		ag.EvalOffset = g.EvalOffset.Seconds()
	}
	if g.EvalDelay != nil {
		ag.EvalDelay = g.EvalDelay.Seconds()
	}
	ag.Rules = make([]ApiRule, 0, len(g.Rules))
	for _, r := range g.Rules {
		ar := r.ToAPI()
		ag.Rules = append(ag.Rules, ar)
	}
	return &ag
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
