package vmselect

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
		err := writeSuccessResponse(w, dashboardsData)
		if err != nil {
			writeErrorResponse(w, fmt.Errorf("cannot send dashboards data: %s", err))
			return
		}
		return
	}

	if !fs.IsPathExist(path) {
		writeErrorResponse(w, fmt.Errorf("cannot find folder with dashboards by provided path: %s", path))
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("cannot obtain abs path for %q: %w", path, err))
		return
	}

	var settings []dashboardSetting
	files, err := os.ReadDir(absPath)
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("cannot read provided directory: %s", absPath))
		return
	}
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		if fs.IsDirOrSymlink(info) {
			continue
		}
		filename := file.Name()
		if filepath.Ext(filename) == ".json" {
			filePath := filepath.Join(absPath, filename)
			f, err := fs.ReadFileOrHTTP(filePath)
			if err != nil {
				writeErrorResponse(w, fmt.Errorf("cannot open file: %s, %s", filename, err))
				return
			}
			var dSettings dashboardSetting
			err = json.Unmarshal(f, &dSettings)
			if err != nil {
				writeErrorResponse(w, fmt.Errorf("cannot unmarshal dashboard settings: %s from file: %s", err, filename))
				return
			}
			if len(dSettings.Rows) == 0 {
				continue
			}
			settings = append(settings, dSettings)
		}
	}

	dashboardsData.DashboardsSettings = append(dashboardsData.DashboardsSettings, settings...)
	err = writeSuccessResponse(w, dashboardsData)
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("cannot send dashboards data: %s", err))
		return
	}
}

func writeErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"error","error":"%s"}`, err.Error())
}

func writeSuccessResponse(w http.ResponseWriter, data dashboardsData) error {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}
