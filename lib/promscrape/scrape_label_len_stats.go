package promscrape

import (
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/metrics"
)

type labelLenHistograms struct {
	nameLenHist  *metrics.Histogram
	valueLenHist *metrics.Histogram
}

var (
	labelLenHistogramsLock sync.Mutex
	labelLenHistogramsMap  = make(map[string]*labelLenHistograms)
)

func getLabelLenHistograms(jobName, relabelStage string) *labelLenHistograms {
	key := jobName + "/" + relabelStage
	labelLenHistogramsLock.Lock()
	defer labelLenHistogramsLock.Unlock()

	h, ok := labelLenHistogramsMap[key]
	if !ok {
		h = &labelLenHistograms{
			nameLenHist: metrics.GetOrCreateHistogram(
				fmt.Sprintf(`vmagent_scrape_label_name_length_bytes{job=%q,relabel=%q}`, jobName, relabelStage)),
			valueLenHist: metrics.GetOrCreateHistogram(
				fmt.Sprintf(`vmagent_scrape_label_value_length_bytes{job=%q,relabel=%q}`, jobName, relabelStage)),
		}
		labelLenHistogramsMap[key] = h
	}
	return h
}

// trackLabelLengthsBefore tracks label lengths from parsed rows before relabeling
func trackLabelLengthsBefore(jobName string, rows []parser.Row) {
	if len(rows) == 0 {
		return
	}
	h := getLabelLenHistograms(jobName, "before")

	for i := range rows {
		row := &rows[i]
		// Track metric name length as __name__ label
		h.nameLenHist.Update(float64(len("__name__")))
		h.valueLenHist.Update(float64(len(row.Metric)))

		for j := range row.Tags {
			tag := &row.Tags[j]
			h.nameLenHist.Update(float64(len(tag.Key)))
			h.valueLenHist.Update(float64(len(tag.Value)))
		}
	}
}

// trackLabelLengthsAfter tracks label lengths from TimeSeries after relabeling
func trackLabelLengthsAfter(jobName string, tss []prompb.TimeSeries) {
	if len(tss) == 0 {
		return
	}
	h := getLabelLenHistograms(jobName, "after")

	for i := range tss {
		ts := &tss[i]
		for j := range ts.Labels {
			label := &ts.Labels[j]
			h.nameLenHist.Update(float64(len(label.Name)))
			h.valueLenHist.Update(float64(len(label.Value)))
		}
	}
}
