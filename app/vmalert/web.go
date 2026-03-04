package main

import (
	"cmp"
	"embed"
	"encoding/json"
	"fmt"
	"math"
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
	ruleStates = []string{"ok", "nomatch", "inactive", "firing", "pending", "unhealthy"}
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

func marshalJson(v any, kind string) ([]byte, *httpserver.ErrorWithStatusCode) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, errResponse(fmt.Errorf("failed to marshal %s: %s", kind, err), http.StatusInternalServerError)
	}
	return data, nil
}

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
		WriteRule(w, r, rule)
		return true
	// current used by old vmalert UI and Grafana Alerts
	case "/vmalert/groups", "/rules":
		rf, err := newRulesFilter(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		// only support filtering by a single state
		state := ""
		if len(rf.states) > 0 {
			state = rf.states[0]
			rf.states = rf.states[:1]
		}
		lr := rh.groups(rf)
		WriteListGroups(w, r, lr.Data.Groups, state)
		return true
	case "/vmalert/notifiers":
		WriteListTargets(w, r, notifier.GetTargets())
		return true

	case "/vmalert/api/v1/notifiers", "/api/v1/notifiers":
		data, err := rh.listNotifiers()
		if err != nil {
			errJson(w, r, err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/rules", "/api/v1/rules":
		// path used by Grafana for ng alerting
		rf, err := newRulesFilter(r)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		data, err := rh.listGroups(rf)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true

	case "/vmalert/api/v1/alerts", "/api/v1/alerts":
		// path used by Grafana for ng alerting
		gf, err := newGroupsFilter(r)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		data, err := rh.listAlerts(gf)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/alert", "/api/v1/alert":
		alert, err := rh.getAlert(r)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		data, err := marshalJson(alert, "alert")
		if err != nil {
			errJson(w, r, err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/rule", "/api/v1/rule":
		apiRule, err := rh.getRule(r)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		rwu := rule.ApiRuleWithUpdates{
			ApiRule:      apiRule,
			StateUpdates: apiRule.Updates,
		}
		data, err := marshalJson(rwu, "rule")
		if err != nil {
			errJson(w, r, err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/group", "/api/v1/group":
		group, err := rh.getGroup(r)
		if err != nil {
			errJson(w, r, err)
			return true
		}
		data, err := marshalJson(group, "group")
		if err != nil {
			errJson(w, r, err)
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

func (rh *requestHandler) getGroup(r *http.Request) (*rule.ApiGroup, *httpserver.ErrorWithStatusCode) {
	groupID, err := strconv.ParseUint(r.FormValue(rule.ParamGroupID), 10, 64)
	if err != nil {
		return nil, errResponse(fmt.Errorf("failed to read %q param: %w", rule.ParamGroupID, err), http.StatusBadRequest)
	}
	obj, err := rh.m.groupAPI(groupID)
	if err != nil {
		return nil, errResponse(err, http.StatusNotFound)
	}
	return obj, nil
}

func (rh *requestHandler) getRule(r *http.Request) (rule.ApiRule, *httpserver.ErrorWithStatusCode) {
	groupID, err := strconv.ParseUint(r.FormValue(rule.ParamGroupID), 10, 64)
	if err != nil {
		return rule.ApiRule{}, errResponse(fmt.Errorf("failed to read %q param: %w", rule.ParamGroupID, err), http.StatusBadRequest)
	}
	ruleID, err := strconv.ParseUint(r.FormValue(rule.ParamRuleID), 10, 64)
	if err != nil {
		return rule.ApiRule{}, errResponse(fmt.Errorf("failed to read %q param: %w", rule.ParamRuleID, err), http.StatusBadRequest)
	}
	obj, err := rh.m.ruleAPI(groupID, ruleID)
	if err != nil {
		return rule.ApiRule{}, errResponse(err, http.StatusNotFound)
	}
	return obj, nil
}

func (rh *requestHandler) getAlert(r *http.Request) (*rule.ApiAlert, *httpserver.ErrorWithStatusCode) {
	groupID, err := strconv.ParseUint(r.FormValue(rule.ParamGroupID), 10, 64)
	if err != nil {
		return nil, errResponse(fmt.Errorf("failed to read %q param: %w", rule.ParamGroupID, err), http.StatusBadRequest)
	}
	alertID, err := strconv.ParseUint(r.FormValue(rule.ParamAlertID), 10, 64)
	if err != nil {
		return nil, errResponse(fmt.Errorf("failed to read %q param: %w", rule.ParamAlertID, err), http.StatusBadRequest)
	}
	a, err := rh.m.alertAPI(groupID, alertID)
	if err != nil {
		return nil, errResponse(err, http.StatusNotFound)
	}
	return a, nil
}

type listGroupsResponse struct {
	Status      string `json:"status"`
	Page        int    `json:"page,omitempty"`
	TotalPages  int    `json:"total_pages,omitempty"`
	TotalGroups int    `json:"total_groups,omitempty"`
	TotalRules  int    `json:"total_rules,omitempty"`
	Data        struct {
		Groups []*rule.ApiGroup `json:"groups"`
	} `json:"data"`
}

type groupsFilter struct {
	groupNames []string
	files      []string
	dsType     config.Type
}

func newGroupsFilter(r *http.Request) (*groupsFilter, *httpserver.ErrorWithStatusCode) {
	_ = r.ParseForm()
	vs := r.Form
	gf := &groupsFilter{
		groupNames: vs["rule_group[]"],
		files:      vs["file[]"],
	}
	dsType := vs.Get("datasource_type")
	if len(dsType) > 0 {
		if config.SupportedType(dsType) {
			gf.dsType = config.NewRawType(dsType)
		} else {
			return nil, errResponse(fmt.Errorf(`invalid parameter "datasource_type": not supported value %q`, dsType), http.StatusBadRequest)
		}
	}
	return gf, nil
}

func (gf *groupsFilter) matches(group *rule.Group) bool {
	if len(gf.groupNames) > 0 && !slices.Contains(gf.groupNames, group.Name) {
		return false
	}
	if len(gf.files) > 0 && !slices.Contains(gf.files, group.File) {
		return false
	}
	if len(gf.dsType.Name) > 0 && gf.dsType.String() != group.Type.String() {
		return false
	}
	return true
}

// see https://prometheus.io/docs/prometheus/latest/querying/api/#rules
type rulesFilter struct {
	gf             *groupsFilter
	ruleNames      []string
	ruleType       string
	excludeAlerts  bool
	states         []string
	maxGroups      int
	pageNum        int
	search         string
	extendedStates bool
}

func newRulesFilter(r *http.Request) (*rulesFilter, *httpserver.ErrorWithStatusCode) {
	gf, err := newGroupsFilter(r)
	if err != nil {
		return nil, err
	}

	var rf rulesFilter
	rf.gf = gf
	vs := r.Form
	ruleTypeParam := vs.Get("type")
	if len(ruleTypeParam) > 0 {
		if ruleType, ok := ruleTypeMap[ruleTypeParam]; ok {
			rf.ruleType = ruleType
		} else {
			return nil, errResponse(fmt.Errorf(`invalid parameter "type": not supported value %q`, ruleTypeParam), http.StatusBadRequest)
		}
	}

	states := vs["state"]
	if len(states) == 0 {
		states = vs["filter"]
	}
	for _, s := range states {
		values := strings.Split(s, ",")
		for _, v := range values {
			if len(v) == 0 {
				continue
			}
			if !slices.Contains(ruleStates, v) {
				return nil, errResponse(fmt.Errorf(`invalid parameter "state": contains not supported value %q`, v), http.StatusBadRequest)
			}
			rf.states = append(rf.states, v)
		}
	}

	rf.excludeAlerts = httputil.GetBool(r, "exclude_alerts")
	rf.extendedStates = httputil.GetBool(r, "extended_states")
	rf.ruleNames = append([]string{}, vs["rule_name[]"]...)
	rf.search = strings.ToLower(vs.Get("search"))

	pageNum := vs.Get("page_num")
	maxGroups := vs.Get("group_limit")
	if pageNum != "" {
		if maxGroups == "" {
			return nil, errResponse(fmt.Errorf(`"group_limit" needs to be present in order to paginate over the groups`), http.StatusBadRequest)
		}
		v, err := strconv.Atoi(pageNum)
		if err != nil || v <= 0 {
			return nil, errResponse(fmt.Errorf(`"page_num" is expected to be a positive number, found %q`, pageNum), http.StatusBadRequest)
		}
		rf.pageNum = v
	}
	if maxGroups != "" {
		v, err := strconv.Atoi(maxGroups)
		if err != nil || v <= 0 {
			return nil, errResponse(fmt.Errorf(`"group_limit" is expected to be a positive number, found %q`, maxGroups), http.StatusBadRequest)
		}
		rf.maxGroups = v
	}
	return &rf, nil
}

func (rf *rulesFilter) matchesRule(r *rule.ApiRule) bool {
	if rf.ruleType != "" && rf.ruleType != r.Type {
		return false
	}
	if len(rf.ruleNames) > 0 && !slices.Contains(rf.ruleNames, r.Name) {
		return false
	}
	if len(rf.states) == 0 {
		return true
	}
	return slices.Contains(rf.states, r.State)
}

func (rh *requestHandler) groups(rf *rulesFilter) *listGroupsResponse {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	skipGroups := (rf.pageNum - 1) * rf.maxGroups
	lr := &listGroupsResponse{
		Status: "success",
	}
	lr.Data.Groups = make([]*rule.ApiGroup, 0)
	if skipGroups >= len(rh.m.groups) {
		return lr
	}
	// sort list of groups for deterministic output
	groups := make([]*rule.Group, 0, len(rh.m.groups))
	for _, group := range rh.m.groups {
		groups = append(groups, group)
	}

	slices.SortFunc(groups, func(a, b *rule.Group) int {
		nameCmp := cmp.Compare(a.Name, b.Name)
		if nameCmp != 0 {
			return nameCmp
		}
		return cmp.Compare(a.File, b.File)
	})
	for _, group := range groups {
		if !rf.gf.matches(group) {
			continue
		}
		groupFound := len(rf.search) == 0 || strings.Contains(strings.ToLower(group.Name), rf.search) || strings.Contains(strings.ToLower(group.File), rf.search)
		g := group.ToAPI()
		// the returned list should always be non-nil
		// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221
		filteredRules := make([]rule.ApiRule, 0)
		for _, rule := range g.Rules {
			if !groupFound && !strings.Contains(strings.ToLower(rule.Name), rf.search) {
				continue
			}
			if rf.extendedStates {
				rule.ExtendState()
			}
			if !rf.matchesRule(&rule) {
				continue
			}
			if rf.excludeAlerts {
				rule.Alerts = nil
			}
			g.States[rule.State]++
			filteredRules = append(filteredRules, rule)
		}
		if len(g.Rules) == 0 || len(filteredRules) > 0 {
			if rf.maxGroups > 0 {
				lr.TotalGroups++
				lr.TotalRules += len(filteredRules)
			}
			if skipGroups > 0 {
				skipGroups--
				continue
			}
			if rf.maxGroups == 0 || len(lr.Data.Groups) < rf.maxGroups {
				g.Rules = filteredRules
				lr.Data.Groups = append(lr.Data.Groups, g)
			}
		}
	}
	if rf.maxGroups > 0 {
		lr.Page = rf.pageNum
		lr.TotalPages = max(int(math.Ceil(float64(lr.TotalGroups)/float64(rf.maxGroups))), 1)
	}
	return lr
}

func (rh *requestHandler) listGroups(rf *rulesFilter) ([]byte, *httpserver.ErrorWithStatusCode) {
	lr := rh.groups(rf)
	if rf.pageNum > 1 && len(lr.Data.Groups) == 0 {
		return nil, errResponse(fmt.Errorf(`page_num exceeds total amount of pages`), http.StatusBadRequest)
	}
	if lr.Page > lr.TotalPages {
		return nil, errResponse(fmt.Errorf(`page_num=%d exceeds total amount of pages in result=%d`, lr.Page, lr.TotalPages), http.StatusBadRequest)
	}
	b, err := json.Marshal(lr)
	if err != nil {
		return nil, errResponse(fmt.Errorf(`error encoding list of groups: %w`, err), http.StatusInternalServerError)
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
	for _, group := range rh.m.groups {
		var alerts []*rule.ApiAlert
		g := group.ToAPI()
		for _, r := range g.Rules {
			if r.Type != rule.TypeAlerting {
				continue
			}
			alerts = append(alerts, r.Alerts...)
		}
		if len(alerts) > 0 {
			gAlerts = append(gAlerts, rule.GroupAlerts{
				Group:  g,
				Alerts: alerts,
			})
		}
	}
	slices.SortFunc(gAlerts, func(a, b rule.GroupAlerts) int {
		return strings.Compare(a.Group.Name, b.Group.Name)
	})
	return gAlerts
}

func (rh *requestHandler) listAlerts(gf *groupsFilter) ([]byte, *httpserver.ErrorWithStatusCode) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	lr := listAlertsResponse{Status: "success"}
	lr.Data.Alerts = make([]*rule.ApiAlert, 0)
	for _, group := range rh.m.groups {
		if !gf.matches(group) {
			continue
		}
		g := group.ToAPI()
		for _, r := range g.Rules {
			if r.Type != rule.TypeAlerting {
				continue
			}
			lr.Data.Alerts = append(lr.Data.Alerts, r.Alerts...)
		}
	}

	// sort list of alerts for deterministic output
	slices.SortFunc(lr.Data.Alerts, func(a, b *rule.ApiAlert) int {
		return strings.Compare(a.ID, b.ID)
	})

	b, err := json.Marshal(lr)
	if err != nil {
		return nil, errResponse(fmt.Errorf(`error encoding list of active alerts: %w`, err), http.StatusInternalServerError)
	}
	return b, nil
}

type listNotifiersResponse struct {
	Status string `json:"status"`
	Data   struct {
		Notifiers []*notifier.ApiNotifier `json:"notifiers"`
	} `json:"data"`
}

func (rh *requestHandler) listNotifiers() ([]byte, *httpserver.ErrorWithStatusCode) {
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
		return nil, errResponse(fmt.Errorf(`error encoding list of notifiers: %w`, err), http.StatusInternalServerError)
	}
	return b, nil
}

func errResponse(err error, sc int) *httpserver.ErrorWithStatusCode {
	return &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: sc,
	}
}

func errJson(w http.ResponseWriter, r *http.Request, err *httpserver.ErrorWithStatusCode) {
	w.Header().Set("Content-Type", "application/json")
	httpserver.Errorf(w, r, `{"error":%q,"errorType":%d}`, err, err.StatusCode)
}
