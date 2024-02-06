package vmselect

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

var (
	vmuiCustomDashboardsPath = flag.String("vmui.customDashboardsPath", "", "Optional path to vmui dashboards. "+
		"See https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards")
	vmuiDefaultTimezone = flag.String("vmui.defaultTimezone", "", "The default timezone to be used in vmui. "+
		"Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local")
)

// dashboardSettings represents dashboard settings file struct.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
type dashboardSettings struct {
	Title    string         `json:"title,omitempty"`
	Filename string         `json:"filename,omitempty"`
	Rows     []dashboardRow `json:"rows"`
}

// panelSettings represents fields which used to show graph.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
type panelSettings struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Expr        []string `json:"expr"`
	Alias       []string `json:"alias,omitempty"`
	ShowLegend  bool     `json:"showLegend,omitempty"`
	Width       int      `json:"width,omitempty"`
}

// dashboardRow represents panels on dashboard.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui/packages/vmui/public/dashboards
type dashboardRow struct {
	Title  string          `json:"title,omitempty"`
	Panels []panelSettings `json:"panels"`
}

// dashboardsData represents all the dashboards settings.
type dashboardsData struct {
	DashboardsSettings []dashboardSettings `json:"dashboardsSettings"`
}

func handleVMUICustomDashboards(w http.ResponseWriter) error {
	path := *vmuiCustomDashboardsPath
	if path == "" {
		writeSuccessResponse(w, []byte(`{"dashboardsSettings": []}`))
		return nil
	}
	settings, err := collectDashboardsSettings(path)
	if err != nil {
		return fmt.Errorf("cannot collect dashboards settings by -vmui.customDashboardsPath=%q: %w", path, err)
	}
	writeSuccessResponse(w, settings)
	return nil
}

func handleVMUITimezone(w http.ResponseWriter) error {
	tz, err := time.LoadLocation(*vmuiDefaultTimezone)
	if err != nil {
		return fmt.Errorf("cannot load timezone %q: %w", *vmuiDefaultTimezone, err)
	}
	response := fmt.Sprintf(`{"timezone": %q}`, tz)
	writeSuccessResponse(w, []byte(response))
	return nil
}

func writeSuccessResponse(w http.ResponseWriter, data []byte) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func collectDashboardsSettings(path string) ([]byte, error) {
	if !fs.IsPathExist(path) {
		return nil, fmt.Errorf("cannot find folder %q", path)
	}
	files := fs.MustReadDir(path)

	var dss []dashboardSettings
	for _, file := range files {
		filename := file.Name()
		if filepath.Ext(filename) != ".json" {
			continue
		}
		filePath := filepath.Join(path, filename)
		f, err := os.ReadFile(filePath)
		if err != nil {
			// There is no need to add more context to the returned error, since os.ReadFile() adds enough context.
			return nil, err
		}
		var ds dashboardSettings
		err = json.Unmarshal(f, &ds)
		if err != nil {
			return nil, fmt.Errorf("cannot parse file %s: %w", filePath, err)
		}
		if len(ds.Rows) > 0 {
			dss = append(dss, ds)
		}
	}

	dd := dashboardsData{DashboardsSettings: dss}
	return json.Marshal(dd)
}
