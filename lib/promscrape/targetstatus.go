package promscrape

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

var tsmGlobal = newTargetStatusMap()

// WriteHumanReadableTargetsStatus writes human-readable status for all the scrape targets to w.
func WriteHumanReadableTargetsStatus(w io.Writer) {
	tsmGlobal.WriteHumanReadable(w)
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

func (tsm *targetStatusMap) Update(sw *ScrapeWork, up bool, scrapeTime, scrapeDuration int64, err error) {
	tsm.mu.Lock()
	tsm.m[sw.ID] = targetStatus{
		sw:             sw,
		up:             up,
		scrapeTime:     scrapeTime,
		scrapeDuration: scrapeDuration,
		err:            err,
	}
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) WriteHumanReadable(w io.Writer) {
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
	scrapeTime     int64
	scrapeDuration int64
	err            error
}

func (st *targetStatus) getDurationFromLastScrape() time.Duration {
	return time.Since(time.Unix(st.scrapeTime/1000, (st.scrapeTime%1000)*1e6))
}
