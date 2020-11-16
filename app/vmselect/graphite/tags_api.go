package graphite

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/metrics"
)

// TagValuesHandler implements /tags/<tag_name> endpoint from Graphite Tags API.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags
func TagValuesHandler(startTime time.Time, tagName string, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	tagValues, err := netstorage.GetGraphiteTagValues(tagName, filter, limit, deadline)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTagValuesResponse(bw, tagName, tagValues)
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
func TagsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	limit, err := getInt(r, "limit")
	if err != nil {
		return err
	}
	filter := r.FormValue("filter")
	labels, err := netstorage.GetGraphiteTags(filter, limit, deadline)
	if err != nil {
		return err
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

func getInt(r *http.Request, argName string) (int, error) {
	argValue := r.FormValue(argName)
	if len(argValue) == 0 {
		return 0, nil
	}
	n, err := strconv.Atoi(argValue)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q=%q: %w", argName, argValue, err)
	}
	return n, nil
}
