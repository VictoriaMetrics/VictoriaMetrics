package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

// APIAlert represents an notifier.Alert state
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

type requestHandler struct {
	m *manager
}

var pathList = [][]string{
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
	case "/api/v1/alerts":
		resph.handle(rh.list())
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

type listAlertsResponse struct {
	Data struct {
		Alerts []*APIAlert `json:"alerts"`
	} `json:"data"`
	Status string `json:"status"`
}

func (rh *requestHandler) list() ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()
	lr := listAlertsResponse{Status: "success"}
	for _, g := range rh.m.groups {
		for _, r := range g.Rules {
			lr.Data.Alerts = append(lr.Data.Alerts, r.AlertsAPI()...)
		}
	}

	// sort list of alerts for deterministic output
	sort.Slice(lr.Data.Alerts, func(i, j int) bool {
		return lr.Data.Alerts[i].ID < lr.Data.Alerts[j].ID
	})

	b, err := json.Marshal(lr)
	if err != nil {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`error encoding list of active alerts: %s`, err),
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
		return nil, badRequest(fmt.Errorf(`cannot parse groupID: %s`, err))
	}
	alertID, err := uint64FromPath(parts[1])
	if err != nil {
		return nil, badRequest(fmt.Errorf(`cannot parse alertID: %s`, err))
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
