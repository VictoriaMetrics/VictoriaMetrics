package promscrape

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"gopkg.in/yaml.v2"
)

var maxDroppedTargets = flag.Int("promscrape.maxDroppedTargets", 1000, "The maximum number of droppedTargets to show at /api/v1/targets page. "+
	"Increase this value if your setup drops more scrape targets during relabeling and you need investigating labels for all the dropped targets. "+
	"Note that the increased number of tracked dropped targets may result in increased memory usage")

var tsmGlobal = newTargetStatusMap()

// WriteTargetResponse serves requests to /target_response?id=<id>
//
// It fetches response for the given target id and returns it.
func WriteTargetResponse(w http.ResponseWriter, r *http.Request) error {
	targetID := r.FormValue("id")
	sw := tsmGlobal.getScrapeWorkByTargetID(targetID)
	if sw == nil {
		return fmt.Errorf("cannot find target for id=%s", targetID)
	}
	data, err := sw.getTargetResponse()
	if err != nil {
		return fmt.Errorf("cannot fetch response from id=%s: %w", targetID, err)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = w.Write(data)
	return err
}

// WriteHumanReadableTargetsStatus writes human-readable status for all the scrape targets to w according to r.
func WriteHumanReadableTargetsStatus(w http.ResponseWriter, r *http.Request) {
	showOriginalLabels, _ := strconv.ParseBool(r.FormValue("show_original_labels"))
	showOnlyUnhealthy, _ := strconv.ParseBool(r.FormValue("show_only_unhealthy"))
	endpointSearch := strings.TrimSpace(r.FormValue("endpoint_search"))
	labelSearch := strings.TrimSpace(r.FormValue("label_search"))
	if accept := r.Header.Get("Accept"); strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tsmGlobal.WriteTargetsHTML(w, showOnlyUnhealthy, endpointSearch, labelSearch)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		tsmGlobal.WriteTargetsPlain(w, showOriginalLabels)
	}
}

// WriteAPIV1Targets writes /api/v1/targets to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func WriteAPIV1Targets(w io.Writer, state string) {
	if state == "" {
		state = "any"
	}
	fmt.Fprintf(w, `{"status":"success","data":{"activeTargets":`)
	if state == "active" || state == "any" {
		tsmGlobal.WriteActiveTargetsJSON(w)
	} else {
		fmt.Fprintf(w, `[]`)
	}
	fmt.Fprintf(w, `,"droppedTargets":`)
	if state == "dropped" || state == "any" {
		droppedTargetsMap.WriteDroppedTargetsJSON(w)
	} else {
		fmt.Fprintf(w, `[]`)
	}
	fmt.Fprintf(w, `}}`)
}

type targetStatusMap struct {
	mu       sync.Mutex
	m        map[*scrapeWork]*targetStatus
	jobNames []string
}

func newTargetStatusMap() *targetStatusMap {
	return &targetStatusMap{
		m: make(map[*scrapeWork]*targetStatus),
	}
}

