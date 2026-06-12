package promscrape

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// WriteMetricRelabelDebug serves requests to /metric-relabel-debug page.
// remotewrite-related relabel configs could be empty as vmsingle doesn't provide remote write feature.
func WriteMetricRelabelDebug(w http.ResponseWriter, r *http.Request, rwGlobalRelabelConfigs string, rwURLRelabelConfigss []string) {
	targetID := r.FormValue("id")
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")
	rwRelabelConfigs := r.FormValue("remote_write_relabel_configs") // global + per-URL configs.

	rwURLRelabelConfigsIdx := r.FormValue("url_relabel_configs_index")     // only for per-URL configs and has to be set with reload_url_relabel_configs.
	reloadRWURLRelabelConfigs := r.FormValue("reload_url_relabel_configs") // if set, it will reset the whole remote_write_relabel_configs.

	format := r.FormValue("format")
	var err error

	rwURLRelabelConfigsLength := len(rwURLRelabelConfigss)

	// if everything is not set, we should load the initial data for user.
	if metric == "" && relabelConfigs == "" && rwRelabelConfigs == "" && rwURLRelabelConfigsIdx == "" && reloadRWURLRelabelConfigs == "" && targetID != "" {
		pcs, labels, ok := getMetricRelabelContextByTargetID(targetID)
		if !ok {
			err = fmt.Errorf("cannot find target for id=%s", targetID)
			targetID = ""
		} else {
			metric = labels.String()
			relabelConfigs += pcs.String()

			// by default use the first per-URL remote write relabel config, if exists.
			rwURLRelabelConfigs := ""
			if len(rwURLRelabelConfigss) > 0 {
				rwURLRelabelConfigs = rwURLRelabelConfigss[0]
			}

			if rwGlobalRelabelConfigs != "" {
				rwRelabelConfigs += "\n# -remoteWrite.relabelConfig"
				rwRelabelConfigs += "\n" + rwGlobalRelabelConfigs
			}
			if rwURLRelabelConfigs != "" {
				rwRelabelConfigs += "\n# -remoteWrite.urlRelabelConfig"
				rwRelabelConfigs += "\n" + rwURLRelabelConfigs
			}
		}
	}

	// if reloadRWURLRelabelConfigs is set, it means user clicked the button and want to reload the rwRelabelConfigs by rwURLRelabelConfigsIdx
	if reloadRWURLRelabelConfigs != "" {
		// set the per-URL remote write relabel according to index, any error will fall back the index to 0.
		rwURLRelabelConfigs := ""
		if len(rwURLRelabelConfigss) > 0 {
			// ignore the error if the input is invalid or exceed the length, and fallback to 0.
			idx, _ := strconv.Atoi(rwURLRelabelConfigsIdx)
			if idx < 0 || idx >= len(rwURLRelabelConfigss) {
				idx = 0
			}
			rwURLRelabelConfigs = rwURLRelabelConfigss[idx]
		}

		// reload will remove the existing content
		if rwGlobalRelabelConfigs != "" {
			rwRelabelConfigs = "\n# -remoteWrite.relabelConfig"
			rwRelabelConfigs += "\n" + rwGlobalRelabelConfigs
		}
		if rwURLRelabelConfigs != "" {
			rwRelabelConfigs += "\n# -remoteWrite.urlRelabelConfig"
			rwRelabelConfigs += "\n" + rwURLRelabelConfigs
		}
	}

	if format == "json" {
		httpserver.EnableCORS(w, r)
		w.Header().Set("Content-Type", "application/json")
	}
	promrelabel.WriteMetricRelabelDebug(w, targetID, metric, relabelConfigs, rwRelabelConfigs, rwURLRelabelConfigsLength, format, err)
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
			metric = labels.labelsString()
			relabelConfigs = pcs.String()
		}
	}
	if format == "json" {
		httpserver.EnableCORS(w, r)
		w.Header().Set("Content-Type", "application/json")
	}
	promrelabel.WriteTargetRelabelDebug(w, targetID, metric, relabelConfigs, format, err)
}
