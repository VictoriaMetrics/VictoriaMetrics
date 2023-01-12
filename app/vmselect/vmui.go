package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// more information how to use this flag please check this link
// https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
var (
	vmuiCustomDashboardsPath = flag.String("vmui.customDashboardsPath", "", "Optional path to vmui predefined dashboards."+
		"How to create dashboards https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards")
)

// dashboardSetting represents dashboard settings file struct
// fields of the dashboardSetting you can find by following next link
// https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
type dashboardSetting struct {
	Title    string         `json:"title,omitempty"`
	Filename string         `json:"filename,omitempty"`
	Rows     []dashboardRow `json:"rows"`
}

// panelSettings represents fields which used to show graph
// fields of the panelSettings you can find by following next link
// https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
type panelSettings struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Expr        []string `json:"expr"`
	Alias       []string `json:"alias,omitempty"`
	ShowLegend  bool     `json:"showLegend,omitempty"`
	Width       int      `json:"width,omitempty"`
}

// dashboardRow represents panels on dashboard
// fields of the dashboardRow you can find by following next link
// https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
type dashboardRow struct {
	Title  string          `json:"title,omitempty"`
	Panels []panelSettings `json:"panels"`
}

// dashboardsData represents all dashboards settings
type dashboardsData struct {
	DashboardsSettings []dashboardSetting `json:"dashboardsSettings"`
}

func handleVMUICustomDashboards(w http.ResponseWriter) {
	path := *vmuiCustomDashboardsPath
	if path == "" {
		writeSuccessResponse(w, []byte(`{"dashboardsSettings": []}`))
		return
	}

	settings, err := collectDashboardsSettings(path)
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("cannot collect dashboards settings by -vmui.customDashboardsPath=%q", path))
		return
	}

	writeSuccessResponse(w, settings)
}

func writeErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"error","error":"%s"}`, err.Error())
}

func writeSuccessResponse(w http.ResponseWriter, data []byte) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func collectDashboardsSettings(path string) ([]byte, error) {

	if !fs.IsPathExist(path) {
		return nil, fmt.Errorf("cannot find folder pointed by -vmui.customDashboardsPath=%q", path)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read folder pointed by -vmui.customDashboardsPath=%q", path)
	}

	var settings []dashboardSetting
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			logger.Errorf("skipping %q at -vmui.customDashboardsPath=%q, since the info for this file cannot be obtained: %s", file.Name(), path, err)
			continue
		}
		if fs.IsDirOrSymlink(info) {
			logger.Infof("skip directory or symlinks: %q in the -vmui.customDashboardsPath=%q", info.Name(), path)
			continue
		}
		filename := file.Name()
		if filepath.Ext(filename) == ".json" {
			filePath := filepath.Join(path, filename)
			f, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("cannot open file at -vmui.customDashboardsPath=%q: %w", filePath, err)
			}
			var dSettings dashboardSetting
			err = json.Unmarshal(f, &dSettings)
			if err != nil {
				return nil, fmt.Errorf("cannot parse file %s: %w", filename, err)
			}
			if len(dSettings.Rows) == 0 {
				continue
			}
			settings = append(settings, dSettings)
		}
	}

	dd := dashboardsData{DashboardsSettings: settings}
	return json.Marshal(dd)
}
