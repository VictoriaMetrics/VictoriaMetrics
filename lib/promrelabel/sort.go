package promrelabel

import (
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// SortLabels sorts labels.
func SortLabels(labels []prompbmarshal.Label) {
	ls := labelsSorterPool.Get().(*labelsSorter)
	*ls = labels
	if !sort.IsSorted(ls) {
		sort.Sort(ls)
	}
	*ls = nil
	labelsSorterPool.Put(ls)
}

var labelsSorterPool = &sync.Pool{
	New: func() interface{} {
		return &labelsSorter{}
	},
}

type labelsSorter []prompbmarshal.Label

func (ls *labelsSorter) Len() int { return len(*ls) }
func (ls *labelsSorter) Swap(i, j int) {
	a := *ls
	a[i], a[j] = a[j], a[i]
}
func (ls *labelsSorter) Less(i, j int) bool {
	a := *ls
	return a[i].Name < a[j].Name
}
