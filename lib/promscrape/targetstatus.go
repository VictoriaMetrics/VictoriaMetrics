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
	"github.com/cespare/xxhash/v2"
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
	activeTab := strings.TrimSpace(r.FormValue("active_tab"))
	if accept := r.Header.Get("Accept"); strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tsmGlobal.WriteTargetsHTML(w, showOnlyUnhealthy, endpointSearch, labelSearch, activeTab)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		tsmGlobal.WriteTargetsPlain(w, showOriginalLabels, showOnlyUnhealthy, endpointSearch, labelSearch)
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

type activeKeyStatus struct {
	key string
	st  targetStatus
}

func (tsm *targetStatusMap) getActiveKeyStatuses() []activeKeyStatus {
	tsm.mu.Lock()
	kss := make([]activeKeyStatus, 0, len(tsm.m))
	for sw, st := range tsm.m {
		key := promLabelsString(sw.Config.OriginalLabels)
		kss = append(kss, activeKeyStatus{
			key: key,
			st:  *st,
		})
	}
	tsm.mu.Unlock()

	sort.Slice(kss, func(i, j int) bool {
		return kss[i].key < kss[j].key
	})
	return kss
}

// WriteActiveTargetsJSON writes `activeTargets` contents to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func (tsm *targetStatusMap) WriteActiveTargetsJSON(w io.Writer) {
	kss := tsm.getActiveKeyStatuses()
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

type (
	droppedTargets struct {
		mu              sync.Mutex
		m               map[uint64]droppedTarget
		lastCleanupTime uint64
	}
	droppedTarget struct {
		originalLabels []prompbmarshal.Label
		deadline       uint64
	}
	droppedKeyStatus struct {
		key            string
		originalLabels []prompbmarshal.Label
	}
)

func (dt *droppedTargets) getDroppedKeyStatuses() []droppedKeyStatus {
	dt.mu.Lock()
	kss := make([]droppedKeyStatus, 0, len(dt.m))
	for _, v := range dt.m {
		key := promLabelsString(v.originalLabels)
		kss = append(kss, droppedKeyStatus{
			key:            key,
			originalLabels: v.originalLabels,
		})
	}
	dt.mu.Unlock()

	sort.Slice(kss, func(i, j int) bool {
		return kss[i].key < kss[j].key
	})
	return kss
}

func (dt *droppedTargets) Register(originalLabels []prompbmarshal.Label) {
	// It is better to have hash collisions instead of spending additional CPU on promLabelsString() call.
	key := labelsHash(originalLabels)
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

func labelsHash(labels []prompbmarshal.Label) uint64 {
	d := xxhashPool.Get().(*xxhash.Digest)
	for _, label := range labels {
		_, _ = d.WriteString(label.Name)
		_, _ = d.WriteString(label.Value)
	}
	h := d.Sum64()
	d.Reset()
	xxhashPool.Put(d)
	return h
}

var xxhashPool = &sync.Pool{
	New: func() interface{} {
		return xxhash.New()
	},
}

// WriteDroppedTargetsJSON writes `droppedTargets` contents to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func (dt *droppedTargets) WriteDroppedTargetsJSON(w io.Writer) {
	kss := dt.getDroppedKeyStatuses()
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
	m: make(map[uint64]droppedTarget),
}

type jobTargetsStatuses struct {
	job           string
	upCount       int
	targetsTotal  int
	targetsStatus []targetStatus
}

func (tsm *targetStatusMap) getTargetsStatusByJob(endpointSearch, labelSearch string) ([]jobTargetsStatuses, []string, error) {
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
	var err error
	jts, err = filterTargets(jts, endpointSearch, labelSearch)
	if len(endpointSearch) > 0 || len(labelSearch) > 0 {
		// Do not show empty jobs if target filters are set.
		emptyJobs = nil
	}
	return jts, emptyJobs, err
}

func filterTargetsByEndpoint(jts []jobTargetsStatuses, searchQuery string) ([]jobTargetsStatuses, error) {
	if searchQuery == "" {
		return jts, nil
	}
	finder, err := regexp.Compile(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", searchQuery, err)
	}
	var jtsFiltered []jobTargetsStatuses
	for _, job := range jts {
		var tss []targetStatus
		for _, ts := range job.targetsStatus {
			if finder.MatchString(ts.sw.Config.ScrapeURL) {
				tss = append(tss, ts)
			}
		}
		if len(tss) == 0 {
			// Skip jobs with zero targets after filtering, so users could see only the requested targets
			continue
		}
		job.targetsStatus = tss
		jtsFiltered = append(jtsFiltered, job)
	}
	return jtsFiltered, nil
}

func filterTargetsByLabels(jts []jobTargetsStatuses, searchQuery string) ([]jobTargetsStatuses, error) {
	if searchQuery == "" {
		return jts, nil
	}
	var ie promrelabel.IfExpression
	if err := ie.Parse(searchQuery); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", searchQuery, err)
	}
	var jtsFiltered []jobTargetsStatuses
	for _, job := range jts {
		var tss []targetStatus
		for _, ts := range job.targetsStatus {
			if ie.Match(ts.sw.Config.Labels) {
				tss = append(tss, ts)
			}
		}
		if len(tss) == 0 {
			// Skip jobs with zero targets after filtering, so users could see only the requested targets
			continue
		}
		job.targetsStatus = tss
		jtsFiltered = append(jtsFiltered, job)
	}
	return jtsFiltered, nil
}

func filterTargets(jts []jobTargetsStatuses, endpointQuery, labelQuery string) ([]jobTargetsStatuses, error) {
	var err error
	jts, err = filterTargetsByEndpoint(jts, endpointQuery)
	if err != nil {
		return nil, err
	}
	jts, err = filterTargetsByLabels(jts, labelQuery)
	if err != nil {
		return nil, err
	}
	return jts, nil
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
func (tsm *targetStatusMap) WriteTargetsHTML(w io.Writer, showOnlyUnhealthy bool, endpointSearch, labelSearch, activeTab string) {
	droppedKeyStatuses := droppedTargetsMap.getDroppedKeyStatuses()
	jss, emptyJobs, err := tsm.getTargetsStatusByJob(endpointSearch, labelSearch)
	WriteTargetsResponseHTML(w, jss, emptyJobs, showOnlyUnhealthy, endpointSearch, labelSearch, activeTab, droppedKeyStatuses, err)
}

// WriteTargetsPlain writes targets grouped by job into writer w in plain text,
// accept filter to show original labels.
func (tsm *targetStatusMap) WriteTargetsPlain(w io.Writer, showOriginalLabels, showOnlyUnhealthy bool, endpointSearch, labelSearch string) {
	jss, emptyJobs, err := tsm.getTargetsStatusByJob(endpointSearch, labelSearch)
	WriteTargetsResponsePlain(w, jss, emptyJobs, showOriginalLabels, showOnlyUnhealthy, err)
}
