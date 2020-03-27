package promscrape

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var tsmGlobal = newTargetStatusMap()

// WriteHumanReadableTargetsStatus writes human-readable status for all the scrape targets to w.
func WriteHumanReadableTargetsStatus(w io.Writer) {
	tsmGlobal.WriteHumanReadable(w)
}

type targetStatusMap struct {
	mu sync.Mutex
	m  map[string]targetStatus
}

func newTargetStatusMap() *targetStatusMap {
	return &targetStatusMap{
		m: make(map[string]targetStatus),
	}
}

func (tsm *targetStatusMap) Reset() {
	tsm.mu.Lock()
	tsm.m = make(map[string]targetStatus)
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) RegisterAll(sws []ScrapeWork) {
	tsm.mu.Lock()
	for i := range sws {
		sw := &sws[i]
		tsm.m[sw.ScrapeURL] = targetStatus{
			sw: sw,
		}
	}
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) UnregisterAll(sws []ScrapeWork) {
	tsm.mu.Lock()
	for i := range sws {
		delete(tsm.m, sws[i].ScrapeURL)
	}
	tsm.mu.Unlock()
}

func (tsm *targetStatusMap) Update(sw *ScrapeWork, up bool, scrapeTime, scrapeDuration int64, err error) {
	tsm.mu.Lock()
	tsm.m[sw.ScrapeURL] = targetStatus{
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
		job := ""
		label := promrelabel.GetLabelByName(st.sw.Labels, "job")
		if label != nil {
			job = label.Value
		}
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
			var labels []string
			for _, label := range promrelabel.FinalizeLabels(nil, st.sw.Labels) {
				labels = append(labels, fmt.Sprintf("%s=%q", label.Name, label.Value))
			}
			labelsStr := "{" + strings.Join(labels, ", ") + "}"
			lastScrape := st.getDurationFromLastScrape()
			errMsg := ""
			if st.err != nil {
				errMsg = st.err.Error()
			}
			fmt.Fprintf(w, "\tstate=%s, endpoint=%s, labels=%s, last_scrape=%.3fs ago, scrape_duration=%.3fs, error=%q\n",
				state, st.sw.ScrapeURL, labelsStr, lastScrape.Seconds(), float64(st.scrapeDuration)/1000, errMsg)
		}
	}
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
