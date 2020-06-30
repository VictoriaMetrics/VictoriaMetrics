package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

type requestHandler struct {
	m *manager
}

var pathList = [][]string{
	{"/api/v1/groups", "list all loaded groups and rules"},
	{"/api/v1/alerts", "list all active alerts"},
	{"/api/v1/groupID/alertID/status", "get alert status by ID"},
	// /metrics is served by httpserver by default
	{"/metrics", "list of application metrics"},
	{"/-/reload", "reload configuration"},
}

func (rh *requestHandler) handler(w http.ResponseWriter, r *http.Request) bool {
	resph := responseHandler{w}
	switch r.URL.Path {
	case "/":
		for _, path := range pathList {
			p, doc := path[0], path[1]
			fmt.Fprintf(w, "<a href='%s'>%q</a> - %s<br/>", p, p, doc)
		}
		return true
	case "/api/v1/groups":
		resph.handle(rh.listGroups())
		return true
	case "/api/v1/alerts":
		resph.handle(rh.listAlerts())
		return true
	case "/-/reload":
		logger.Infof("api config reload was called, sending sighup")
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true
	default:
		// /api/v1/<groupName>/<alertID>/status
		if strings.HasSuffix(r.URL.Path, "/status") {
			resph.handle(rh.alert(r.URL.Path))
			return true
		}
		return false
	}
}

type listGroupsResponse struct {
	Data struct {
		Groups []APIGroup `json:"groups"`
	} `json:"data"`
	Status string `json:"status"`
}

func (rh *requestHandler) listGroups() ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	lr := listGroupsResponse{Status: "success"}
	for _, g := range rh.m.groups {
		lr.Data.Groups = append(lr.Data.Groups, g.toAPI())
	}

	// sort list of alerts for deterministic output
	sort.Slice(lr.Data.Groups, func(i, j int) bool {
		return lr.Data.Groups[i].Name < lr.Data.Groups[j].Name
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

type listAlertsResponse struct {
	Data struct {
		Alerts []*APIAlert `json:"alerts"`
	} `json:"data"`
	Status string `json:"status"`
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

func (rh *requestHandler) alert(path string) ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	parts := strings.SplitN(strings.TrimPrefix(path, "/api/v1/"), "/", 3)
	if len(parts) != 3 {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`path %q cointains /status suffix but doesn't match pattern "/group/alert/status"`, path),
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
	return json.Marshal(resp)
}

// responseHandler wrapper on http.ResponseWriter with sugar
type responseHandler struct{ http.ResponseWriter }

func (w responseHandler) handle(b []byte, err error) {
	if err != nil {
		httpserver.Errorf(w, "%s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
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
