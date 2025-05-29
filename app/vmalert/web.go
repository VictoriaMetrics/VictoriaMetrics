package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var reloadAuthKey = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*")

type requestHandler struct {
	m *manager
}

var (
	//go:embed vmui
	vmuiFiles      embed.FS
	vmuiFileServer = http.FileServer(http.FS(vmuiFiles))
)

func (rh *requestHandler) handler(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if path == "/vmui" {
		// VMUI access via incomplete url without `/` in the end. Redirect to complete url.
		// Use relative redirect, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		_ = r.ParseForm()
		newURL := "vmui/?" + r.Form.Encode()
		httpserver.Redirect(w, newURL)
		return true
	}
	if path == "" || path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>VictoriaMetrics VMAlert</h2></br>")
		fmt.Fprintf(w, "See docs at <a href='https://docs.victoriametrics.com/victoriametrics/vmalert/'>https://docs.victoriametrics.com/victoriametrics/vmalert/</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		httpserver.WriteAPIHelp(w, [][2]string{
			{"vmui", "Web UI"},
			{"api/v1/notifiers", "list all static and discovered endpoints, where alerts are sent to"},
			{"api/v1/rules", "list all loaded groups and rules"},
			{"api/v1/alerts", "list all active alerts"},
			{"-/reload", "reload configuration"},
		})
		return true
	}
	switch path {
	case "/api/v1/rules":
		// path used by Grafana for ng alerting
		rf := extractRulesFilter(r)
		data, err := rh.listGroups(rf)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/api/v1/alerts":
		// path used by Grafana for ng alerting
		data, err := rh.listAlerts()
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return true
	case "/api/v1/notifiers":
		data, err := rh.listNotifiers()
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
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
	}
	if strings.HasPrefix(path, "/vmui") {
		r.URL.Path = path
		vmuiFileServer.ServeHTTP(w, r)
		return true
	}
	return false
}

type listGroupsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []*apiGroup `json:"groups"`
	} `json:"data"`
}

// see https://prometheus.io/docs/prometheus/latest/querying/api/#rules
type rulesFilter struct {
	files         []string
	groupNames    []string
	ruleNames     []string
	ruleType      string
	excludeAlerts bool
}

func extractRulesFilter(r *http.Request) rulesFilter {
	rf := rulesFilter{}

	var ruleType string
	ruleTypeParam := r.URL.Query().Get("type")
	// for some reason, `type` in filter doesn't match `type` in response,
	// so we use this matching here
	if ruleTypeParam == "alert" {
		ruleType = ruleTypeAlerting
	} else if ruleTypeParam == "record" {
		ruleType = ruleTypeRecording
	}
	rf.ruleType = ruleType

	rf.excludeAlerts = httputil.GetBool(r, "exclude_alerts")
	rf.ruleNames = append([]string{}, r.Form["rule_name[]"]...)
	rf.groupNames = append([]string{}, r.Form["rule_group[]"]...)
	rf.files = append([]string{}, r.Form["file[]"]...)
	return rf
}

func (rh *requestHandler) listGroups(rf rulesFilter) ([]byte, error) {
	rh.m.groupsMu.RLock()
	defer rh.m.groupsMu.RUnlock()

	lr := listGroupsResponse{Status: "success"}
	lr.Data.Groups = make([]*apiGroup, 0)
	for _, group := range rh.m.groups {
		if len(rf.groupNames) > 0 && !slices.Contains(rf.groupNames, group.Name) {
			continue
		}
		if len(rf.files) > 0 && !slices.Contains(rf.files, group.File) {
			continue
		}

		g := groupToAPI(group)
		// the returned list should always be non-nil
		// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4221
		filteredRules := make([]apiRule, 0)
		for _, r := range g.Rules {
			if rf.ruleType != "" && rf.ruleType != r.Type {
				continue
			}
			if len(rf.ruleNames) > 0 && !slices.Contains(rf.ruleNames, r.Name) {
				continue
			}
			if rf.excludeAlerts {
				r.Alerts = nil
			}
			filteredRules = append(filteredRules, r)
		}
		g.Rules = filteredRules
		lr.Data.Groups = append(lr.Data.Groups, g)
	}
	// sort list of groups for deterministic output
	slices.SortFunc(lr.Data.Groups, func(a, b *apiGroup) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.File, b.File)
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
	Status string `json:"status"`
	Data   struct {
		Alerts []*apiAlert `json:"alerts"`
	} `json:"data"`
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
	slices.SortFunc(lr.Data.Alerts, func(a, b *apiAlert) int {
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
		Notifiers []*apiNotifier `json:"notifiers"`
	} `json:"data"`
}

func (rh *requestHandler) listNotifiers() ([]byte, error) {
	targets := notifier.GetTargets()

	lr := listNotifiersResponse{Status: "success"}
	lr.Data.Notifiers = make([]*apiNotifier, 0)
	for protoName, protoTargets := range targets {
		notifier := &apiNotifier{
			Kind:    string(protoName),
			Targets: make([]*apiTarget, 0, len(protoTargets)),
		}
		for _, target := range protoTargets {
			notifier.Targets = append(notifier.Targets, &apiTarget{
				Address: target.Notifier.Addr(),
				Labels:  target.Labels.ToMap(),
			})
		}
		lr.Data.Notifiers = append(lr.Data.Notifiers, notifier)
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
