package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/tpl"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	once     = sync.Once{}
	apiLinks [][2]string
	navItems []tpl.NavItem
)

func initLinks() {
	apiLinks = [][2]string{
		// api links are relative since they can be used by external clients,
		// such as Grafana, and proxied via vmselect.
		{"api/v1/rules", "list all loaded groups and rules"},
		{"api/v1/alerts", "list all active alerts"},
		{fmt.Sprintf("api/v1/alert?%s=<int>&%s=<int>", paramGroupID, paramAlertID), "get alert status by group and alert ID"},

		// system links
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
}

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
	once.Do(func() {
		initLinks()
	})

	if strings.HasPrefix(r.URL.Path, "/vmalert/static") {
		staticServer.ServeHTTP(w, r)
		return true
	}

	switch r.URL.Path {
	case "/", "/vmalert", "/vmalert/":
		if r.Method != "GET" {
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
	case "/-/reload":
		logger.Infof("api config reload was called, sending sighup")
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true

	default:
		// Support of deprecated links:
		// * /api/v1/<groupID>/<alertID>/status
		// * <groupID>/<alertID>/status
		// TODO: to remove in next versions

		if !strings.HasSuffix(r.URL.Path, "/status") {
			httpserver.Errorf(w, r, "unsupported path requested: %q ", r.URL.Path)
			return false
		}
		alert, err := rh.alertByPath(strings.TrimPrefix(r.URL.Path, "/api/v1/"))
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}

		redirectURL := alert.WebLink()
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			redirectURL = alert.APILink()
		}
		http.Redirect(w, r, "/"+redirectURL, http.StatusPermanentRedirect)
		return true
	}
}

const (
	paramGroupID = "group_id"
	paramAlertID = "alert_id"
)

func (rh *requestHandler) getAlert(r *http.Request) (*APIAlert, error) {
	groupID, err := strconv.ParseUint(r.FormValue(paramGroupID), 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %s", paramGroupID, err)
	}
	alertID, err := strconv.ParseUint(r.FormValue(paramAlertID), 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q param: %s", paramAlertID, err)
	}
	a, err := rh.m.AlertAPI(groupID, alertID)
	if err != nil {
		return nil, errResponse(err, http.StatusNotFound)
	}
	return a, nil
}

type listGroupsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []APIGroup `json:"groups"`
	} `json:"data"`
}

func (rh *requestHandler) groups() []APIGroup {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	var groups []APIGroup
	for _, g := range rh.m.groups {
		groups = append(groups, g.toAPI())
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
		Alerts []*APIAlert `json:"alerts"`
	} `json:"data"`
}

func (rh *requestHandler) groupAlerts() []GroupAlerts {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	var groupAlerts []GroupAlerts
	for _, g := range rh.m.groups {
		var alerts []*APIAlert
		for _, r := range g.Rules {
			a, ok := r.(*AlertingRule)
			if !ok {
				continue
			}
			alerts = append(alerts, a.AlertsToAPI()...)
		}
		if len(alerts) > 0 {
			groupAlerts = append(groupAlerts, GroupAlerts{
				Group:  g.toAPI(),
				Alerts: alerts,
			})
		}
	}
	return groupAlerts
}

func (rh *requestHandler) listAlerts() ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	lr := listAlertsResponse{Status: "success"}
	for _, g := range rh.m.groups {
		for _, r := range g.Rules {
			a, ok := r.(*AlertingRule)
			if !ok {
				continue
			}
			lr.Data.Alerts = append(lr.Data.Alerts, a.AlertsToAPI()...)
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

func (rh *requestHandler) alertByPath(path string) (*APIAlert, error) {
	if strings.HasPrefix(path, "/vmalert") {
		path = strings.TrimLeft(path, "/vmalert")
	}
	parts := strings.SplitN(strings.TrimLeft(path, "/"), "/", -1)
	if len(parts) != 3 {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`path %q cointains /status suffix but doesn't match pattern "/groupID/alertID/status"`, path),
			StatusCode: http.StatusBadRequest,
		}
	}
	groupID, err := uint64FromPath(parts[0])
	if err != nil {
		return nil, badRequest(fmt.Errorf(`cannot parse groupID: %w`, err))
	}
	alertID, err := uint64FromPath(parts[1])
	if err != nil {
		return nil, badRequest(fmt.Errorf(`cannot parse alertID: %w`, err))
	}
	resp, err := rh.m.AlertAPI(groupID, alertID)
	if err != nil {
		return nil, errResponse(err, http.StatusNotFound)
	}
	return resp, nil
}

func uint64FromPath(path string) (uint64, error) {
	s := strings.TrimRight(path, "/")
	return strconv.ParseUint(s, 10, 0)
}

func badRequest(err error) *httpserver.ErrorWithStatusCode {
	return errResponse(err, http.StatusBadRequest)
}

func errResponse(err error, sc int) *httpserver.ErrorWithStatusCode {
	return &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: sc,
	}
}
