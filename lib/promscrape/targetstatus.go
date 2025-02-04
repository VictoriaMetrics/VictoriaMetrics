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

	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

var maxDroppedTargets = flag.Int("promscrape.maxDroppedTargets", 10000, "The maximum number of droppedTargets to show at /api/v1/targets page. "+
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
	filter := getRequestFilter(r)
	tsr := tsmGlobal.getTargetsStatusByJob(filter)
	if accept := r.Header.Get("Accept"); strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		WriteTargetsResponseHTML(w, tsr, filter)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		WriteTargetsResponsePlain(w, tsr, filter)
	}
}

// WriteServiceDiscovery writes /service-discovery response to w similar to http://demo.robustperception.io:9090/service-discovery
func WriteServiceDiscovery(w http.ResponseWriter, r *http.Request) {
	filter := getRequestFilter(r)
	tsr := tsmGlobal.getTargetsStatusByJob(filter)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	WriteServiceDiscoveryResponse(w, tsr, filter)
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

	// the current number of `up` targets in the given jobName
	upByJob map[string]int

	// the current number of `down` targets in the given jobName
	downByJob map[string]int
}

func newTargetStatusMap() *targetStatusMap {
	return &targetStatusMap{
		m:         make(map[*scrapeWork]*targetStatus),
		upByJob:   make(map[string]int),
		downByJob: make(map[string]int),
	}
}

func (tsm *targetStatusMap) registerJobNames(jobNames []string) {
	tsm.mu.Lock()
	tsm.registerJobsMetricsLocked(tsm.jobNames, jobNames)
	tsm.jobNames = append(tsm.jobNames[:0], jobNames...)
	tsm.mu.Unlock()
}

// registerJobsMetricsLocked registers metrics for new jobs and unregisters metrics for removed jobs
//
// tsm.mu must be locked when calling this function.
func (tsm *targetStatusMap) registerJobsMetricsLocked(prevJobNames, currentJobNames []string) {
	prevNames := make(map[string]struct{}, len(prevJobNames))
	currentNames := make(map[string]struct{}, len(currentJobNames))
	for _, jobName := range currentJobNames {
		currentNames[jobName] = struct{}{}
	}
	for _, jobName := range prevJobNames {
		prevNames[jobName] = struct{}{}
		if _, ok := currentNames[jobName]; !ok {
			metrics.UnregisterMetric(fmt.Sprintf(`vm_promscrape_scrape_pool_targets{scrape_job=%q, status="up"}`, jobName))
			metrics.UnregisterMetric(fmt.Sprintf(`vm_promscrape_scrape_pool_targets{scrape_job=%q, status="down"}`, jobName))
		}
	}

	for _, jobName := range currentJobNames {
		if _, ok := prevNames[jobName]; ok {
			continue
		}
		jobNameLocal := jobName
		_ = metrics.NewGauge(fmt.Sprintf(`vm_promscrape_scrape_pool_targets{scrape_job=%q, status="up"}`, jobName), func() float64 {
			tsm.mu.Lock()
			n := tsm.upByJob[jobNameLocal]
			tsm.mu.Unlock()
			return float64(n)
		})
		_ = metrics.NewGauge(fmt.Sprintf(`vm_promscrape_scrape_pool_targets{scrape_job=%q, status="down"}`, jobName), func() float64 {
			tsm.mu.Lock()
			n := tsm.downByJob[jobNameLocal]
			tsm.mu.Unlock()
			return float64(n)
		})
	}
}

