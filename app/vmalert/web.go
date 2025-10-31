package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/tpl"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var reloadAuthKey = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*")

var (
	apiLinks = [][2]string{
		// api links are relative since they can be used by external clients,
		// such as Grafana, and proxied via vmselect.
		{"api/v1/rules", "list all loaded groups and rules"},
		{"api/v1/alerts", "list all active alerts"},
		{"api/v1/notifiers", "list all notifiers"},
		{fmt.Sprintf("api/v1/alert?%s=<int>&%s=<int>", rule.ParamGroupID, rule.ParamAlertID), "get alert status by group and alert ID"},
		{fmt.Sprintf("api/v1/rule?%s=<int>&%s=<int>", rule.ParamGroupID, rule.ParamRuleID), "get rule status by group and rule ID"},
		{fmt.Sprintf("api/v1/group?%s=<int>", rule.ParamGroupID), "get group status by group ID"},
	}
	systemLinks = [][2]string{
		{"vmalert/groups", "UI"},
		{"flags", "command-line flags"},
		{"metrics", "list of application metrics"},
		{"-/reload", "reload configuration"},
	}
	navItems = []tpl.NavItem{
		{Name: "vmalert", URL: "../vmalert", Icon: "vm"},
		{Name: "Groups", URL: "groups"},
		{Name: "Alerts", URL: "alerts"},
		{Name: "Notifiers", URL: "notifiers"},
		{Name: "Docs", URL: "https://docs.victoriametrics.com/victoriametrics/vmalert/"},
	}
	ruleTypeMap = map[string]string{
		"alert":  rule.TypeAlerting,
		"record": rule.TypeRecording,
	}
)

type requestHandler struct {
	m *manager
}

var (
	//go:embed static
	staticFiles   embed.FS
	staticHandler = http.FileServer(http.FS(staticFiles))
	staticServer  = http.StripPrefix("/vmalert", staticHandler)
)

