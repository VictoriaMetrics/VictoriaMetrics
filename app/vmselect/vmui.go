package vmselect

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// more information how to use this flag please check this link
// https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
var (
	vmuiCustomDashboardsPath = flag.String("vmui.customDashboardsPath", "", "Optional path to vmui predefined dashboards")
)

var predefinedDashboards = &dashboardsData{}

// How frequently check defined vmuiCustomDashboardsPath
const updateDashboardsTimeout = time.Minute * 10

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

	mx                sync.Mutex
	expireAtTimestamp time.Time
}

func handleVMUICustomDashboards(w http.ResponseWriter) {
	path := *vmuiCustomDashboardsPath
	if path == "" {
		writeSuccessResponse(w, predefinedDashboards)
		return
	}

	if !fs.IsPathExist(path) {
		writeErrorResponse(w, fmt.Errorf("cannot find folder pointed by -vmui.customDashboardsPath=%q", path))
		return
	}

	requestTime := time.Now()
	if predefinedDashboards.expireAtTimestamp.Before(requestTime) {
		settings, err := collectDashboardsSettings(path)
		if err != nil {
			writeErrorResponse(w, fmt.Errorf("cannot collect dashboards settings by -vmui.customDashboardsPath=%q", path))
			return
		}

		predefinedDashboards.expireAtTimestamp = time.Now().Add(updateDashboardsTimeout)

		predefinedDashboards.mx.Lock()
		predefinedDashboards.DashboardsSettings = nil
		predefinedDashboards.DashboardsSettings = append(predefinedDashboards.DashboardsSettings, settings...)
		predefinedDashboards.mx.Unlock()
	}

	writeSuccessResponse(w, predefinedDashboards)
	return
}

func writeErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"error","error":"%q"}`, err.Error())
}

func writeSuccessResponse(w http.ResponseWriter, data *dashboardsData) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("error encode dashboards settings: %s", err)
	}
}

func collectDashboardsSettings(path string) ([]dashboardSetting, error) {

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read folder pointed by -vmui.customDashboardsPath=%q", path)
	}

	var settings []dashboardSetting
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			logger.Errorf("cannot get file info: %s", err)
			continue
		}
		if fs.IsDirOrSymlink(info) {
			logger.Infof("skip directory or symlinks in the -vmui.customDashboardsPath=%q", path)
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
	return settings, nil
}