func (tsm *targetStatusMap) Reset() {
	tsm.mu.Lock()
	tsm.m = make(map[*scrapeWork]*targetStatus)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) registerJobNames(jobNames []string) {
	tsm.mu.Lock()
	tsm.jobNames = append(tsm.jobNames[:0], jobNames...)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Register(sw *scrapeWork) {
	tsm.mu.Lock()
	tsm.m[sw] = &targetStatus{
		sw: sw,
	}
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Unregister(sw *scrapeWork) {
	tsm.mu.Lock()
	delete(tsm.m, sw)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Update(sw *scrapeWork, group string, up bool, scrapeTime, scrapeDuration int64, samplesScraped int, err error) {
	tsm.mu.Lock()
	ts := tsm.m[sw]
	if ts == nil {
		ts = &targetStatus{
			sw: sw,
		}
		tsm.m[sw] = ts
	}
	ts.up = up
	ts.scrapeGroup = group
	ts.scrapeTime = scrapeTime
	ts.scrapeDuration = scrapeDuration
	ts.samplesScraped = samplesScraped
	ts.scrapesTotal++
	if !up {
		ts.scrapesFailed++
	}
	ts.err = err
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) getScrapeWorkByTargetID(targetID string) *scrapeWork {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	for sw := range tsm.m {
		if getTargetID(sw) == targetID {
			return sw
		}
	}
	return nil
}

func getTargetID(sw *scrapeWork) string {
	return fmt.Sprintf("%016x", uintptr(unsafe.Pointer(sw)))
}

// StatusByGroup returns the number of targets with status==up
// for the given group name
func (tsm *targetStatusMap) StatusByGroup(group string, up bool) int {
	var count int
	tsm.mu.Lock()
	for _, st := range tsm.m {
		if st.scrapeGroup == group && st.up == up {
			count++
		}
	}
	tsm.mu.Unlock()
	return count
}

// WriteActiveTargetsJSON writes `activeTargets` contents to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func (tsm *targetStatusMap) WriteActiveTargetsJSON(w io.Writer) {
	tsm.mu.Lock()
	type keyStatus struct {
		key string
		st  targetStatus
	}
	kss := make([]keyStatus, 0, len(tsm.m))
	for sw, st := range tsm.m {
		key := promLabelsString(sw.Config.OriginalLabels)
		kss = append(kss, keyStatus{
			key: key,
			st:  *st,
		})
	}
	tsm.mu.Unlock()

	sort.Slice(kss, func(i, j int) bool {
		return kss[i].key < kss[j].key
	})
	fmt.Fprintf(w, `[`)
	for i, ks := range kss {
		st := ks.st
		fmt.Fprintf(w, `{"discoveredLabels":`)
		writeLabelsJSON(w, st.sw.Config.OriginalLabels)
		fmt.Fprintf(w, `,"labels":`)
		labelsFinalized := promrelabel.FinalizeLabels(nil, st.sw.Config.Labels)
		writeLabelsJSON(w, labelsFinalized)
		fmt.Fprintf(w, `,"scrapePool":%q`, st.sw.Config.Job())
		fmt.Fprintf(w, `,"scrapeUrl":%q`, st.sw.Config.ScrapeURL)
		errMsg := ""
		if st.err != nil {
			errMsg = st.err.Error()
		}
		fmt.Fprintf(w, `,"lastError":%q`, errMsg)
		fmt.Fprintf(w, `,"lastScrape":%q`, time.Unix(st.scrapeTime/1000, (st.scrapeTime%1000)*1e6).Format(time.RFC3339Nano))
		fmt.Fprintf(w, `,"lastScrapeDuration":%g`, (time.Millisecond * time.Duration(st.scrapeDuration)).Seconds())
		fmt.Fprintf(w, `,"lastSamplesScraped":%d`, st.samplesScraped)
		state := "up"
		if !st.up {
			state = "down"
		}
		fmt.Fprintf(w, `,"health":%q}`, state)
		if i+1 < len(kss) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `]`)
}

func writeLabelsJSON(w io.Writer, labels []prompbmarshal.Label) {
	fmt.Fprintf(w, `{`)
	for i, label := range labels {
		fmt.Fprintf(w, "%q:%q", label.Name, label.Value)
		if i+1 < len(labels) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `}`)
}

type targetStatus struct {
	sw             *scrapeWork
	up             bool
	scrapeGroup    string
	scrapeTime     int64
	scrapeDuration int64
	samplesScraped int
	scrapesTotal   int
	scrapesFailed  int
	err            error
}

func (st *targetStatus) getDurationFromLastScrape() time.Duration {
	return time.Since(time.Unix(st.scrapeTime/1000, (st.scrapeTime%1000)*1e6))
}

type droppedTargets struct {
	mu              sync.Mutex
	m               map[string]droppedTarget
	lastCleanupTime uint64
}

type droppedTarget struct {
	originalLabels []prompbmarshal.Label
	deadline       uint64
}

func (dt *droppedTargets) Register(originalLabels []prompbmarshal.Label) {
	key := promLabelsString(originalLabels)
	currentTime := fasttime.UnixTimestamp()
	dt.mu.Lock()
	if k, ok := dt.m[key]; ok {
		k.deadline = currentTime + 10*60
		dt.m[key] = k
	} else if len(dt.m) < *maxDroppedTargets {
		dt.m[key] = droppedTarget{
			originalLabels: originalLabels,
			deadline:       currentTime + 10*60,
		}
	}
	if currentTime-dt.lastCleanupTime > 60 {
		for k, v := range dt.m {
			if currentTime > v.deadline {
				delete(dt.m, k)
			}
		}
		dt.lastCleanupTime = currentTime
	}
	dt.mu.Unlock()
}

// WriteDroppedTargetsJSON writes `droppedTargets` contents to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func (dt *droppedTargets) WriteDroppedTargetsJSON(w io.Writer) {
	dt.mu.Lock()
	type keyStatus struct {
		key            string
		originalLabels []prompbmarshal.Label
	}
	kss := make([]keyStatus, 0, len(dt.m))
	for _, v := range dt.m {
		key := promLabelsString(v.originalLabels)
		kss = append(kss, keyStatus{
			key:            key,
			originalLabels: v.originalLabels,
		})
	}
	dt.mu.Unlock()

	sort.Slice(kss, func(i, j int) bool {
		return kss[i].key < kss[j].key
	})
	fmt.Fprintf(w, `[`)
	for i, ks := range kss {
		fmt.Fprintf(w, `{"discoveredLabels":`)
		writeLabelsJSON(w, ks.originalLabels)
		fmt.Fprintf(w, `}`)
		if i+1 < len(kss) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `]`)
}

var droppedTargetsMap = &droppedTargets{
	m: make(map[string]droppedTarget),
}

type jobTargetsStatuses struct {
	job           string
	upCount       int
	targetsTotal  int
	targetsStatus []targetStatus
}

func (tsm *targetStatusMap) getTargetsStatusByJob() ([]jobTargetsStatuses, []string) {
	byJob := make(map[string][]targetStatus)
	tsm.mu.Lock()
	for _, st := range tsm.m {
		job := st.sw.Config.jobNameOriginal
		byJob[job] = append(byJob[job], *st)
	}
	jobNames := append([]string{}, tsm.jobNames...)
	tsm.mu.Unlock()

	var jts []jobTargetsStatuses
	for job, statuses := range byJob {
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].sw.Config.ScrapeURL < statuses[j].sw.Config.ScrapeURL
		})
		ups := 0
		var targetsStatuses []targetStatus
		for _, ts := range statuses {
			if ts.up {
				ups++
			}
			targetsStatuses = append(targetsStatuses, ts)
		}
		jts = append(jts, jobTargetsStatuses{
			job:           job,
			upCount:       ups,
			targetsTotal:  len(statuses),
			targetsStatus: targetsStatuses,
		})
	}
	sort.Slice(jts, func(i, j int) bool {
		return jts[i].job < jts[j].job
	})
	emptyJobs := getEmptyJobs(jts, jobNames)
	return jts, emptyJobs
}