func (rh *requestHandler) handler(w http.ResponseWriter, r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/vmalert/static") {
		staticServer.ServeHTTP(w, r)
		return true
	}

	switch r.URL.Path {
	case "/", "/vmalert", "/vmalert/":
		if r.Method != http.MethodGet {
			httpserver.Errorf(w, r, "path %q supports only GET method", r.URL.Path)
			return false
		}
		WriteWelcome(w, r)
		return true
	case "/vmalert/alerts":
		WriteListAlerts(w, r, rh.groupAlerts())
		return true
	case "/vmalert/alert":
		alert, err := rh.getAlert(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		WriteAlert(w, r, alert)
		return true
	case "/vmalert/rule":
		rule, err := rh.getRule(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		WriteRuleDetails(w, r, rule)
		return true
	case "/vmalert/groups":
		rf, err := newRulesFilter(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		data, _ := rh.groups(rf)
		WriteListGroups(w, r, data, rf.filter)
		return true
	case "/vmalert/notifiers":
		WriteListTargets(w, r, notifier.GetTargets())
		return true

	// special cases for Grafana requests,
	// served without `vmalert` prefix:
	case "/rules":
		// Grafana makes an extra request to `/rules`
		// handler in addition to `/api/v1/rules` calls in alerts UI
		var data []*rule.ApiGroup
		rf, err := newRulesFilter(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		data, _ = rh.groups(rf)
		WriteListGroups(w, r, data, rf.filter)
		return true

	case "/vmalert/api/v1/notifiers", "/api/v1/notifiers":
		data, err := rh.listNotifiers()
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/rules", "/api/v1/rules":
		// path used by Grafana for ng alerting
		var data []byte
		rf, err := newRulesFilter(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		data, err = rh.listGroups(rf)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true

	case "/vmalert/api/v1/alerts", "/api/v1/alerts":
		// path used by Grafana for ng alerting
		rf, err := newRulesFilter(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		data, err := rh.listAlerts(rf)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/alert", "/api/v1/alert":
		alert, err := rh.getAlert(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		data, err := json.Marshal(alert)
		if err != nil {
			httpserver.Errorf(w, r, "failed to marshal alert: %s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/rule", "/api/v1/rule":
		apiRule, err := rh.getRule(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		rwu := rule.ApiRuleWithUpdates{
			ApiRule:      apiRule,
			StateUpdates: apiRule.Updates,
		}
		data, err := json.Marshal(rwu)
		if err != nil {
			httpserver.Errorf(w, r, "failed to marshal rule: %s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/group", "/api/v1/group":
		group, err := rh.getGroup(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		data, err := json.Marshal(group)
		if err != nil {
			httpserver.Errorf(w, r, "failed to marshal group: %s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/-/reload":
		if !httpserver.CheckAuthFlag(w, r, reloadAuthKey) {
			return true
		}
		logger.Infof("api config reload was called, sending sighup")
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true

	default:
		return false
	}
}

func (rh *requestHandler) getGroup(r *http.Request) (*rule.ApiGroup, error) {
	groupID, err := strconv.ParseUint(r.FormValue(rule.ParamGroupID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %w", rule.ParamGroupID, err)
	}
	obj, err := rh.m.groupAPI(groupID)
	if err != nil {
		return nil, errResponse(err, http.StatusNotFound)
	}
	return obj, nil
}

func (rh *requestHandler) getRule(r *http.Request) (rule.ApiRule, error) {
	groupID, err := strconv.ParseUint(r.FormValue(rule.ParamGroupID), 10, 64)
	if err != nil {
		return rule.ApiRule{}, fmt.Errorf("failed to read %q param: %w", rule.ParamGroupID, err)
	}
	ruleID, err := strconv.ParseUint(r.FormValue(rule.ParamRuleID), 10, 64)
	if err != nil {
		return rule.ApiRule{}, fmt.Errorf("failed to read %q param: %w", rule.ParamRuleID, err)
	}
	obj, err := rh.m.ruleAPI(groupID, ruleID)
	if err != nil {
		return rule.ApiRule{}, errResponse(err, http.StatusNotFound)
	}
	return obj, nil
}

func (rh *requestHandler) getAlert(r *http.Request) (*rule.ApiAlert, error) {
	groupID, err := strconv.ParseUint(r.FormValue(rule.ParamGroupID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %w", rule.ParamGroupID, err)
	}
	alertID, err := strconv.ParseUint(r.FormValue(rule.ParamAlertID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %w", rule.ParamAlertID, err)
	}
	a, err := rh.m.alertAPI(groupID, alertID)
	if err != nil {
		return nil, errResponse(err, http.StatusNotFound)
	}
	return a, nil
}

type listGroupsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups         []*rule.apiGroup `json:"groups"`
		GroupNextToken string           `json:"groupNextToken,omitempty"`
	} `json:"data"`
}

// see https://prometheus.io/docs/prometheus/latest/querying/api/#rules
type rulesFilter struct {
	files         []string
	groupNames    []string
	ruleNames     []string
	ruleType      string
	excludeAlerts bool
	filter        string
	dsType        config.Type
	maxGroups     int
	nextToken     string
}

func newRulesFilter(r *http.Request) (*rulesFilter, error) {
	rf := &rulesFilter{}
	query := r.URL.Query()

	ruleTypeParam := query.Get("type")
	if len(ruleTypeParam) > 0 {
		if ruleType, ok := ruleTypeMap[ruleTypeParam]; ok {
			rf.ruleType = ruleType
		} else {
			return nil, errResponse(fmt.Errorf(`invalid parameter "type": not supported value %q`, ruleTypeParam), http.StatusBadRequest)
		}
	}

	dsType := query.Get("datasource_type")
	if len(dsType) > 0 {
		if config.SupportedType(dsType) {
			rf.dsType = config.NewRawType(dsType)
		} else {
			return nil, errResponse(fmt.Errorf(`invalid parameter "datasource_type": not supported value %q`, dsType), http.StatusBadRequest)
		}
	}

	filter := strings.ToLower(query.Get("filter"))
	if len(filter) > 0 {
		if filter == "nomatch" || filter == "unhealthy" {
			rf.filter = filter
		} else {
			return nil, errResponse(fmt.Errorf(`invalid parameter "filter": not supported value %q`, filter), http.StatusBadRequest)
		}
	}

	rf.excludeAlerts = httputil.GetBool(r, "exclude_alerts")
	rf.ruleNames = append([]string{}, r.Form["rule_name[]"]...)
	rf.groupNames = append([]string{}, r.Form["rule_group[]"]...)
	rf.files = append([]string{}, r.Form["file[]"]...)

	rf.nextToken = r.URL.Query().Get("group_next_token")
	maxGroups := r.URL.Query().Get("group_limit")
	if rf.nextToken != "" && maxGroups == "" {
		return nil, errors.New("group_limit needs to be present in order to paginate over the groups")
	}
	if maxGroups != "" {
		mgs, err := strconv.ParseInt(maxGroups, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("group_limit needs to be a valid number: %w", err)
		}
		if mgs <= 0 {
			return nil, errors.New("group_limit needs to be greater than 0")
		}
		rf.maxGroups = int(mgs)
	}
	return rf, nil
}

func (rf *rulesFilter) matchesGroup(group *rule.Group) bool {
	if len(rf.groupNames) > 0 && !slices.Contains(rf.groupNames, group.Name) {
		return false
	}
	if len(rf.files) > 0 && !slices.Contains(rf.files, group.File) {
		return false
	}
	if len(rf.dsType.Name) > 0 && rf.dsType.String() != group.Type.String() {
		return false
	}
	return true
}

func (rh *requestHandler) groups(rf *rulesFilter) ([]*rule.ApiGroup, string) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	var (
		foundToken   bool
		grpNextToken string
	)
	groups := make([]*rule.ApiGroup, 0)
	for _, group := range rh.m.groups {
		if rf.maxGroups > 0 && rf.nextToken != "" && !foundToken {
			if rf.nextToken != strconv.Itoa(int(group.GetID())) {
				continue
			}
			foundToken = true
		}
		if !rf.matchesGroup(group) {
			continue
		}
		g := group.ToAPI()
		// the returned list should always be non-nil
		// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221
		filteredRules := make([]rule.ApiRule, 0)
		for _, rule := range g.Rules {
			if rf.ruleType != "" && rf.ruleType != rule.Type {
				continue
			}
			if len(rf.ruleNames) > 0 && !slices.Contains(rf.ruleNames, rule.Name) {
				continue
			}
			if (rule.LastError == "" && rf.filter == "unhealthy") || (!isNoMatch(rule) && rf.filter == "nomatch") {
				continue
			}
			if rf.excludeAlerts {
				rule.Alerts = nil
			}
			if rule.LastError != "" {
				g.Unhealthy++
			} else {
				g.Healthy++
			}
			if isNoMatch(rule) {
				g.NoMatch++
			}
			filteredRules = append(filteredRules, rule)
		}
		if len(groups) > 0 {
			if len(groups) == rf.maxGroups {
				// We've reached the capacity of our page plus one. That means that for sure there will be at least one
				// rule group in a subsequent request. Therefore, a next token is required.
				grpNextToken = strconv.Itoa(int(group.GetID()))
				break
			}
		}
		g.Rules = filteredRules
		groups = append(groups, g)
	}
	// sort list of groups for deterministic output
	slices.SortFunc(groups, func(a, b *rule.ApiGroup) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.File, b.File)
	})
	return groups, grpNextToken
}

func (rh *requestHandler) listGroups(rf *rulesFilter) ([]byte, error) {
	lr := listGroupsResponse{Status: "success"}
	lr.Data.Groups, lr.Data.GroupNextToken = rh.groups(rf)
	b, err := json.Marshal(lr)
	if err != nil {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`error encoding list of active alerts: %w`, err),
			StatusCode: http.StatusInternalServerError,
		}
	}
	return b, nil
}

type listAlertsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []*rule.ApiAlert `json:"alerts"`
	} `json:"data"`
}

func (rh *requestHandler) groupAlerts() []rule.GroupAlerts {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	var gAlerts []rule.GroupAlerts
	for _, g := range rh.m.groups {
		var alerts []*rule.ApiAlert
		for _, r := range g.Rules {
			a, ok := r.(*rule.AlertingRule)
			if !ok {
				continue
			}
			alerts = append(alerts, a.AlertsToAPI()...)
		}
		if len(alerts) > 0 {
			gAlerts = append(gAlerts, rule.GroupAlerts{
				Group:  g.ToAPI(),
				Alerts: alerts,
			})
		}
	}
	slices.SortFunc(gAlerts, func(a, b rule.GroupAlerts) int {
		return strings.Compare(a.Group.Name, b.Group.Name)
	})
	return gAlerts
}

func (rh *requestHandler) listAlerts(rf *rulesFilter) ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	lr := listAlertsResponse{Status: "success"}
	lr.Data.Alerts = make([]*rule.ApiAlert, 0)
	for _, group := range rh.m.groups {
		if !rf.matchesGroup(group) {
			continue
		}
		for _, r := range group.Rules {
			a, ok := r.(*rule.AlertingRule)
			if !ok {
				continue
			}
			lr.Data.Alerts = append(lr.Data.Alerts, a.AlertsToAPI()...)
		}
	}

	// sort list of alerts for deterministic output
	slices.SortFunc(lr.Data.Alerts, func(a, b *rule.ApiAlert) int {
		return strings.Compare(a.ID, b.ID)
	})

	b, err := json.Marshal(lr)
	if err != nil {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`error encoding list of active alerts: %w`, err),
			StatusCode: http.StatusInternalServerError,
		}
	}
	return b, nil
}

type listNotifiersResponse struct {
	Status string `json:"status"`
	Data   struct {
		Notifiers []*notifier.ApiNotifier `json:"notifiers"`
	} `json:"data"`
}

func (rh *requestHandler) listNotifiers() ([]byte, error) {
	targets := notifier.GetTargets()

	lr := listNotifiersResponse{Status: "success"}
	lr.Data.Notifiers = make([]*notifier.ApiNotifier, 0)
	for protoName, protoTargets := range targets {
		nr := &notifier.ApiNotifier{
			Kind:    protoName,
			Targets: make([]*notifier.ApiTarget, 0, len(protoTargets)),
		}
		for _, target := range protoTargets {
			nr.Targets = append(nr.Targets, &notifier.ApiTarget{
				Address:   target.Addr(),
				Labels:    target.Labels.ToMap(),
				LastError: target.LastError(),
			})
		}
		lr.Data.Notifiers = append(lr.Data.Notifiers, nr)
	}

	b, err := json.Marshal(lr)
	if err != nil {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`error encoding list of notifiers: %w`, err),
			StatusCode: http.StatusInternalServerError,
		}
	}
	return b, nil
}

func errResponse(err error, sc int) *httpserver.ErrorWithStatusCode {
	return &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: sc,
	}
}
