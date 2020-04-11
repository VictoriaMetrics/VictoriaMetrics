package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
)

// apiAlert has info for an alert.
type apiAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    time.Time         `json:"activeAt"`
	Value       string            `json:"value"`
}

type requestHandler struct {
	groups []Group
}

func (rh *requestHandler) handler(w http.ResponseWriter, r *http.Request) bool {
	resph := responseHandler{w}
	switch r.URL.Path {
	default:
		if strings.HasSuffix(r.URL.Path, "/status") {
			resph.handle(rh.alert(r.URL.Path))
			return true
		}
		return false
	case "/api/v1/alerts":
		resph.handle(rh.listActiveAlerts())
		return true
	}
}

func (rh *requestHandler) listActiveAlerts() ([]byte, error) {
	type listAlertsResponse struct {
		Data struct {
			Alerts []apiAlert `json:"alerts"`
		} `json:"data"`
		Status string `json:"status"`
	}
	lr := listAlertsResponse{Status: "success"}
	for _, g := range rh.groups {
		alerts := g.ActiveAlerts()
		for i := range alerts {
			alert := alerts[i]
			lr.Data.Alerts = append(lr.Data.Alerts, apiAlert{
				Labels:      alert.Labels,
				Annotations: alert.Annotations,
				State:       alert.State.String(),
				ActiveAt:    alert.Start,
				Value:       strconv.FormatFloat(alert.Value, 'e', -1, 64),
			})
		}
	}

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
	parts := strings.SplitN(strings.TrimPrefix(path, "/api/v1/"), "/", 3)
	if len(parts) != 3 {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`path %q cointains /status suffix but doesn't match pattern "/group/alert/status"`, path),
			StatusCode: http.StatusBadRequest,
		}
	}
	group := strings.TrimRight(parts[0], "/")
	idStr := strings.TrimRight(parts[1], "/")
	id, err := strconv.ParseUint(idStr, 10, 0)
	if err != nil {
		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf(`cannot parse int from %s"`, idStr),
			StatusCode: http.StatusBadRequest,
		}
	}
	for _, g := range rh.groups {
		if g.Name == group {
			for i := range g.Rules {
				if alert := g.Rules[i].Alert(id); alert != nil {
					return json.Marshal(apiAlert{
						Labels:      alert.Labels,
						Annotations: alert.Annotations,
						State:       alert.State.String(),
						ActiveAt:    alert.Start,
						Value:       strconv.FormatFloat(alert.Value, 'e', -1, 64),
					})
				}
			}
		}
	}
	return nil, &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf(`cannot find alert %s in %s"`, idStr, group),
		StatusCode: http.StatusNotFound,
	}
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