func (tsm *targetStatusMap) Register(sw *scrapeWork) {
	jobName := sw.Config.jobNameOriginal

	tsm.mu.Lock()
	tsm.m[sw] = &targetStatus{
		sw: sw,
	}
	tsm.downByJob[jobName]++
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Unregister(sw *scrapeWork) {
	jobName := sw.Config.jobNameOriginal

	tsm.mu.Lock()
	ts, ok := tsm.m[sw]
	if !ok {
		logger.Panicf("BUG: missing Register() call for the target %q", jobName)
	}
	if ts.up {
		tsm.upByJob[jobName]--
	} else {
		tsm.downByJob[jobName]--
	}
	delete(tsm.m, sw)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Update(sw *scrapeWork, up bool, scrapeTime, scrapeDuration int64, scrapeResponseSize, samplesScraped int, err error) {
	jobName := sw.Config.jobNameOriginal

	tsm.mu.Lock()
	ts, ok := tsm.m[sw]
	if !ok {
		logger.Panicf("BUG: missing Register() call for the target %q", jobName)
	}
	if up && !ts.up {
		tsm.upByJob[jobName]++
		tsm.downByJob[jobName]--
	} else if !up && ts.up {
		tsm.upByJob[jobName]--
		tsm.downByJob[jobName]++
	}
	ts.up = up
	ts.scrapeTime = scrapeTime
	ts.scrapeDuration = scrapeDuration
	ts.samplesScraped = samplesScraped
	ts.scrapeResponseSize = scrapeResponseSize
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
		// The target is uniquely identified by a pointer to its original labels.
		if getLabelsID(sw.Config.OriginalLabels) == targetID {
			return sw
		}
	}
	return nil
}

func getLabelsID(labels *promutils.Labels) string {
	return fmt.Sprintf("%016x", uintptr(unsafe.Pointer(labels)))
}

// StatusByGroup returns the number of targets with status==up
// for the given group name
func (tsm *targetStatusMap) StatusByGroup(group string, up bool) int {
	var count int
	tsm.mu.Lock()
	for _, ts := range tsm.m {
		if ts.sw.ScrapeGroup == group && ts.up == up {
			count++
		}
	}
	tsm.mu.Unlock()
	return count
}

func (tsm *targetStatusMap) getActiveTargetStatuses() []targetStatus {
	tsm.mu.Lock()
	tss := make([]targetStatus, 0, len(tsm.m))
	for _, ts := range tsm.m {
		tss = append(tss, *ts)
	}
	tsm.mu.Unlock()
	// Sort discovered targets by __address__ label, so they stay in consistent order across calls
	sort.Slice(tss, func(i, j int) bool {
		addr1 := tss[i].sw.Config.OriginalLabels.Get("__address__")
		addr2 := tss[j].sw.Config.OriginalLabels.Get("__address__")
		return addr1 < addr2
	})
	return tss
}

// WriteActiveTargetsJSON writes `activeTargets` contents to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func (tsm *targetStatusMap) WriteActiveTargetsJSON(w io.Writer) {
	tss := tsm.getActiveTargetStatuses()
	fmt.Fprintf(w, `[`)
	for i, ts := range tss {
		fmt.Fprintf(w, `{"discoveredLabels":`)
		writeLabelsJSON(w, ts.sw.Config.OriginalLabels)
		fmt.Fprintf(w, `,"labels":`)
		writeLabelsJSON(w, ts.sw.Config.Labels)
		fmt.Fprintf(w, `,"scrapePool":%s`, stringsutil.JSONString(ts.sw.Config.Job()))
		fmt.Fprintf(w, `,"scrapeUrl":%s`, stringsutil.JSONString(ts.sw.Config.ScrapeURL))
		errMsg := ""
		if ts.err != nil {
			errMsg = ts.err.Error()
		}
		fmt.Fprintf(w, `,"lastError":%s`, stringsutil.JSONString(errMsg))
		fmt.Fprintf(w, `,"lastScrape":"%s"`, time.Unix(ts.scrapeTime/1000, (ts.scrapeTime%1000)*1e6).Format(time.RFC3339Nano))
		fmt.Fprintf(w, `,"lastScrapeDuration":%g`, (time.Millisecond * time.Duration(ts.scrapeDuration)).Seconds())
		fmt.Fprintf(w, `,"lastSamplesScraped":%d`, ts.samplesScraped)
		state := "up"
		if !ts.up {
			state = "down"
		}
		fmt.Fprintf(w, `,"health":%s}`, stringsutil.JSONString(state))
		if i+1 < len(tss) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `]`)
}

func writeLabelsJSON(w io.Writer, labels *promutils.Labels) {
	fmt.Fprintf(w, `{`)
	labelsList := labels.GetLabels()
	for i, label := range labelsList {
		fmt.Fprintf(w, "%s:%s", stringsutil.JSONString(label.Name), stringsutil.JSONString(label.Value))
		if i+1 < len(labelsList) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `}`)
}

type targetStatus struct {
	sw                 *scrapeWork
	up                 bool
	scrapeTime         int64
	scrapeDuration     int64
	scrapeResponseSize int
	samplesScraped     int
	scrapesTotal       int
	scrapesFailed      int
	err                error
}

