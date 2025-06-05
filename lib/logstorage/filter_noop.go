package logstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterNoop does nothing
type filterNoop struct {
}

func (fn *filterNoop) String() string {
	return "*"
}

func (fn *filterNoop) updateNeededFields(_ *prefixfilter.Filter) {
	// nothing to do
}

func (fn *filterNoop) applyToBlockResult(_ *blockResult, _ *bitmap) {
	// nothing to do
}

func (fn *filterNoop) applyToBlockSearch(_ *blockSearch, _ *bitmap) {
	// nothing to do
}
