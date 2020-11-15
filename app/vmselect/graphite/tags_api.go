package graphite

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/metrics"
)

// TagsHandler implements handler for /tags endpoint.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit := 0
	if limitStr := r.FormValue("limit"); len(limitStr) > 0 {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return fmt.Errorf("cannot parse limit=%q: %w", limit, err)
		}
	}
	labels, err := netstorage.GetGraphiteTags(limit, deadline)
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	if len(filter) > 0 {
		// Anchor filter regexp to the beginning of the string as Graphite does.
		// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/localdatabase.py#L157
		filter = "^(?:" + filter + ")"
		re, err := regexp.Compile(filter)
		if err != nil {
			return fmt.Errorf("cannot parse regexp filter=%q: %w", filter, err)
		}
		dst := labels[:0]
		for _, label := range labels {
			if re.MatchString(label) {
				dst = append(dst, label)
			}
		}
		labels = dst
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsResponse(bw, labels)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsDuration.UpdateDuration(startTime)
	return nil
}

var tagsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags"}`)