func (ts *targetStatus) getDurationFromLastScrape() string {
	if ts.scrapeTime <= 0 {
		return "never scraped"
	}
	d := time.Since(time.Unix(ts.scrapeTime/1000, (ts.scrapeTime%1000)*1e6))
	return fmt.Sprintf("%.3fs ago", d.Seconds())
}

func (ts *targetStatus) getSizeFromLastScrape() string {
	if ts.scrapeResponseSize <= 0 {
		return "never scraped"
	}
	return fmt.Sprintf("%.3fKiB", float64(ts.scrapeResponseSize)/1024)
}

type droppedTargets struct {
	mu sync.Mutex
	m  map[uint64]droppedTarget

	// totalTargets contains the total number of dropped targets registered via Register() call.
	totalTargets int
}

type droppedTarget struct {
	originalLabels    *promutils.Labels
	relabelConfigs    *promrelabel.ParsedConfigs
	dropReason        targetDropReason
	clusterMemberNums []int
}

type targetDropReason string

const (
	targetDropReasonRelabeling       = targetDropReason("relabeling")         // target dropped because of relabeling
	targetDropReasonMissingScrapeURL = targetDropReason("missing scrape URL") // target dropped because of missing scrape URL
	targetDropReasonDuplicate        = targetDropReason("duplicate")          // target with the given set of labels already exists
	targetDropReasonSharding         = targetDropReason("sharding")           // target is dropped becase of sharding https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets
)

func (dt *droppedTargets) getTargetsList() []droppedTarget {
	dt.mu.Lock()
	dts := make([]droppedTarget, 0, len(dt.m))
	for _, v := range dt.m {
		dts = append(dts, v)
	}
	dt.mu.Unlock()
	// Sort discovered targets by __address__ label, so they stay in consistent order across calls
	sort.Slice(dts, func(i, j int) bool {
		addr1 := dts[i].originalLabels.Get("__address__")
		addr2 := dts[j].originalLabels.Get("__address__")
		return addr1 < addr2
	})
	return dts
}

// Register registers dropped target with the given originalLabels.
//
// The relabelConfigs must contain relabel configs, which were applied to originalLabels.
// The reason must contain the reason why the target has been dropped.
func (dt *droppedTargets) Register(originalLabels *promutils.Labels, relabelConfigs *promrelabel.ParsedConfigs, reason targetDropReason, clusterMemberNums []int) {
	if originalLabels == nil {
		// Do not register target without originalLabels. This is the case when *dropOriginalLabels is set to true.
		return
	}
	// It is better to have hash collisions instead of spending additional CPU on originalLabels.String() call.
	key := labelsHash(originalLabels)
	dt.mu.Lock()
	if _, ok := dt.m[key]; !ok {
		dt.totalTargets++
	}
	dt.m[key] = droppedTarget{
		originalLabels:    originalLabels,
		relabelConfigs:    relabelConfigs,
		dropReason:        reason,
		clusterMemberNums: clusterMemberNums,
	}
	if len(dt.m) > *maxDroppedTargets {
		for k := range dt.m {
			delete(dt.m, k)
			if len(dt.m) <= *maxDroppedTargets {
				break
			}
		}
	}
	dt.mu.Unlock()
}

func (dt *droppedTargets) getTotalTargets() int {
	dt.mu.Lock()
	n := dt.totalTargets
	dt.mu.Unlock()
	return n
}

func labelsHash(labels *promutils.Labels) uint64 {
	d := xxhashPool.Get().(*xxhash.Digest)
	for _, label := range labels.GetLabels() {
		_, _ = d.WriteString(label.Name)
		_, _ = d.WriteString(label.Value)
	}
	h := d.Sum64()
	d.Reset()
	xxhashPool.Put(d)
	return h
}

var xxhashPool = &sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

// WriteDroppedTargetsJSON writes `droppedTargets` contents to w according to https://prometheus.io/docs/prometheus/latest/querying/api/#targets
func (dt *droppedTargets) WriteDroppedTargetsJSON(w io.Writer) {
	dts := dt.getTargetsList()
	fmt.Fprintf(w, `[`)
	for i, dt := range dts {
		fmt.Fprintf(w, `{"discoveredLabels":`)
		writeLabelsJSON(w, dt.originalLabels)
		fmt.Fprintf(w, `}`)
		if i+1 < len(dts) {
			fmt.Fprintf(w, `,`)
		}
	}
	fmt.Fprintf(w, `]`)
}

