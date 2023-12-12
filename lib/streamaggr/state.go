package streamaggr

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// WriteHumanReadableState writes human-readable state for all aggregations.
func WriteHumanReadableState(w http.ResponseWriter, r *http.Request, rws map[string]*Aggregators) {
	rwActive := r.FormValue("rw")
	if rwActive == "" {
		for key := range rws {
			rwActive = key
			break
		}
	}
	rw, ok := rws[rwActive]
	if !ok {
		_, _ = fmt.Fprintf(w, "not found remoteWrite '%v'", rwActive)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	aggParam := r.FormValue("agg")
	if aggParam == "" {
		WriteStreamAggHTML(w, rws, rwActive)
		return
	}

	aggNum, err := strconv.Atoi(aggParam)
	if err != nil {
		_, _ = fmt.Fprintf(w, "incorrect parameter 'agg': %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if aggNum >= len(rw.as) {
		_, _ = fmt.Fprintf(w, "not found aggregation with num '%v'", aggNum)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	agg := rw.as[aggNum]

	var as aggrState
	output := r.FormValue("output")
	for _, a := range agg.aggrStates {
		if output == "" {
			as = a
			break
		}
		if a.getOutputName() == output {
			as = a
			break
		}
	}
	if as == nil {
		_, _ = fmt.Fprintf(w, "not found output '%v'", output)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	limitNum := 1000
	limitParam := r.FormValue("limit")
	if limitParam != "" {
		limitNum, err = strconv.Atoi(limitParam)
		if err != nil {
			_, _ = fmt.Fprintf(w, "incorrect parameter 'limit': %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	filter, err := url.QueryUnescape(r.FormValue("filter"))
	if err != nil {
		_, _ = fmt.Fprintf(w, "incorrect parameter 'filter': %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	WriteStreamAggOutputStateHTML(w, rwActive, aggNum, agg, as, limitNum, filter)
}
