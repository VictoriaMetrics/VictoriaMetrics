package promscrape

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// WriteMetricRelabelDebug serves requests to /metric-relabel-debug page.
// remotewrite-related relabel configs could be empty as vmsingle doesn't provide remote write feature.
func WriteMetricRelabelDebug(w http.ResponseWriter, r *http.Request, rwGlobalRelabelConfigs string, rwURLRelabelConfigss []string) {
	targetID := r.FormValue("id")
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")

	// if set, it means user selected another URL from the dropdown and everything will be reloaded.
	reloadRWURLRelabelConfigs := r.FormValue("reload_url_relabel_configs")
	rwURLRelabelConfigsIdxStr := r.FormValue("url_relabel_configs_index")

	format := r.FormValue("format")
	var err error

	// if all per-URL config is empty, it means no per-URL rule is configured.
	// set it to 0 so the user do not see the options in debug page.
	rwURLRelabelConfigsLength := 0
	for _, urlRelabelConfig := range rwURLRelabelConfigss {
		if urlRelabelConfig != "" {
			rwURLRelabelConfigsLength = len(rwURLRelabelConfigss)
			break
		}
	}

	rwURLRelabelConfigsIdx, idxErr := strconv.Atoi(rwURLRelabelConfigsIdxStr)
	if idxErr != nil {
		rwURLRelabelConfigsIdx = -1
	}

	// load the initial data with specific remote write URL index (default 0) in 2 cases:
	// - relabel config is empty. load scrape relabel (if targetID exist) + remote write related relabel (always).
	// - `reload` is set. load scrape relabel (if targetID exist) + reload remote write related relabel (by the URL index).
	init := metric == "" && relabelConfigs == "" && reloadRWURLRelabelConfigs == ""
	reload := reloadRWURLRelabelConfigs != ""
	if init || reload {
		// scrape related relabel labels & rules
		var (
			pcs    = &promrelabel.ParsedConfigs{} // could be empty
			labels *promutil.Labels
			ok     bool
		)
		if targetID != "" {
			pcs, labels, ok = getMetricRelabelContextByTargetID(targetID)
			if !ok {
				err = fmt.Errorf("cannot find target for id=%s", targetID)
				targetID = ""
			} else {
				metric = "up"
				metric += labels.String()
			}
		}

		// general relabel rules (remote write)
		// set the per-URL remote write relabel according to index, any error will fall back the index to 0.
		rwURLRelabelConfigs := ""
		if len(rwURLRelabelConfigss) > 0 {
			// ignore the error if the input is invalid or exceed the length, and fallback to 0.
			if rwURLRelabelConfigsIdx < 0 || rwURLRelabelConfigsIdx >= len(rwURLRelabelConfigss) {
				rwURLRelabelConfigsIdx = 0
			}
			rwURLRelabelConfigs = rwURLRelabelConfigss[rwURLRelabelConfigsIdx]
		}

		relabelConfigs = composeRelabelConfigs(pcs.String(), rwGlobalRelabelConfigs, rwURLRelabelConfigs, rwURLRelabelConfigsIdx)
	}

	if format == "json" {
		httpserver.EnableCORS(w, r)
		w.Header().Set("Content-Type", "application/json")
	}
	promrelabel.WriteMetricRelabelDebug(w, targetID, metric, relabelConfigs, rwURLRelabelConfigsLength, rwURLRelabelConfigsIdx, format, err)
}

func composeRelabelConfigs(relabelConfigs, rwGlobalRelabelConfigs, rwURLRelabelConfigs string, rwURLIdx int) string {
	if relabelConfigs != "" {
		relabelConfigs = "# -promscrape.config .scrape_configs[].metric_relabel_configs\n" + strings.TrimSpace(relabelConfigs) + "\n"
	}

	if rwGlobalRelabelConfigs != "" {
		relabelConfigs += "\n# -remoteWrite.relabelConfig"
		relabelConfigs += "\n" + strings.TrimSpace(rwGlobalRelabelConfigs) + "\n"
	}

	if rwURLRelabelConfigs != "" {
		relabelConfigs += fmt.Sprintf("\n# -remoteWrite.urlRelabelConfig=remote-write-url-%d", rwURLIdx)
		relabelConfigs += "\n" + strings.TrimSpace(rwURLRelabelConfigs) + "\n"
	}

	return relabelConfigs
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
			relabelConfigs = "# -promscrape.config .scrape_configs[].relabel_configs\n" + pcs.String()
		}
	}
	if format == "json" {
		httpserver.EnableCORS(w, r)
		w.Header().Set("Content-Type", "application/json")
	}
	promrelabel.WriteTargetRelabelDebug(w, targetID, metric, relabelConfigs, format, err)
}