var droppedTargetsMap = &droppedTargets{
	m: make(map[uint64]droppedTarget),
}

type jobTargetsStatuses struct {
	jobName       string
	upCount       int
	targetsTotal  int
	targetsStatus []targetStatus
}

func (tsm *targetStatusMap) getTargetsStatusByJob(filter *requestFilter) *targetsStatusResult {
	byJob := make(map[string][]targetStatus)
	tsm.mu.Lock()
	for _, ts := range tsm.m {
		jobName := ts.sw.Config.jobNameOriginal
		if filter.originalJobName != "" && jobName != filter.originalJobName {
			continue
		}
		byJob[jobName] = append(byJob[jobName], *ts)
	}
	jobNames := append([]string{}, tsm.jobNames...)
	tsm.mu.Unlock()

	var jts []*jobTargetsStatuses
	for jobName, statuses := range byJob {
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].sw.Config.ScrapeURL < statuses[j].sw.Config.ScrapeURL
		})
		ups := 0
		var targetsStatuses []targetStatus
		for _, ts := range statuses {
			if ts.up {
				ups++
			}
			if filter.showOnlyUnhealthy && ts.up {
				continue
			}
			targetsStatuses = append(targetsStatuses, ts)
		}
		if filter.showOnlyUnhealthy && len(targetsStatuses) == 0 {
			continue
		}
		jts = append(jts, &jobTargetsStatuses{
			jobName:       jobName,
			upCount:       ups,
			targetsTotal:  len(statuses),
			targetsStatus: targetsStatuses,
		})
	}
	sort.Slice(jts, func(i, j int) bool {
		return jts[i].jobName < jts[j].jobName
	})
	emptyJobs := getEmptyJobs(jts, jobNames)
	var err error
	jts, err = filterTargets(jts, filter.endpointSearch, filter.labelSearch)
	if len(filter.endpointSearch) > 0 || len(filter.labelSearch) > 0 || filter.showOnlyUnhealthy {
		// Do not show empty jobs if target filters are set.
		emptyJobs = nil
	}
	dts := droppedTargetsMap.getTargetsList()
	return &targetsStatusResult{
		hasOriginalLabels:  !*dropOriginalLabels,
		jobTargetsStatuses: jts,
		droppedTargets:     dts,
		emptyJobs:          emptyJobs,
		err:                err,
	}
}

