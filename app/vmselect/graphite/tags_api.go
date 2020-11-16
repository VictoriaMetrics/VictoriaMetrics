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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/metrics"
)

// TagValuesHandler implements /tags/<tag_name> endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagValuesHandler(startTime time.Time, at *auth.Token, tagName string, w http.ResponseWriter, r *http.Request) error {
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
	denyPartialResponse := searchutils.GetDenyPartialResponse(r)
	tagValues, isPartial, err := netstorage.GetGraphiteTagValues(at, denyPartialResponse, tagName, limit, deadline)
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	if len(filter) > 0 {
		tagValues, err = applyRegexpFilter(filter, tagValues)
		if err != nil {
			return err
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagValuesResponse(bw, isPartial, tagName, tagValues)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagValuesDuration.UpdateDuration(startTime)
	return nil
}

var tagValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags/<tag_name>"}`)

// TagsHandler implements /tags endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagsHandler(startTime time.Time, at *auth.Token, w http.ResponseWriter, r *http.Request) error {
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
	denyPartialResponse := searchutils.GetDenyPartialResponse(r)
	labels, isPartial, err := netstorage.GetGraphiteTags(at, denyPartialResponse, limit, deadline)
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	if len(filter) > 0 {
		labels, err = applyRegexpFilter(filter, labels)
		if err != nil {
			return err
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagsResponse(bw, isPartial, labels)
	if err := bw.Flush(); err != nil {
		return err
	}
	tagsDuration.UpdateDuration(startTime)
	return nil
}

var tagsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/tags"}`)

func applyRegexpFilter(filter string, ss []string) ([]string, error) {
	// Anchor filter regexp to the beginning of the string as Graphite does.
	// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/localdatabase.py#L157
	filter = "^(?:" + filter + ")"
	re, err := regexp.Compile(filter)
	if err != nil {
		return nil, fmt.Errorf("cannot parse regexp filter=%q: %w", filter, err)
	}
	dst := ss[:0]
	for _, s := range ss {
		if re.MatchString(s) {
			dst = append(dst, s)
		}
	}
	return dst, nil
}
