package promscrape

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// WriteMetricRelabelDebug serves requests to /metric-relabel-debug page
func WriteMetricRelabelDebug(w http.ResponseWriter, r *http.Request) {
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")
	promrelabel.WriteMetricRelabelDebug(w, metric, relabelConfigs)
}

// WriteTargetRelabelDebug generates response for /target-relabel-debug page
func WriteTargetRelabelDebug(w http.ResponseWriter, r *http.Request) {
	targetID := r.FormValue("id")
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")
	var err error

	if metric == "" && relabelConfigs == "" {
		pcs, labels, ok := getRelabelContextByTargetID(targetID)
		if !ok {
			err = fmt.Errorf("cannot find target for id=%s", targetID)
			targetID = ""
		} else {
			metric = labels.String()
			relabelConfigs = pcs.String()
		}
	}
	promrelabel.WriteTargetRelabelDebug(w, targetID, metric, relabelConfigs, err)
}
