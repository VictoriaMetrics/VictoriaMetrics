package remotewrite

import (
	"flag"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var sortLabels = flag.Bool("sortLabels", false, `Whether to sort labels for incoming samples before writing them to all the configured remote storage systems. `+
	`This may be needed for reducing memory usage at remote storage when the order of labels in incoming samples is random. `+
	`For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}`+
	`Enabled sorting for labels can slow down ingestion performance a bit`)

// sortLabelsIfNeeded sorts labels if -sortLabels command-line flag is set.
func sortLabelsIfNeeded(tss []prompbmarshal.TimeSeries) {
	if !*sortLabels {
		return
	}
	// The slc is used for avoiding memory allocation when passing labels to sort.Sort.
	slc := sortLabelsCtxPool.Get().(*sortLabelsCtx)
	for i := range tss {
		slc.labels = tss[i].Labels
		sort.Sort(&slc.labels)
	}
	slc.labels = nil
	sortLabelsCtxPool.Put(slc)
}

type sortLabelsCtx struct {
	labels sortedLabels
}

var sortLabelsCtxPool = &sync.Pool{
	New: func() interface{} {
		return &sortLabelsCtx{}
	},
}

type sortedLabels []prompbmarshal.Label

func (sl *sortedLabels) Len() int { return len(*sl) }
func (sl *sortedLabels) Less(i, j int) bool {
	a := *sl
	return a[i].Name < a[j].Name
}
func (sl *sortedLabels) Swap(i, j int) {
	a := *sl
	a[i], a[j] = a[j], a[i]
}