func filterJobsByEndpoint(jts []jobTargetsStatuses, searchQuery string) ([]jobTargetsStatuses, error) {
	if searchQuery == "" {
		return jts, nil
	}
	finder, err := regexp.Compile(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("cannot parse regexp for %s: %w", searchQuery, err)
	}

	var filteredJts []jobTargetsStatuses
	for _, job := range jts {
		var ts []targetStatus
		for _, targetStatus := range job.targetsStatus {
			if finder.MatchString(targetStatus.sw.Config.ScrapeURL) {
				ts = append(ts, targetStatus)
			}
		}
		job.targetsStatus = ts
		filteredJts = append(filteredJts, job)
		ts = ts[:0]
	}
	return filteredJts, nil
}

func filterJobsByLabels(jts []jobTargetsStatuses, searchQuery string) ([]jobTargetsStatuses, error) {
	if searchQuery == "" {
		return jts, nil
	}
	searchQuery = strings.TrimRight(strings.TrimLeft(searchQuery, "'"), "'")

	var ie promrelabel.IfExpression
	if err := yaml.UnmarshalStrict([]byte("'"+searchQuery+"'"), &ie); err != nil {
		return nil, fmt.Errorf("unexpected error during unmarshal search query: %s", err)
	}

	var filteredJts []jobTargetsStatuses
	for _, job := range jts {
		var ts []targetStatus
		for _, targetStatus := range job.targetsStatus {
			labels, err := parseMetricWithLabels(targetStatus.sw.Config.LabelsString())
			if err != nil {
				return nil, fmt.Errorf("unexpected error during parse search query: %s", err)
			}
			if ie.Match(labels) {
				ts = append(ts, targetStatus)
			}
		}
		job.targetsStatus = ts
		filteredJts = append(filteredJts, job)
		ts = ts[:0]
	}
	return filteredJts, nil
}

