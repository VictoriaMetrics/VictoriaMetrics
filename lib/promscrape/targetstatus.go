package promscrape

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var maxDroppedTargets = flag.Int("promscrape.maxDroppedTargets", 1000, "The maximum number of droppedTargets to show at /api/v1/targets page. "+
	"Increase this value if your setup drops more scrape targets during relabeling and you need investigating labels for all the dropped targets. "+
	"Note that the increased number of tracked dropped targets may result in increased memory usage")

var tsmGlobal = newTargetStatusMap()

// WriteHumanReadableTargetsStatus writes human-readable status for all the scrape targets to w according to r.
func WriteHumanReadableTargetsStatus(w http.ResponseWriter, r *http.Request) {
	showOriginalLabels, _ := strconv.ParseBool(r.FormValue("show_original_labels"))
	showOnlyUnhealthy, _ := strconv.ParseBool(r.FormValue("show_only_unhealthy"))
	if accept := r.Header.Get("Accept"); strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tsmGlobal.WriteTargetsHTML(w, showOnlyUnhealthy)
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
	m        map[*ScrapeWork]*targetStatus
	jobNames []string
}

func newTargetStatusMap() *targetStatusMap {
	return &targetStatusMap{
		m: make(map[*ScrapeWork]*targetStatus),
	}
}

func (tsm *targetStatusMap) Reset() {
	tsm.mu.Lock()
	tsm.m = make(map[*ScrapeWork]*targetStatus)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) registerJobNames(jobNames []string) {
	tsm.mu.Lock()
	tsm.jobNames = append(tsm.jobNames[:0], jobNames...)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Register(sw *ScrapeWork) {
	tsm.mu.Lock()
	tsm.m[sw] = &targetStatus{
		sw: sw,
	}
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Unregister(sw *ScrapeWork) {
	tsm.mu.Lock()
	delete(tsm.m, sw)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Update(sw *ScrapeWork, group string, up bool, scrapeTime, scrapeDuration int64, samplesScraped int, err error) {
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
	ts.err = err
	tsm.mu.Unlock()
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
		key := promLabelsString(sw.OriginalLabels)
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
		writeLabelsJSON(w, st.sw.OriginalLabels)
		fmt.Fprintf(w, `,"labels":`)
		labelsFinalized := promrelabel.FinalizeLabels(nil, st.sw.Labels)
		writeLabelsJSON(w, labelsFinalized)
		fmt.Fprintf(w, `,"scrapePool":%q`, st.sw.Job())
		fmt.Fprintf(w, `,"scrapeUrl":%q`, st.sw.ScrapeURL)
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
	sw             *ScrapeWork
	up             bool
	scrapeGroup    string
	scrapeTime     int64
	scrapeDuration int64
	samplesScraped int
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

type jobTargetStatus struct {
	up             bool
	endpoint       string
	labels         []prompbmarshal.Label
	originalLabels []prompbmarshal.Label
	lastScrapeTime time.Duration
	scrapeDuration time.Duration
	samplesScraped int
	errMsg         string
}

type jobTargetsStatuses struct {
	job           string
	upCount       int
	targetsTotal  int
	targetsStatus []jobTargetStatus
}

func (tsm *targetStatusMap) getTargetsStatusByJob() ([]jobTargetsStatuses, []string) {
	byJob := make(map[string][]targetStatus)
	tsm.mu.Lock()
	for _, st := range tsm.m {
		job := st.sw.jobNameOriginal
		byJob[job] = append(byJob[job], *st)
	}
	jobNames := append([]string{}, tsm.jobNames...)
	tsm.mu.Unlock()

	var jts []jobTargetsStatuses
	for job, statuses := range byJob {
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].sw.ScrapeURL < statuses[j].sw.ScrapeURL
		})
		ups := 0
		var targetsStatuses []jobTargetStatus
		for _, ts := range statuses {
			if ts.up {
				ups++
			}
		}
		for _, st := range statuses {
			errMsg := ""
			if st.err != nil {
				errMsg = st.err.Error()
			}
			targetsStatuses = append(targetsStatuses, jobTargetStatus{
				up:             st.up,
				endpoint:       st.sw.ScrapeURL,
				labels:         promrelabel.FinalizeLabels(nil, st.sw.Labels),
				originalLabels: st.sw.OriginalLabels,
				lastScrapeTime: st.getDurationFromLastScrape(),
				scrapeDuration: time.Duration(st.scrapeDuration) * time.Millisecond,
				samplesScraped: st.samplesScraped,
				errMsg:         errMsg,
			})
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
func (tsm *targetStatusMap) WriteTargetsHTML(w io.Writer, showOnlyUnhealthy bool) {
	jss, emptyJobs := tsm.getTargetsStatusByJob()
	WriteTargetsResponseHTML(w, jss, emptyJobs, showOnlyUnhealthy)
}

// WriteTargetsPlain writes targets grouped by job into writer w in plain text,
// accept filter to show original labels.
func (tsm *targetStatusMap) WriteTargetsPlain(w io.Writer, showOriginalLabels bool) {
	jss, emptyJobs := tsm.getTargetsStatusByJob()
	WriteTargetsResponsePlain(w, jss, emptyJobs, showOriginalLabels)
}
