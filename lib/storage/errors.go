package storage

import (
	"fmt"
)

// FlagSeriesLimit contains a name of the flag name placeholder
// for search limits reaching errors
const FlagSeriesLimit = "-search.max*"

// ErrSeriesLimit is a generic error for cases when search limit is reached
var ErrSeriesLimit = fmt.Errorf("either narrow down the search or increase %s command-line "+
	"flag values at vmselect; see https://docs.victoriametrics.com/#resource-usage-limits", FlagSeriesLimit)