func filterJobs(jts []jobTargetsStatuses, endpointQuery, labelQuery string) ([]jobTargetsStatuses, error) {
	jobsByEndpoint, err := filterJobsByEndpoint(jts, endpointQuery)
	if err != nil {
		return nil, err
	}
	jobsByLabels, err := filterJobsByLabels(jobsByEndpoint, labelQuery)
	if err != nil {
		return nil, err
	}
	return jobsByLabels, nil
}

func parseMetricWithLabels(labels string) ([]prompbmarshal.Label, error) {
	// add metrics and a value to labels, so it could be parsed by prometheus protocol parser.
	s := "vmagent" + labels + " 123"
	var rows prometheus.Rows
	var err error
	rows.UnmarshalWithErrLogger(s, func(s string) {
		err = fmt.Errorf("error during metric parse: %s", s)
	})
	if err != nil {
		return nil, err
	}
	if len(rows.Rows) != 1 {
		return nil, fmt.Errorf("unexpected number of rows parsed; got %d; want 1", len(rows.Rows))
	}
	r := rows.Rows[0]
	var lfs []prompbmarshal.Label
	if r.Metric != "" {
		lfs = append(lfs, prompbmarshal.Label{
			Name:  "__name__",
			Value: r.Metric,
		})
	}
	for _, tag := range r.Tags {
		lfs = append(lfs, prompbmarshal.Label{
			Name:  tag.Key,
			Value: tag.Value,
		})
	}
	return lfs, nil
}

func getEmptyJobs(jts []jobTargetsStatuses, jobNames []string) []string {
	jobNamesMap := make(map[string]struct{}, len(jobNames))
	for _, jobName := range jobNames {
		jobNamesMap[jobName] = struct{}{}
	}
	for i := range jts {
		delete(jobNamesMap, jts[i].job)
	}
	emptyJobs := make([]string, 0, len(jobNamesMap))
	for k := range jobNamesMap {
		emptyJobs = append(emptyJobs, k)
	}
	sort.Strings(emptyJobs)
	return emptyJobs
}

// WriteTargetsHTML writes targets status grouped by job into writer w in html table,
// accepts filter to show only unhealthy targets.
func (tsm *targetStatusMap) WriteTargetsHTML(w io.Writer, showOnlyUnhealthy bool, endpointSearch, labelSearch string) {
	jss, emptyJobs := tsm.getTargetsStatusByJob()
	var err error
	jss, err = filterJobs(jss, endpointSearch, labelSearch)
	WriteTargetsResponseHTML(w, jss, emptyJobs, showOnlyUnhealthy, endpointSearch, labelSearch, err)
}

// WriteTargetsPlain writes targets grouped by job into writer w in plain text,
// accept filter to show original labels.
func (tsm *targetStatusMap) WriteTargetsPlain(w io.Writer, showOriginalLabels bool) {
	jss, emptyJobs := tsm.getTargetsStatusByJob()
	WriteTargetsResponsePlain(w, jss, emptyJobs, showOriginalLabels)
}
