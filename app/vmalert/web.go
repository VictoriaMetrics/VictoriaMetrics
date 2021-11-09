package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	pathPrefix := httpserver.GetPathPrefix()
	apiLinks = [][2]string{
		{path.Join(pathPrefix, "api/v1/groups"), "list all loaded groups and rules"},
		{path.Join(pathPrefix, "api/v1/alerts"), "list all active alerts"},
		{path.Join(pathPrefix, "api/v1/groupID/alertID/status"), "get alert status by ID"},
		{path.Join(pathPrefix, "flags"), "command-line flags"},
		{path.Join(pathPrefix, "metrics"), "list of application metrics"},
		{path.Join(pathPrefix, "-/reload"), "reload configuration"},
	}
	navItems = []tpl.NavItem{
		{Name: "vmalert", Url: pathPrefix},
		{Name: "Groups", Url: path.Join(pathPrefix, "groups")},
		{Name: "Alerts", Url: path.Join(pathPrefix, "alerts")},
		{Name: "Docs", Url: "https://docs.victoriametrics.com/vmalert.html"},
	}
}

type requestHandler struct {
	m *manager
}

func (rh *requestHandler) handler(w http.ResponseWriter, r *http.Request) bool {
	once.Do(func() {
		initLinks()
	})

	switch r.URL.Path {
	case "/":
		if r.Method != "GET" {
			return false
		}
		WriteWelcome(w)
		return true
	case "/alerts":
		WriteListAlerts(w, rh.groupAlerts())
		return true
	case "/groups":
		WriteListGroups(w, rh.groups())
		return true
	case "/api/v1/groups":
		data, err := rh.listGroups()
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/api/v1/alerts":
		data, err := rh.listAlerts()
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
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
		if !strings.HasSuffix(r.URL.Path, "/status") {
			return false
		}
		alert, err := rh.alertByPath(strings.TrimPrefix(r.URL.Path, "/api/v1/"))
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}

		// /api/v1/<groupID>/<alertID>/status
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			data, err := json.Marshal(alert)
			if err != nil {
				httpserver.Errorf(w, r, "failed to marshal alert: %s", err)
				return true
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return true
		}

		// <groupID>/<alertID>/status
		WriteAlert(w, alert)
		return true
	}
}

type listGroupsResponse struct {
	Data struct {
		Groups []APIGroup `json:"groups"`
	} `json:"data"`
	Status string `json:"status"`
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
	Data struct {
		Alerts []*APIAlert `json:"alerts"`
	} `json:"data"`
	Status string `json:"status"`
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
			alerts = append(alerts, a.AlertsAPI()...)
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
			lr.Data.Alerts = append(lr.Data.Alerts, a.AlertsAPI()...)
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
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	parts := strings.SplitN(strings.TrimLeft(path, "/"), "/", 3)
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
