package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterTimeOffset has different behaviour, depends on query context:
//
// If there are more than one time filter in a single 'and' group,
// so time offset should be applied to all time filters in the group.
// For example, '_time:1h and _time:offset 1h' should be merged into '_time:1h offset 1h'
//
// If there are no time filter in a single 'and' group,
// so the '_time:' filter should be applied.
type filterTimeOffset struct {
	// offset that should be applied to filterTime.
	offset int64
}

func (fto *filterTimeOffset) String() string {
	panic(fmt.Errorf("BUG: filterTimeOffset should not be called"))
}

func (fto *filterTimeOffset) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("_time")
}

func (fto *filterTimeOffset) applyToBlockResult(*blockResult, *bitmap) {
	panic(fmt.Errorf("BUG: filterTimeOffset should not filter any data"))
}

func (fto *filterTimeOffset) applyToBlockSearch(*blockSearch, *bitmap) {
	panic(fmt.Errorf("BUG: filterTimeOffset should not filter any data"))
}