func filterTargetsByEndpoint(jts []*jobTargetsStatuses, searchQuery string) ([]*jobTargetsStatuses, error) {
	if searchQuery == "" {
		return jts, nil
	}
	finder, err := regexp.Compile(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", searchQuery, err)
	}
	var jtsFiltered []*jobTargetsStatuses
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

func filterTargetsByLabels(jts []*jobTargetsStatuses, searchQuery string) ([]*jobTargetsStatuses, error) {
	if searchQuery == "" {
		return jts, nil
	}
	var ie promrelabel.IfExpression
	if err := ie.Parse(searchQuery); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", searchQuery, err)
	}
	var jtsFiltered []*jobTargetsStatuses
	for _, job := range jts {
		var tss []targetStatus
		for _, ts := range job.targetsStatus {
			labels := ts.sw.Config.Labels.GetLabels()
			if ie.Match(labels) {
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

func filterTargets(jts []*jobTargetsStatuses, endpointQuery, labelQuery string) ([]*jobTargetsStatuses, error) {
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

func getEmptyJobs(jts []*jobTargetsStatuses, jobNames []string) []string {
	jobNamesMap := make(map[string]struct{}, len(jobNames))
	for _, jobName := range jobNames {
		jobNamesMap[jobName] = struct{}{}
	}
	for i := range jts {
		delete(jobNamesMap, jts[i].jobName)
	}
	emptyJobs := make([]string, 0, len(jobNamesMap))
	for k := range jobNamesMap {
		emptyJobs = append(emptyJobs, k)
	}
	sort.Strings(emptyJobs)
	return emptyJobs
}

type requestFilter struct {
	showOriginalLabels bool
	showOnlyUnhealthy  bool
	endpointSearch     string
	labelSearch        string
	originalJobName    string
}

func getRequestFilter(r *http.Request) *requestFilter {
	showOriginalLabels, _ := strconv.ParseBool(r.FormValue("show_original_labels"))
	showOnlyUnhealthy, _ := strconv.ParseBool(r.FormValue("show_only_unhealthy"))
	endpointSearch := strings.TrimSpace(r.FormValue("endpoint_search"))
	labelSearch := strings.TrimSpace(r.FormValue("label_search"))
	return &requestFilter{
		showOriginalLabels: showOriginalLabels,
		showOnlyUnhealthy:  showOnlyUnhealthy,
		endpointSearch:     endpointSearch,
		labelSearch:        labelSearch,
	}
}

type targetsStatusResult struct {
	hasOriginalLabels  bool
	jobTargetsStatuses []*jobTargetsStatuses
	droppedTargets     []droppedTarget
	emptyJobs          []string
	err                error
}

type targetLabels struct {
	up                bool
	originalLabels    *promutils.Labels
	labels            *promutils.Labels
	dropReason        targetDropReason
	clusterMemberNums []int
}
type targetLabelsByJob struct {
	jobName        string
	targets        []targetLabels
	activeTargets  int
	droppedTargets int
}

func getMetricRelabelContextByTargetID(targetID string) (*promrelabel.ParsedConfigs, *promutils.Labels, bool) {
	tsmGlobal.mu.Lock()
	defer tsmGlobal.mu.Unlock()

	for sw := range tsmGlobal.m {
		// The target is uniquely identified by a pointer to its original labels.
		if getLabelsID(sw.Config.OriginalLabels) == targetID {
			return sw.Config.MetricRelabelConfigs, sw.Config.Labels, true
		}
	}
	return nil, nil, false
}

func getTargetRelabelContextByTargetID(targetID string) (*promrelabel.ParsedConfigs, *promutils.Labels, bool) {
	var relabelConfigs *promrelabel.ParsedConfigs
	var labels *promutils.Labels
	found := false

	// Search for relabel context in tsmGlobal (aka active targets)
	tsmGlobal.mu.Lock()
	for sw := range tsmGlobal.m {
		// The target is uniquely identified by a pointer to its original labels.
		if getLabelsID(sw.Config.OriginalLabels) == targetID {
			relabelConfigs = sw.Config.RelabelConfigs
			labels = sw.Config.OriginalLabels
			found = true
			break
		}
	}
	tsmGlobal.mu.Unlock()

	if found {
		return relabelConfigs, labels, true
	}

	// Search for relabel context in droppedTargetsMap (aka deleted targets)
	droppedTargetsMap.mu.Lock()
	for _, dt := range droppedTargetsMap.m {
		if getLabelsID(dt.originalLabels) == targetID {
			relabelConfigs = dt.relabelConfigs
			labels = dt.originalLabels
			found = true
			break
		}
	}
	droppedTargetsMap.mu.Unlock()

	return relabelConfigs, labels, found
}

func (tsr *targetsStatusResult) getTargetLabelsByJob() []*targetLabelsByJob {
	byJob := make(map[string]*targetLabelsByJob)
	for _, jts := range tsr.jobTargetsStatuses {
		jobName := jts.jobName
		for _, ts := range jts.targetsStatus {
			m := byJob[jobName]
			if m == nil {
				m = &targetLabelsByJob{
					jobName: jobName,
				}
				byJob[jobName] = m
			}
			m.activeTargets++
			m.targets = append(m.targets, targetLabels{
				up:             ts.up,
				originalLabels: ts.sw.Config.OriginalLabels,
				labels:         ts.sw.Config.Labels,
			})
		}
	}
	for _, dt := range tsr.droppedTargets {
		jobName := dt.originalLabels.Get("job")
		m := byJob[jobName]
		if m == nil {
			m = &targetLabelsByJob{
				jobName: jobName,
			}
			byJob[jobName] = m
		}
		m.droppedTargets++
		m.targets = append(m.targets, targetLabels{
			originalLabels:    dt.originalLabels,
			dropReason:        dt.dropReason,
			clusterMemberNums: dt.clusterMemberNums,
		})
	}
	a := make([]*targetLabelsByJob, 0, len(byJob))
	for _, tls := range byJob {
		a = append(a, tls)
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].jobName < a[j].jobName
	})
	return a
}
