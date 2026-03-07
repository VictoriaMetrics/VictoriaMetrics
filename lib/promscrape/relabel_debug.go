package promscrape

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
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
			relabelConfigs += "# metrics_relabel_configs\n"
			relabelConfigs += pcs.String()

			// these are not target specific, so it should be placed here instead of `WriteMetricRelabelDebug` func, which handles target specific data.
			rwRelabelConfigs := remotewrite.GetRemoteWriteRelabelConfigString()
			rwURLRelabelConfigs := remotewrite.GetURLRelabelConfig()

			relabelConfigs += "\n# -remoteWrite.relabelConfig"
			relabelConfigs += "\n" + rwRelabelConfigs

			// we could have different relabel config for different remote write URL, but there's no way to know which one the user wants to debug.
			// so we append the 1st one here, and comment out the rest. user can see them on the page and edit to activate them.
			for i := range rwURLRelabelConfigs {

				if i == 0 {
					relabelConfigs += "\n# -remoteWrite.urlRelabelConfig"

					// append the URL info
					relabelConfigs += "\n# " + rwURLRelabelConfigs[i].Url

					// append the relabeling config string
					relabelConfigs += "\n" + rwURLRelabelConfigs[i].RelabelConfigStr
					continue
				}

				// add comment # before every line.
				relabelConfigs += "\n# " + rwURLRelabelConfigs[i].Url
				lines := strings.Split(rwURLRelabelConfigs[i].RelabelConfigStr, "\n")
				for _, line := range lines {
					relabelConfigs += "\n#" + line
				}
			}
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
