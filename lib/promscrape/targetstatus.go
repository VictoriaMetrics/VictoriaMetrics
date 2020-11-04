package promscrape

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var maxDroppedTargets = flag.Int("promscrape.maxDroppedTargets", 1000, "The maximum number of `droppedTargets` shown at /api/v1/targets page. "+
	"Increase this value if your setup drops more scrape targets during relabeling and you need investigating labels for all the dropped targets. "+
	"Note that the increased number of tracked dropped targets may result in increased memory usage")

var tsmGlobal = newTargetStatusMap()

// WriteHumanReadableTargetsStatus writes human-readable status for all the scrape targets to w.
func WriteHumanReadableTargetsStatus(w io.Writer, showOriginalLabels bool) {
	tsmGlobal.WriteHumanReadable(w, showOriginalLabels)
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
	mu sync.Mutex
	m  map[uint64]targetStatus
}

func newTargetStatusMap() *targetStatusMap {
	return &targetStatusMap{
		m: make(map[uint64]targetStatus),
	}
}

func (tsm *targetStatusMap) Reset() {
	tsm.mu.Lock()
	tsm.m = make(map[uint64]targetStatus)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Register(sw *ScrapeWork) {
	tsm.mu.Lock()
	tsm.m[sw.ID] = targetStatus{
		sw: sw,
	}
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Unregister(sw *ScrapeWork) {
	tsm.mu.Lock()
	delete(tsm.m, sw.ID)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Update(sw *ScrapeWork, group string, up bool, scrapeTime, scrapeDuration int64, err error) {
	tsm.mu.Lock()
	tsm.m[sw.ID] = targetStatus{
		sw:             sw,
		up:             up,
		scrapeGroup:    group,
		scrapeTime:     scrapeTime,
		scrapeDuration: scrapeDuration,
		err:            err,
	}
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
	for _, st := range tsm.m {
		key := promLabelsString(st.sw.OriginalLabels)
		kss = append(kss, keyStatus{
			key: key,
			st:  st,
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

func (tsm *targetStatusMap) WriteHumanReadable(w io.Writer, showOriginalLabels bool) {
	byJob := make(map[string][]targetStatus)
	tsm.mu.Lock()
	for _, st := range tsm.m {
		job := st.sw.Job()
		byJob[job] = append(byJob[job], st)
	}
	tsm.mu.Unlock()

	var jss []jobStatus
	for job, statuses := range byJob {
		jss = append(jss, jobStatus{
			job:      job,
			statuses: statuses,
		})
	}
	sort.Slice(jss, func(i, j int) bool {
		return jss[i].job < jss[j].job
	})

	for _, js := range jss {
		sts := js.statuses
		sort.Slice(sts, func(i, j int) bool {
			return sts[i].sw.ScrapeURL < sts[j].sw.ScrapeURL
		})
		ups := 0
		for _, st := range sts {
			if st.up {
				ups++
			}
		}
		fmt.Fprintf(w, "job=%q (%d/%d up)\n", js.job, ups, len(sts))
		for _, st := range sts {
			state := "up"
			if !st.up {
				state = "down"
			}
			labelsStr := st.sw.LabelsString()
			if showOriginalLabels {
				labelsStr += ", originalLabels=" + promLabelsString(st.sw.OriginalLabels)
			}
			lastScrape := st.getDurationFromLastScrape()
			errMsg := ""
			if st.err != nil {
				errMsg = st.err.Error()
			}
			fmt.Fprintf(w, "\tstate=%s, endpoint=%s, labels=%s, last_scrape=%.3fs ago, scrape_duration=%.3fs, error=%q\n",
				state, st.sw.ScrapeURL, labelsStr, lastScrape.Seconds(), float64(st.scrapeDuration)/1000, errMsg)
		}
	}
	fmt.Fprintf(w, "\n")
}

type jobStatus struct {
	job      string
	statuses []targetStatus
}

type targetStatus struct {
	sw             *ScrapeWork
	up             bool
	scrapeGroup    string
	scrapeTime     int64
	scrapeDuration int64
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
