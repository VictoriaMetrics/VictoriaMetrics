package promscrape

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// WriteMetricRelabelDebug serves requests to /metric-relabel-debug page
func WriteMetricRelabelDebug(w http.ResponseWriter, r *http.Request) {
	targetID := r.FormValue("id")
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")
	format := r.FormValue("format")
	var err error

	if metric == "" && relabelConfigs == "" && targetID != "" {
		pcs, labels, ok := getMetricRelabelContextByTargetID(targetID)
		if !ok {
			err = fmt.Errorf("cannot find target for id=%s", targetID)
			targetID = ""
		} else {
			metric = labels.String()
			relabelConfigs = pcs.String()
		}
	}
	if format == "json" {
		httpserver.EnableCORS(w, r)
		w.Header().Set("Content-Type", "application/json")
	}
	promrelabel.WriteMetricRelabelDebug(w, targetID, metric, relabelConfigs, format, err)
}

// WriteTargetRelabelDebug generates response for /target-relabel-debug page
func WriteTargetRelabelDebug(w http.ResponseWriter, r *http.Request) {
	targetID := r.FormValue("id")
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")
	format := r.FormValue("format")
	var err error

	if metric == "" && relabelConfigs == "" && targetID != "" {
		pcs, labels, ok := getTargetRelabelContextByTargetID(targetID)
		if !ok {
			err = fmt.Errorf("cannot find target for id=%s", targetID)
			targetID = ""
		} else {
			metric = labels.String()
			relabelConfigs = pcs.String()
		}
	}
	if format == "json" {
		httpserver.EnableCORS(w, r)
		w.Header().Set("Content-Type", "application/json")
	}
	promrelabel.WriteTargetRelabelDebug(w, targetID, metric, relabelConfigs, format, err)
}
