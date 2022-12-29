package vmselect

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

var (
	vmuiCustomDashboardsPath = flag.String("vmui.customDashboardsPath", "", "Optional path to vmui predefined dashboards")
)

// dashboardSetting represents dashboard settings file struct
type dashboardSetting struct {
	Title    string         `json:"title,omitempty"`
	Filename string         `json:"filename,omitempty"`
	Rows     []dashboardRow `json:"rows"`
}

// panelSettings represents fields which used to show graph
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
type dashboardRow struct {
	Title  string          `json:"title,omitempty"`
	Panels []panelSettings `json:"panels"`
}

// dashboardsData represents all dashboards settings
type dashboardsData struct {
	DashboardsSettings []dashboardSetting `json:"dashboardsSettings"`
}

func handleVMUICustomDashboards(w http.ResponseWriter) {
	var dashboardsData dashboardsData
	path := *vmuiCustomDashboardsPath
	if path == "" {
		writeSuccessResponse(w, dashboardsData)
		return
	}

	if !fs.IsPathExist(path) {
		writeErrorResponse(w, fmt.Errorf("cannot find folder pointed by -vmui.customDashboardsPath=%q", path))
		return
	}

	files, err := os.ReadDir(path)
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("cannot read folder pointed by -vmui.customDashboardsPath=%q", path))
		return
	}

	var settings []dashboardSetting
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			logger.Errorf("cannot get file info: %s", err)
			continue
		}
		if fs.IsDirOrSymlink(info) {
			logger.Infof("skip directory or symlinks")
			continue
		}
		filename := file.Name()
		if filepath.Ext(filename) == ".json" {
			filePath := filepath.Join(path, filename)
			f, err := os.ReadFile(filePath)
			if err != nil {
				writeErrorResponse(w, fmt.Errorf("cannot open file at -vmui.customDashboardsPath=%q: %w", filePath, err))
				return
			}
			var dSettings dashboardSetting
			err = json.Unmarshal(f, &dSettings)
			if err != nil {
				writeErrorResponse(w, fmt.Errorf("cannot parse file %s: %w", filename, err))
				return
			}
			if len(dSettings.Rows) == 0 {
				continue
			}
			settings = append(settings, dSettings)
		}
	}

	dashboardsData.DashboardsSettings = append(dashboardsData.DashboardsSettings, settings...)
	writeSuccessResponse(w, dashboardsData)
	return
}

func writeErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"error","error":"%q"}`, err.Error())
}

func writeSuccessResponse(w http.ResponseWriter, data dashboardsData) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("error encode dashboards settings: %s", err)
	}
}
