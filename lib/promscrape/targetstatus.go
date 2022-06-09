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

func (tsm *targetStatusMap) Update(sw *scrapeWork, up bool, scrapeTime, scrapeDuration int64, samplesScraped int, err error) {
	tsm.mu.Lock()
	ts := tsm.m[sw]
	if ts == nil {
		ts = &targetStatus{
			sw: sw,
		}
		tsm.m[sw] = ts
	}
	ts.up = up
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
		addr1 := promrelabel.GetLabelValueByName(tss[i].sw.Config.OriginalLabels, "__address__")
		addr2 := promrelabel.GetLabelValueByName(tss[j].sw.Config.OriginalLabels, "__address__")
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
		labelsFinalized := promrelabel.FinalizeLabels(nil, ts.sw.Config.Labels)
		writeLabelsJSON(w, labelsFinalized)
		fmt.Fprintf(w, `,"scrapePool":%q`, ts.sw.Config.Job())
		fmt.Fprintf(w, `,"scrapeUrl":%q`, ts.sw.Config.ScrapeURL)
		errMsg := ""
		if ts.err != nil {
			errMsg = ts.err.Error()
		}
		fmt.Fprintf(w, `,"lastError":%q`, errMsg)
		fmt.Fprintf(w, `,"lastScrape":%q`, time.Unix(ts.scrapeTime/1000, (ts.scrapeTime%1000)*1e6).Format(time.RFC3339Nano))
		fmt.Fprintf(w, `,"lastScrapeDuration":%g`, (time.Millisecond * time.Duration(ts.scrapeDuration)).Seconds())
		fmt.Fprintf(w, `,"lastSamplesScraped":%d`, ts.samplesScraped)
		state := "up"
		if !ts.up {
			state = "down"
		}
		fmt.Fprintf(w, `,"health":%q}`, state)
		if i+1 < len(tss) {
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
	scrapeTime     int64
	scrapeDuration int64
	samplesScraped int
	scrapesTotal   int
	scrapesFailed  int
	err            error
}

func (ts *targetStatus) getDurationFromLastScrape() time.Duration {
	return time.Since(time.Unix(ts.scrapeTime/1000, (ts.scrapeTime%1000)*1e6))
}

type droppedTargets struct {
	mu              sync.Mutex
	m               map[uint64]droppedTarget
	lastCleanupTime uint64
}

type droppedTarget struct {
	originalLabels []prompbmarshal.Label
	deadline       uint64
}

func (dt *droppedTargets) getTargetsLabels() [][]prompbmarshal.Label {
	dt.mu.Lock()
	dtls := make([][]prompbmarshal.Label, 0, len(dt.m))
	for _, v := range dt.m {
		dtls = append(dtls, v.originalLabels)
	}
	dt.mu.Unlock()
	// Sort discovered targets by __address__ label, so they stay in consistent order across calls
	sort.Slice(dtls, func(i, j int) bool {
		addr1 := promrelabel.GetLabelValueByName(dtls[i], "__address__")
		addr2 := promrelabel.GetLabelValueByName(dtls[j], "__address__")
		return addr1 < addr2
	})
	return dtls
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
	dtls := dt.getTargetsLabels()
	fmt.Fprintf(w, `[`)
	for i, labels := range dtls {
		fmt.Fprintf(w, `{"discoveredLabels":`)
		writeLabelsJSON(w, labels)
		fmt.Fprintf(w, `}`)
		if i+1 < len(dtls) {
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
	if len(filter.endpointSearch) > 0 || len(filter.labelSearch) > 0 {
		// Do not show empty jobs if target filters are set.
		emptyJobs = nil
	}
	dtls := droppedTargetsMap.getTargetsLabels()
	return &targetsStatusResult{
		jobTargetsStatuses:   jts,
		droppedTargetsLabels: dtls,
		emptyJobs:            emptyJobs,
		err:                  err,
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
	jobTargetsStatuses   []*jobTargetsStatuses
	droppedTargetsLabels [][]prompbmarshal.Label
	emptyJobs            []string
	err                  error
}

type targetLabels struct {
	up               bool
	discoveredLabels []prompbmarshal.Label
	labels           []prompbmarshal.Label
}
type targetLabelsByJob struct {
	jobName        string
	targets        []targetLabels
	activeTargets  int
	droppedTargets int
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
				up:               ts.up,
				discoveredLabels: ts.sw.Config.OriginalLabels,
				labels:           ts.sw.Config.Labels,
			})
		}
	}
	for _, labels := range tsr.droppedTargetsLabels {
		jobName := promrelabel.GetLabelValueByName(labels, "job")
		m := byJob[jobName]
		if m == nil {
			m = &targetLabelsByJob{
				jobName: jobName,
			}
			byJob[jobName] = m
		}
		m.droppedTargets++
		m.targets = append(m.targets, targetLabels{
			discoveredLabels: labels,
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
