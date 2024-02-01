package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/tpl"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var reloadAuthKey = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed as authKey=...")

var (
	apiLinks = [][2]string{
		// api links are relative since they can be used by external clients,
		// such as Grafana, and proxied via vmselect.
		{"api/v1/rules", "list all loaded groups and rules"},
		{"api/v1/alerts", "list all active alerts"},
		{fmt.Sprintf("api/v1/alert?%s=<int>&%s=<int>", paramGroupID, paramAlertID), "get alert status by group and alert ID"},
	}
	systemLinks = [][2]string{
		{"/flags", "command-line flags"},
		{"/metrics", "list of application metrics"},
		{"/-/reload", "reload configuration"},
	}
	navItems = []tpl.NavItem{
		{Name: "vmalert", Url: "."},
		{Name: "Groups", Url: "groups"},
		{Name: "Alerts", Url: "alerts"},
		{Name: "Notifiers", Url: "notifiers"},
		{Name: "Docs", Url: "https://docs.victoriametrics.com/vmalert.html"},
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
		WriteListGroups(w, r, rh.groups())
		return true
	case "/vmalert/notifiers":
		WriteListTargets(w, r, notifier.GetTargets())
		return true

	// special cases for Grafana requests,
	// served without `vmalert` prefix:
	case "/rules":
		// Grafana makes an extra request to `/rules`
		// handler in addition to `/api/v1/rules` calls in alerts UI,
		WriteListGroups(w, r, rh.groups())
		return true

	case "/vmalert/api/v1/rules", "/api/v1/rules":
		// path used by Grafana for ng alerting
		data, err := rh.listGroups()
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/vmalert/api/v1/alerts", "/api/v1/alerts":
		// path used by Grafana for ng alerting
		data, err := rh.listAlerts()
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
		rule, err := rh.getRule(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		rwu := apiRuleWithUpdates{
			apiRule:      rule,
			StateUpdates: rule.Updates,
		}
		data, err := json.Marshal(rwu)
		if err != nil {
			httpserver.Errorf(w, r, "failed to marshal rule: %s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/-/reload":
		if !httpserver.CheckAuthFlag(w, r, reloadAuthKey.Get(), "reloadAuthKey") {
			return true
		}
		logger.Infof("api config reload was called, sending sighup")
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true

	default:
		httpserver.Errorf(w, r, "unsupported path requested: %q ", r.URL.Path)
		return false
	}
}

func (rh *requestHandler) getRule(r *http.Request) (apiRule, error) {
	groupID, err := strconv.ParseUint(r.FormValue(paramGroupID), 10, 64)
	if err != nil {
		return apiRule{}, fmt.Errorf("failed to read %q param: %w", paramGroupID, err)
	}
	ruleID, err := strconv.ParseUint(r.FormValue(paramRuleID), 10, 64)
	if err != nil {
		return apiRule{}, fmt.Errorf("failed to read %q param: %w", paramRuleID, err)
	}
	obj, err := rh.m.ruleAPI(groupID, ruleID)
	if err != nil {
		return apiRule{}, errResponse(err, http.StatusNotFound)
	}
	return obj, nil
}

func (rh *requestHandler) getAlert(r *http.Request) (*apiAlert, error) {
	groupID, err := strconv.ParseUint(r.FormValue(paramGroupID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %w", paramGroupID, err)
	}
	alertID, err := strconv.ParseUint(r.FormValue(paramAlertID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %w", paramAlertID, err)
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
		Groups []apiGroup `json:"groups"`
	} `json:"data"`
}

func (rh *requestHandler) groups() []apiGroup {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	groups := make([]apiGroup, 0)
	for _, g := range rh.m.groups {
		groups = append(groups, groupToAPI(g))
	}

	// sort list of alerts for deterministic output
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}

func (rh *requestHandler) listGroups() ([]byte, error) {
	lr := listGroupsResponse{Status: "success"}
	lr.Data.Groups = rh.groups()
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
		Alerts []*apiAlert `json:"alerts"`
	} `json:"data"`
}

func (rh *requestHandler) groupAlerts() []groupAlerts {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	var gAlerts []groupAlerts
	for _, g := range rh.m.groups {
		var alerts []*apiAlert
		for _, r := range g.Rules {
			a, ok := r.(*rule.AlertingRule)
			if !ok {
				continue
			}
			alerts = append(alerts, ruleToAPIAlert(a)...)
		}
		if len(alerts) > 0 {
			gAlerts = append(gAlerts, groupAlerts{
				Group:  groupToAPI(g),
				Alerts: alerts,
			})
		}
	}
	sort.Slice(gAlerts, func(i, j int) bool {
		return gAlerts[i].Group.Name < gAlerts[j].Group.Name
	})
	return gAlerts
}

func (rh *requestHandler) listAlerts() ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	lr := listAlertsResponse{Status: "success"}
	lr.Data.Alerts = make([]*apiAlert, 0)
	for _, g := range rh.m.groups {
		for _, r := range g.Rules {
			a, ok := r.(*rule.AlertingRule)
			if !ok {
				continue
			}
			lr.Data.Alerts = append(lr.Data.Alerts, ruleToAPIAlert(a)...)
		}
	}

	// sort list of alerts for deterministic output
	sort.Slice(lr.Data.Alerts, func(i, j int) bool {
		return lr.Data.Alerts[i].ID < lr.Data.Alerts[j].ID
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

func errResponse(err error, sc int) *httpserver.ErrorWithStatusCode {
	return &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: sc,
	}
}
