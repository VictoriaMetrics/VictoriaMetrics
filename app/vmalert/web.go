package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
)

// APIAlert has info for an alert.
type APIAlert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Group       string            `json:"group"`
	Expression  string            `json:"expression"`
	State       string            `json:"state"`
	Value       string            `json:"value"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	ActiveAt    time.Time         `json:"activeAt"`
}

type requestHandler struct {
	groups []Group
	mu     sync.RWMutex
}

func (rh *requestHandler) runConfigUpdater(ctx context.Context, reloadChan <-chan os.Signal, groupUpdateStorage map[string]chan Group, w *watchdog, wg *sync.WaitGroup) {
	logger.Infof("starting config updater")
	defer wg.Done()
	for {
		select {
		case <-reloadChan:
			logger.Infof("get sighup signal, updating config")
			configReloadTotal.Inc()
			newRules, err := readRules()
			if err != nil {
				logger.Errorf("sighup, cannot read new rules: %v", err)
				configReloadErrorTotal.Inc()
				continue
			}

			rh.mu.Lock()
			configReloadOkTotal.Inc()
			//send new group to running watchers
			for _, group := range newRules {
				//update or start new group
				if updateChan, ok := groupUpdateStorage[group.Name]; ok {
					updateChan <- group
				} else {
					//its new group, we need to start it
					updateChan := make(chan Group, 1)
					groupUpdateStorage[group.Name] = updateChan
					wg.Add(1)
					go func(grp Group) {
						w.run(ctx, grp, *evaluationInterval, updateChan)
						wg.Done()
					}(group)
					//add new group to route handler
					rh.groups = append(rh.groups, group)
				}
			}
			//we have to check, if group is missing and remove it
			for groupName, updateChan := range groupUpdateStorage {
				var exist bool
				for _, newGroup := range newRules {
					if groupName == newGroup.Name {
						exist = true
					}
				}
				if !exist {
					logger.Infof("group not exists in new rules, remove it, group: %s", groupName)
					delete(groupUpdateStorage, groupName)
					updateChan <- Group{Rules: []*Rule{}}
					for i, group := range rh.groups {
						if group.Name == groupName {
							rh.groups[i] = rh.groups[len(rh.groups)-1]
							rh.groups[len(rh.groups)-1] = Group{}
							rh.groups = rh.groups[:len(rh.groups)-1]
						}
					}
				}
			}
			rh.mu.Unlock()
			logger.Infof("finished sync")

		case <-ctx.Done():
			logger.Infof("exiting config updater")
			return

		}
	}
}

var pathList = [][]string{
	{"/api/v1/alerts", "list all active alerts"},
	{"/api/v1/groupName/alertID/status", "get alert status by ID"},
	// /metrics is served by httpserver by default
	{"/metrics", "list of application metrics"},
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
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	lr := listAlertsResponse{Status: "success"}
	for _, g := range rh.groups {
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
	rh.mu.RLock()
	defer rh.mu.RUnlock()
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
			Err:        fmt.Errorf(`cannot parse int from %q`, idStr),
			StatusCode: http.StatusBadRequest,
		}
	}
	for _, g := range rh.groups {
		if g.Name != group {
			continue
		}
		for _, rule := range g.Rules {
			if apiAlert := rule.AlertAPI(id); apiAlert != nil {
				return json.Marshal(apiAlert)
			}
		}
	}
	return nil, &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf(`cannot find alert %s in %q`, idStr, group),
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
